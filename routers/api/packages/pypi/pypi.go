// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pypi

import (
	"encoding/hex"
	"io"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	packages_model "forgejo.org/models/packages"
	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
	packages_module "forgejo.org/modules/packages"
	pypi_module "forgejo.org/modules/packages/pypi"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/validation"
	"forgejo.org/routers/api/packages/helper"
	"forgejo.org/services/context"
	packages_service "forgejo.org/services/packages"

	"github.com/munnerz/goautoneg"
)

// https://peps.python.org/pep-0426/#name
var (
	normalizer  = strings.NewReplacer(".", "-", "_", "-")
	nameMatcher = regexp.MustCompile(`\A(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\.\-_]*[a-zA-Z0-9])\z`)
)

// https://peps.python.org/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions
var versionMatcher = regexp.MustCompile(`\Av?` +
	`(?:[0-9]+!)?` + // epoch
	`[0-9]+(?:\.[0-9]+)*` + // release segment
	`(?:[-_\.]?(?:a|b|c|rc|alpha|beta|pre|preview)[-_\.]?[0-9]*)?` + // pre-release
	`(?:-[0-9]+|[-_\.]?(?:post|rev|r)[-_\.]?[0-9]*)?` + // post release
	`(?:[-_\.]?dev[-_\.]?[0-9]*)?` + // dev release
	`(?:\+[a-z0-9]+(?:[-_\.][a-z0-9]+)*)?` + // local version
	`\z`)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

// v1HTMLPackageMetadata returns the metadata for a single package in Simple HTML per PEP691
func v1HTMLPackageMetadata(ctx *context.Context, ctype string) {
	packageName := normalizer.Replace(ctx.Params("id"))

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypePyPI, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// sort package descriptors by version to mimic PyPI format
	sort.Slice(pds, func(i, j int) bool {
		return strings.Compare(pds[i].Version.Version, pds[j].Version.Version) < 0
	})

	ctx.Data["RegistryURL"] = setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/pypi"
	ctx.Data["PackageDescriptor"] = pds[0]
	ctx.Data["PackageDescriptors"] = pds
	// PEP 691 defines `text/html` as an alias for this media type, so a single
	// Content-Type header is sufficient.
	ctx.Resp.Header().Set("Content-Type", ctype)
	ctx.HTML(http.StatusOK, "api/packages/pypi/simple")
}

// v1JSONPackageMetadata returns the metadata for a single package in Simple JSON per PEP691
func v1JSONPackageMetadata(ctx *context.Context, ctype string) {
	packageName := normalizer.Replace(ctx.Params("id"))

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypePyPI, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// sort package descriptors by version to mimic PyPI format
	slices.SortFunc(pds, func(a, b *packages_model.PackageDescriptor) int {
		return strings.Compare(a.Version.Version, b.Version.Version)
	})
	registryURL := setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/pypi"
	versions := make([]string, len(pvs))
	for i, pv := range pvs {
		versions[i] = pv.Version
	}
	var fileCounter int
	for _, pd := range pds {
		fileCounter += len(pd.Files)
	}
	files := make([]pypi_module.FileJSON, fileCounter)
	var i int
	for _, pd := range pds {
		for _, file := range pd.Files {
			files[i] = pypi_module.FileJSON{
				Filename:       file.File.Name,
				URL:            registryURL + "/files/" + pd.Package.LowerName + "/" + pd.Version.Version + "/" + file.File.Name,
				RequiresPython: pd.Metadata.(*pypi_module.Metadata).RequiresPython,
				Hashes:         pypi_module.FileHashesJSON{SHA256: file.Blob.HashSHA256},
				Size:           file.Blob.Size,
			}
			i++
		}
	}
	content := pypi_module.PackageJSON{
		Name:     pds[0].Package.Name,
		Meta:     pypi_module.PackageMetaJSON{APIVersion: "1.4"},
		Versions: versions,
		Files:    files,
	}
	// PEP 691 requires the canonical media type for JSON responses. Sending an
	// additional `application/json` header breaks strict clients (e.g. uv).
	ctx.Resp.Header().Set("Content-Type", ctype)
	if err := json.NewEncoder(ctx.Resp).Encode(content); err != nil {
		log.Error("Render JSON failed: %v", err)
		apiError(ctx, http.StatusInternalServerError, err)
	}
}

