// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"forgejo.org/models/db"
	packages_model "forgejo.org/models/packages"
	container_model "forgejo.org/models/packages/container"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
	packages_module "forgejo.org/modules/packages"
	container_module "forgejo.org/modules/packages/container"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
	notify_service "forgejo.org/services/notify"
	packages_service "forgejo.org/services/packages"

	digest "github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

// maximum size of a container manifest
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-manifests
const MaxManifestSize = 10 * 1024 * 1024

var (
	ReferencePattern = regexp.MustCompile(`\A[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}\z`)
	ErrTagInvalid    = util.NewInvalidArgumentErrorf("Tag is invalid")
)

// ManifestCreationInfo describes a manifest to create
type ManifestCreationInfo struct {
	MediaType  string
	Owner      *user_model.User
	Creator    *user_model.User
	Image      string
	Reference  string
	IsTagged   bool
	Properties map[string]string
}

func GetLocalManifest(ctx context.Context, ownerID int64, imageName, reference string) (*packages_model.PackageFileDescriptor, error) {
	opts, err := GetManifestSearchOptions(
		ownerID,
		imageName,
		reference,
	)
	if err != nil {
		return nil, err
	}
	// Get blob or err
	log.Debug("Trying to find manifest with %s locally", reference)
	pdf, err := WorkaroundGetContainerBlob(ctx, opts)
	if err != nil {
		if errors.Is(err, container_model.ErrContainerBlobNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("could not get container blob: %s", err.Error())
	}

	return pdf, nil
}

func NewManifestCreationInfo(owner, creator *user_model.User, mediaType, image, reference string) (*ManifestCreationInfo, error) {
	isTagged := digest.Digest(reference).Validate() != nil

	mci := &ManifestCreationInfo{
		MediaType: mediaType,
		Owner:     owner,
		Creator:   creator,
		Image:     image,
		Reference: reference,
		IsTagged:  isTagged,
	}

	if mci.IsTagged && !ReferencePattern.MatchString(reference) {
		return &ManifestCreationInfo{}, ErrTagInvalid
	}

	return mci, nil
}

func isValidMediaType(mt string) bool {
	return strings.HasPrefix(mt, "application/vnd.docker.") || strings.HasPrefix(mt, "application/vnd.oci.")
}

func isImageManifestMediaType(mt string) bool {
	return strings.EqualFold(mt, oci.MediaTypeImageManifest) || strings.EqualFold(mt, "application/vnd.docker.distribution.manifest.v2+json")
}

func isImageIndexMediaType(mt string) bool {
	return strings.EqualFold(mt, oci.MediaTypeImageIndex) || strings.EqualFold(mt, "application/vnd.docker.distribution.manifest.list.v2+json")
}

func GetManifestSearchOptions(ownerID int64, image, reference string) (*container_model.BlobSearchOptions, error) {
	opts := &container_model.BlobSearchOptions{
		OwnerID:    ownerID,
		Image:      image,
		IsManifest: true,
	}

	if digest.Digest(reference).Validate() == nil {
		opts.Digest = reference
	} else if ReferencePattern.MatchString(reference) {
		opts.Tag = reference
	} else {
		return nil, container_model.ErrContainerBlobNotExist
	}

	return opts, nil
}

func ProcessManifest(ctx context.Context, mci ManifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	var index oci.Index
	if err := json.NewDecoder(buf).Decode(&index); err != nil {
		return "", err
	}

	if index.SchemaVersion != 2 {
		return "", ErrUnsupported.WithMessage("Schema version is not supported")
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if !isValidMediaType(mci.MediaType) {
		mci.MediaType = index.MediaType
		if !isValidMediaType(mci.MediaType) {
			return "", ErrManifestInvalid.WithMessage("MediaType not recognized")
		}
	}

	if isImageManifestMediaType(mci.MediaType) {
		return processImageManifest(ctx, mci, buf)
	} else if isImageIndexMediaType(mci.MediaType) {
		return processImageManifestIndex(ctx, mci, buf)
	}
	return "", ErrManifestInvalid
}

func processImageManifest(ctx context.Context, mci ManifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	manifestDigest := ""

	err := func() error {
		var manifest oci.Manifest
		if err := json.NewDecoder(buf).Decode(&manifest); err != nil {
			return err
		}

		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			return err
		}

		ctx, committer, err := db.TxContext(ctx)
		if err != nil {
			return err
		}
		defer committer.Close()

		configDescriptor, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
			OwnerID: mci.Owner.ID,
			Image:   mci.Image,
			Digest:  string(manifest.Config.Digest),
		})
		if err != nil {
			return err
		}

		configReader, err := packages_module.NewContentStore().Get(packages_module.BlobHash256Key(configDescriptor.Blob.HashSHA256))
		if err != nil {
			return err
		}
		defer configReader.Close()

		metadata, err := container_module.ParseImageConfig(manifest.Config.MediaType, configReader)
		if err != nil {
			return err
		}

		metadata.Annotations = manifest.Annotations

		blobReferences := make([]*blobReference, 0, 1+len(manifest.Layers))

		blobReferences = append(blobReferences, &blobReference{
			Digest:       manifest.Config.Digest,
			MediaType:    manifest.Config.MediaType,
			File:         configDescriptor,
			ExpectedSize: manifest.Config.Size,
		})

		for _, layer := range manifest.Layers {
			pfd, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
				OwnerID: mci.Owner.ID,
				Image:   mci.Image,
				Digest:  string(layer.Digest),
			})
			if err != nil {
				return err
			}

			blobReferences = append(blobReferences, &blobReference{
				Digest:       layer.Digest,
				MediaType:    layer.MediaType,
				File:         pfd,
				ExpectedSize: layer.Size,
			})
		}

		pv, err := createPackageAndVersion(ctx, mci, metadata)
		if err != nil {
			return err
		}

		uploadVersion, err := packages_model.GetInternalVersionByNameAndVersion(ctx, mci.Owner.ID, packages_model.TypeContainer, mci.Image, container_model.UploadVersion)
		if err != nil && err != packages_model.ErrPackageNotExist {
			return err
		}

		for _, ref := range blobReferences {
			if err := createFileFromBlobReference(ctx, pv, uploadVersion, ref); err != nil {
				return err
			}
		}

		pb, created, digest, err := createManifestBlob(ctx, mci, pv, buf)
		removeBlob := false
		defer func() {
			if removeBlob {
				contentStore := packages_module.NewContentStore()
				if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
					log.Error("Error deleting package blob from content store: %v", err)
				}
			}
		}()
		if err != nil {
			removeBlob = created
			return err
		}

		if err := committer.Commit(); err != nil {
			removeBlob = created
			return err
		}

		if err := notifyPackageCreate(ctx, mci.Creator, pv); err != nil {
			return err
		}

		manifestDigest = digest

		return nil
	}()
	if err != nil {
		return "", err
	}

	return manifestDigest, nil
}

func processImageManifestIndex(ctx context.Context, mci ManifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	manifestDigest := ""

	err := func() error {
		var index oci.Index
		if err := json.NewDecoder(buf).Decode(&index); err != nil {
			return err
		}

		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			return err
		}

		ctx, committer, err := db.TxContext(ctx)
		if err != nil {
			return err
		}
		defer committer.Close()

		metadata := &container_module.Metadata{
			Type:      container_module.TypeOCI,
			Manifests: make([]*container_module.Manifest, 0, len(index.Manifests)),
		}

		for _, manifest := range index.Manifests {
			if !isImageManifestMediaType(manifest.MediaType) {
				return ErrManifestInvalid
			}

			platform := container_module.DefaultPlatform
			if manifest.Platform != nil {
				platform = fmt.Sprintf("%s/%s", manifest.Platform.OS, manifest.Platform.Architecture)
				if manifest.Platform.Variant != "" {
					platform = fmt.Sprintf("%s/%s", platform, manifest.Platform.Variant)
				}
			}

			pfd, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
				OwnerID:    mci.Owner.ID,
				Image:      mci.Image,
				Digest:     string(manifest.Digest),
				IsManifest: true,
			})
			if err != nil {
				if err == container_model.ErrContainerBlobNotExist {
					return ErrManifestBlobUnknown
				}
				return err
			}

			size, err := packages_model.CalculateFileSize(ctx, &packages_model.PackageFileSearchOptions{
				VersionID: pfd.File.VersionID,
			})
			if err != nil {
				return err
			}

			metadata.Manifests = append(metadata.Manifests, &container_module.Manifest{
				Platform: platform,
				Digest:   string(manifest.Digest),
				Size:     size,
			})
		}

		pv, err := createPackageAndVersion(ctx, mci, metadata)
		if err != nil {
			return err
		}

		pb, created, digest, err := createManifestBlob(ctx, mci, pv, buf)
		removeBlob := false
		defer func() {
			if removeBlob {
				contentStore := packages_module.NewContentStore()
				if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
					log.Error("Error deleting package blob from content store: %v", err)
				}
			}
		}()
		if err != nil {
			removeBlob = created
			return err
		}

		if err := committer.Commit(); err != nil {
			removeBlob = created
			return err
		}

		if err := notifyPackageCreate(ctx, mci.Creator, pv); err != nil {
			return err
		}

		manifestDigest = digest

		return nil
	}()
	if err != nil {
		return "", err
	}

	return manifestDigest, nil
}