func PackageMetadata(ctx *context.Context) {
	types := ctx.Req.Header.Values("Accept")
	if len(types) == 0 {
		v1HTMLPackageMetadata(ctx, "text/html")
		return
	}

	// `text/html` is listed first so that an `Accept: */*` request falls back
	// to the HTML representation instead of the JSON one.
	accept := goautoneg.Negotiate(strings.Join(types, ","), []string{"text/html", "application/vnd.pypi.simple.v1+json", "application/vnd.pypi.simple.v1+html", "application/vnd.pypi.simple.latest+json", "application/vnd.pypi.simple.latest+html"})

	switch accept {
	case "application/vnd.pypi.simple.v1+json", "application/vnd.pypi.simple.latest+json":
		// The `latest` media types are aliases for the `v1` media types, so the
		// canonical `v1` type is used in the response.
		v1JSONPackageMetadata(ctx, "application/vnd.pypi.simple.v1+json")
		return
	case "application/vnd.pypi.simple.v1+html", "application/vnd.pypi.simple.latest+html":
		v1HTMLPackageMetadata(ctx, "application/vnd.pypi.simple.v1+html")
		return
	case "text/html": // alias of v1+ html
		v1HTMLPackageMetadata(ctx, "text/html")
		return
	}

	ctx.PlainText(http.StatusNotAcceptable, "no supported content type in Accept header")
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.Context) {
	packageName := normalizer.Replace(ctx.Params("id"))
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, u, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypePyPI,
			Name:        packageName,
			Version:     packageVersion,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.Context) {
	file, fileHeader, err := ctx.Req.FormFile("content")
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	buf, err := packages_module.CreateHashedBufferFromReader(file)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	_, _, hashSHA256, _, _ := buf.Sums()

	if !strings.EqualFold(ctx.Req.FormValue("sha256_digest"), hex.EncodeToString(hashSHA256)) {
		apiError(ctx, http.StatusBadRequest, "hash mismatch")
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	packageName := normalizer.Replace(ctx.Req.FormValue("name"))
	packageVersion := ctx.Req.FormValue("version")
	if !isValidNameAndVersion(packageName, packageVersion) {
		apiError(ctx, http.StatusBadRequest, "invalid name or version")
		return
	}

	// Ensure ctx.Req.Form exists.
	_ = ctx.Req.ParseForm()

	var homepageURL string
	projectURLs := ctx.Req.Form["project_urls"]
	for _, purl := range projectURLs {
		label, url, found := strings.Cut(purl, ",")
		if !found {
			continue
		}
		if normalizeLabel(label) != "homepage" {
			continue
		}
		homepageURL = strings.TrimSpace(url)
		break
	}

	if len(homepageURL) == 0 {
		// TODO: Home-page is a deprecated metadata field. Remove this branch once it's no longer apart of the spec.
		homepageURL = ctx.Req.FormValue("home_page")
	}

	if !validation.IsValidURL(homepageURL) {
		homepageURL = ""
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypePyPI,
				Name:        packageName,
				Version:     packageVersion,
			},
			SemverCompatible: false,
			Creator:          ctx.Doer,
			Metadata: &pypi_module.Metadata{
				Author:          ctx.Req.FormValue("author"),
				Description:     ctx.Req.FormValue("description"),
				LongDescription: ctx.Req.FormValue("long_description"),
				Summary:         ctx.Req.FormValue("summary"),
				ProjectURL:      homepageURL,
				License:         ctx.Req.FormValue("license"),
				RequiresPython:  ctx.Req.FormValue("requires_python"),
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: fileHeader.Filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

// Normalizes a Project-URL label.
// See https://packaging.python.org/en/latest/specifications/well-known-project-urls/#label-normalization.
func normalizeLabel(label string) string {
	var builder strings.Builder

	// "A label is normalized by deleting all ASCII punctuation and whitespace, and then converting the result
	// to lowercase."
	for _, r := range label {
		if unicode.IsPunct(r) || unicode.IsSpace(r) {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}

	return builder.String()
}

func isValidNameAndVersion(packageName, packageVersion string) bool {
	return nameMatcher.MatchString(packageName) && versionMatcher.MatchString(packageVersion)
}