func notifyPackageCreate(ctx context.Context, doer *user_model.User, pv *packages_model.PackageVersion) error {
	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		return err
	}

	notify_service.PackageCreate(ctx, doer, pd)

	return nil
}

func createPackageAndVersion(ctx context.Context, mci ManifestCreationInfo, metadata *container_module.Metadata) (*packages_model.PackageVersion, error) {
	created := true
	p := &packages_model.Package{
		OwnerID:   mci.Owner.ID,
		Type:      packages_model.TypeContainer,
		Name:      strings.ToLower(mci.Image),
		LowerName: strings.ToLower(mci.Image),
	}
	var err error

	if p, err = packages_model.TryInsertPackage(ctx, p); err != nil {
		if err == packages_model.ErrDuplicatePackage {
			created = false
		} else {
			log.Error("Error inserting package: %v", err)
			return nil, err
		}
	}

	if created {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, strings.ToLower(mci.Owner.LowerName+"/"+mci.Image)); err != nil {
			log.Error("Error setting package property %s: %v", container_module.PropertyRepository, err)
			return nil, err
		}
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepositoryAutolinkingPending, "yes"); err != nil {
			log.Error("Error setting package property %s: %v", container_module.PropertyRepositoryAutolinkingPending, err)
			return nil, err
		}
	}

	// Check if auto-linking is required (this only happens after creation of package (not version!))
	autolinkRequiredProps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepositoryAutolinkingPending)
	if err != nil {
		log.Error("Error getting package properties %s: %v", container_module.PropertyRepositoryAutolinkingPending, err)
		return nil, err
	}
	if len(autolinkRequiredProps) > 0 {
		autolinkRequiredProp := autolinkRequiredProps[0]
		if autolinkRequiredProp != nil && autolinkRequiredProp.Value == "yes" { // check if auto-link is required (this prevents re-auto-linking on new versions, since the property is not set there)
			if _, err := tryAutoLink(ctx, p, mci.Owner.LowerName, mci.Image, metadata, mci.Creator); err != nil {
				log.Error("Auto-linking failed for package %d: %v", p.ID, err)
			}
			// remove property regardless of success/failure to keep behavior consistent and prevent retries on re-runs.
			if err := packages_model.DeletePropertyByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepositoryAutolinkingPending); err != nil {
				return nil, err
			}
		}
	}

	metadata.IsTagged = mci.IsTagged

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	_pv := &packages_model.PackageVersion{
		PackageID:    p.ID,
		CreatorID:    mci.Creator.ID,
		Version:      strings.ToLower(mci.Reference),
		LowerVersion: strings.ToLower(mci.Reference),
		MetadataJSON: string(metadataJSON),
	}
	var pv *packages_model.PackageVersion
	if pv, err = packages_model.GetOrInsertVersion(ctx, _pv); err != nil {
		if err == packages_model.ErrDuplicatePackageVersion {
			if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
				return nil, err
			}

			// keep download count on overwrite
			_pv.DownloadCount = pv.DownloadCount

			if pv, err = packages_model.GetOrInsertVersion(ctx, _pv); err != nil {
				log.Error("Error inserting package: %v", err)
				return nil, err
			}
		} else {
			log.Error("Error inserting package: %v", err)
			return nil, err
		}
	}

	if err := packages_service.CheckCountQuotaExceeded(ctx, mci.Creator, mci.Owner); err != nil {
		return nil, err
	}

	if mci.IsTagged {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestTagged, ""); err != nil {
			log.Error("Error setting package version property: %v", err)
			return nil, err
		}
	}
	for _, manifest := range metadata.Manifests {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestReference, manifest.Digest); err != nil {
			log.Error("Error setting package version property: %v", err)
			return nil, err
		}
	}

	return pv, nil
}

type blobReference struct {
	Digest       digest.Digest
	MediaType    string
	Name         string
	File         *packages_model.PackageFileDescriptor
	ExpectedSize int64
	IsLead       bool
}

func createFileFromBlobReference(ctx context.Context, pv, uploadVersion *packages_model.PackageVersion, ref *blobReference) error {
	if ref.File.Blob.Size != ref.ExpectedSize {
		return ErrSizeInvalid
	}

	if ref.Name == "" {
		ref.Name = strings.ToLower(fmt.Sprintf("sha256_%s", ref.File.Blob.HashSHA256))
	}

	pf := &packages_model.PackageFile{
		VersionID: pv.ID,
		BlobID:    ref.File.Blob.ID,
		Name:      ref.Name,
		LowerName: ref.Name,
		IsLead:    ref.IsLead,
	}
	var err error
	if pf, err = packages_model.TryInsertFile(ctx, pf); err != nil {
		if err == packages_model.ErrDuplicatePackageFile {
			// Skip this blob because the manifest contains the same filesystem layer multiple times.
			return nil
		}
		log.Error("Error inserting package file: %v", err)
		return err
	}

	props := map[string]string{
		container_module.PropertyMediaType: ref.MediaType,
		container_module.PropertyDigest:    string(ref.Digest),
	}
	for name, value := range props {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeFile, pf.ID, name, value); err != nil {
			log.Error("Error setting package file property: %v", err)
			return err
		}
	}

	// Remove the file from the blob upload version
	if uploadVersion != nil && ref.File.File != nil && uploadVersion.ID == ref.File.File.VersionID {
		if err := packages_service.DeletePackageFile(ctx, ref.File.File); err != nil {
			return err
		}
	}

	return nil
}

func createManifestBlob(ctx context.Context, mci ManifestCreationInfo, pv *packages_model.PackageVersion, buf *packages_module.HashedBuffer) (*packages_model.PackageBlob, bool, string, error) {
	pb, exists, err := packages_model.GetOrInsertBlob(ctx, packages_service.NewPackageBlob(buf))
	if err != nil {
		log.Error("Error inserting package blob: %v", err)
		return nil, false, "", err
	}
	// FIXME: Workaround to be removed in v1.20
	// https://github.com/go-gitea/gitea/issues/19586
	if exists {
		err = packages_module.NewContentStore().Has(packages_module.BlobHash256Key(pb.HashSHA256))
		if err != nil && (errors.Is(err, util.ErrNotExist) || errors.Is(err, os.ErrNotExist)) {
			log.Debug("Package registry inconsistent: blob %s does not exist on file system", pb.HashSHA256)
			exists = false
		}
	}
	if !exists {
		contentStore := packages_module.NewContentStore()
		if err := contentStore.Save(packages_module.BlobHash256Key(pb.HashSHA256), buf, buf.Size()); err != nil {
			log.Error("Error saving package blob in content store: %v", err)
			return nil, false, "", err
		}
	}

	manifestDigest := DigestFromHashSummer(buf)
	err = createFileFromBlobReference(ctx, pv, nil, &blobReference{
		Digest:       digest.Digest(manifestDigest),
		MediaType:    mci.MediaType,
		Name:         container_model.ManifestFilename,
		File:         &packages_model.PackageFileDescriptor{Blob: pb},
		ExpectedSize: pb.Size,
		IsLead:       true,
	})

	return pb, !exists, manifestDigest, err
}

// Attempty to link a package to a repository in the following order of precedence: by annotation, by label and finally by image name.
// If it fails, it returns false, nil. Only actual errors are returned, so don't use the err return only to determine if the linking was performed.
func tryAutoLink(ctx context.Context, p *packages_model.Package, imageOwner, imageName string, metadata *container_module.Metadata, doer *user_model.User) (linked bool, err error) {
	// We can use the same function for linking by annotation as is used for
	// linking by label, since the field has the exact same structure
	if linkedByAnnotation, err := tryAutolinkByLabel(ctx, p, metadata.Annotations, doer); err != nil {
		return false, err
	} else if linkedByAnnotation {
		log.Info("Image %s/%s was auto-linked by annotation", imageOwner, imageName)
		return true, nil
	}

	if linkedByLabel, err := tryAutolinkByLabel(ctx, p, metadata.Labels, doer); err != nil {
		return false, err
	} else if linkedByLabel {
		log.Info("Image %s/%s was auto-linked by label", imageOwner, imageName)
		return true, nil
	}

	if linkedByName, err := tryAutolinkByImageName(ctx, p, imageOwner, imageName, doer); err != nil {
		return false, err
	} else if linkedByName {
		log.Info("Image %s/%s was auto-linked by image name", imageOwner, imageName)
		return true, nil
	}

	return false, nil
}

// Tries to link a package to a repository by label from metadata.
// If it fails, it returns false, nil. Only actual errors are returned, so don't use the err return to determine if the linking was performed.
func tryAutolinkByLabel(ctx context.Context, p *packages_model.Package, labels map[string]string, doer *user_model.User) (linked bool, err error) {
	if labels == nil {
		return false, nil
	}

	labelRepo, ok := labels["org.opencontainers.image.source"]
	if !ok {
		return false, nil
	}

	u, err := url.Parse(labelRepo)
	if err != nil {
		log.Warn("Failed to extract label value org.opencontainers.image.source: value is not in format '{host}/{owner}/{repo}' (is: %s)", labelRepo)
		return false, nil // we do not return an error here, since a malformed label should simply be ignored
	}

	fullBasePath := fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
	if setting.AppURL != fullBasePath {
		log.Warn("Failed to extract label value org.opencontainers.image.source: host does not match Forgejo AppURL (is: %s, want: %s)", fullBasePath, setting.AppURL)
		return false, nil
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) != 2 {
		log.Warn("Failed to extract label value org.opencontainers.image.source: value is not in format '{host}/{owner}/{repo}' (is: %s)", labelRepo)
	}

	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, pathParts[0], pathParts[1])
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			return false, err // this is a legit error
		}
		return false, nil
	}

	if err := packages_service.LinkToRepository(ctx, p, repository, doer); err != nil {
		if errors.Is(err, util.ErrPermissionDenied) {
			return false, nil // we don't want an error case if the user does not have write access to the repo they have write access to
		}
		return false, err
	}
	return true, nil
}

// Tries to link a package to a repository by its name (using {owner}/{repo}[/...]).
// If it fails, it returns false, nil. Only actual errors are returned, so don't use the err return to determine if the linking was performed.
func tryAutolinkByImageName(ctx context.Context, p *packages_model.Package, imageOwner, imageName string, doer *user_model.User) (linked bool, err error) {
	repoName, _, _ := strings.Cut(imageName, "/")
	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, imageOwner, repoName)
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			return false, err // this is a legit error
		}
		return false, nil
	}
	if err := packages_service.LinkToRepository(ctx, p, repository, doer); err != nil {
		if errors.Is(err, util.ErrPermissionDenied) {
			return false, nil // we don't want an error case if the user does not have write access to the repo they have write access to
		}
		return false, err
	}
	return true, nil
}
