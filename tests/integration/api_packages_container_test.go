// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	packages_model "forgejo.org/models/packages"
	container_model "forgejo.org/models/packages/container"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	container_module "forgejo.org/modules/packages/container"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	packages_service "forgejo.org/services/packages"
	"forgejo.org/tests"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageContainer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadPackage)
	privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31})

	has := func(l packages_model.PackagePropertyList, name string) bool {
		for _, pp := range l {
			if pp.Name == name {
				return true
			}
		}
		return false
	}
	getAllByName := func(l packages_model.PackagePropertyList, name string) []string {
		values := make([]string, 0, len(l))
		for _, pp := range l {
			if pp.Name == name {
				values = append(values, pp.Value)
			}
		}
		return values
	}

	images := []string{"test", "te/st", "oras-artifact"}
	tags := []string{"latest", "main"}
	multiTag := "multi"

	unknownDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	blobContent, _ := base64.StdEncoding.DecodeString(`H4sIAAAJbogA/2IYBaNgFIxYAAgAAP//Lq+17wAEAAA=`)
	blobDigest := "sha256:" + sha256Hash(string(blobContent))
	maintainer := "contact@forgejo.org"
	description := "PACKAGE DESCRIPTION"
	labelDescription := "org.opencontainers.image.description"

	configContent := `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/true"],"ArgsEscaped":true,"Labels":{"maintainer":"` + maintainer + `","` + labelDescription + `":"` + description + `"},"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"container":"b89fe92a887d55c0961f02bdfbfd8ac3ddf66167db374770d2d9e9fab3311510","container_config":{"Hostname":"b89fe92a887d","Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/true\"]"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"created":"2022-01-01T00:00:00.000000000Z","docker_version":"20.10.12","history":[{"created":"2022-01-01T00:00:00.000000000Z","created_by":"/bin/sh -c #(nop) COPY file:0e7589b0c800daaf6fa460d2677101e4676dd9491980210cb345480e513f3602 in /true "},{"created":"2022-01-01T00:00:00.000000001Z","created_by":"/bin/sh -c #(nop)  CMD [\"/true\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0ff3b91bdf21ecdf2f2f3d4372c2098a14dbe06cd678e8f0a85fd4902d00e2e2"]}}`
	configDigest := "sha256:" + sha256Hash(configContent)
	configSize := fmt.Sprintf("%d", len(configContent))

	manifestContent := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":` + configSize + `},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4","size":32}]}`
	manifestDigest := "sha256:" + sha256Hash(manifestContent)

	untaggedManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageManifest + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":` + configSize + `},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4","size":32}]}`
	untaggedManifestDigest := "sha256:" + sha256Hash(untaggedManifestContent)

	indexManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageIndex + `","manifests":[{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","digest":"` + manifestDigest + `","platform":{"os":"linux","architecture":"arm","variant":"v7"}},{"mediaType":"` + oci.MediaTypeImageManifest + `","digest":"` + untaggedManifestDigest + `","platform":{"os":"linux","architecture":"arm64","variant":"v8"}}]}`
	indexManifestDigest := "sha256:" + sha256Hash(indexManifestContent)

	anonymousToken := ""
	readUserToken := ""
	userToken := ""

	t.Run("Authenticate", func(t *testing.T) {
		type TokenResponse struct {
			Token string `json:"token"`
		}

		authenticate := []string{`Bearer realm="` + setting.AppURL + `v2/token",service="container_registry",scope="*"`}

		t.Run("Anonymous", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			resp := MakeRequest(t, req, http.StatusUnauthorized)

			assert.ElementsMatch(t, authenticate, resp.Header().Values("WWW-Authenticate"))

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
			resp = MakeRequest(t, req, http.StatusOK)

			tokenResponse := &TokenResponse{}
			DecodeJSON(t, resp, &tokenResponse)

			assert.NotEmpty(t, tokenResponse.Token)

			anonymousToken = fmt.Sprintf("Bearer %s", tokenResponse.Token)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
				AddTokenAuth(anonymousToken)
			MakeRequest(t, req, http.StatusOK)

			defer test.MockVariableValue(&setting.Service.RequireSignInView, true)()

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
			MakeRequest(t, req, http.StatusUnauthorized)
		})

		t.Run("User", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			resp := MakeRequest(t, req, http.StatusUnauthorized)

			assert.ElementsMatch(t, authenticate, resp.Header().Values("WWW-Authenticate"))

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL)).
				AddBasicAuth(user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			tokenResponse := &TokenResponse{}
			DecodeJSON(t, resp, &tokenResponse)

			assert.NotEmpty(t, tokenResponse.Token)

			userToken = fmt.Sprintf("Bearer %s", tokenResponse.Token)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusOK)

			// Token that should enforce the read scope.
			t.Run("Read scope", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				session := loginUser(t, user.Name)
				token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadPackage)

				req := NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
				req.SetBasicAuth(user.Name, token)

				resp := MakeRequest(t, req, http.StatusOK)

				tokenResponse := &TokenResponse{}
				DecodeJSON(t, resp, &tokenResponse)

				assert.NotEmpty(t, tokenResponse.Token)

				readUserToken = fmt.Sprintf("Bearer %s", tokenResponse.Token)

				req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
					AddTokenAuth(readUserToken)
				MakeRequest(t, req, http.StatusOK)
			})
		})

		t.Run("No token issued if credentials are invalid", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
			// Setting the header explicitly instead of using AddBasicAuth to supply an invalid password.
			req.SetBasicAuth("user2", "very-invalid")
			resp := MakeRequest(t, req, http.StatusUnauthorized)

			assert.Equal(t, authenticate, resp.Header().Values("WWW-Authenticate"))
		})

		t.Run("Basic authentication", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			req.AddBasicAuth("does-not-exist")
			resp := MakeRequest(t, req, http.StatusUnauthorized)

			assert.Equal(t, authenticate, resp.Header().Values("WWW-Authenticate"))

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			req.AddBasicAuth(user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			assert.Empty(t, resp.Header().Get("WWW-Authenticate"))
		})

		t.Run("Basic authentication w/ Authorized Integration", func(t *testing.T) {
			ait := newAITester(t, func(ai *auth_model.AuthorizedIntegration) {
				ai.Scope = auth_model.AccessTokenScopeReadPackage
			})
			defer ait.close()
			token := ait.signedJWT()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
			req.SetBasicAuth(user.Name, token)

			resp := MakeRequest(t, req, http.StatusOK)

			tokenResponse := &TokenResponse{}
			DecodeJSON(t, resp, &tokenResponse)

			assert.NotEmpty(t, tokenResponse.Token)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
				AddTokenAuth(fmt.Sprintf("Bearer %s", tokenResponse.Token))
			MakeRequest(t, req, http.StatusOK)
		})
	})

	t.Run("DetermineSupport", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
			AddTokenAuth(userToken)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "registry/2.0", resp.Header().Get("Docker-Distribution-Api-Version"))
	})

	t.Run("ORAS Artifact Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		image := "oras-artifact"
		url := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, image)

		// Empty config blob (common in ORAS artifacts)
		emptyConfigDigest := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		emptyConfigContent := ""

		// Upload empty config blob
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, emptyConfigDigest), bytes.NewReader([]byte(emptyConfigContent))).
			AddTokenAuth(userToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, emptyConfigDigest), resp.Header().Get("Location"))
		assert.Equal(t, emptyConfigDigest, resp.Header().Get("Docker-Content-Digest"))

		// Verify empty blob exists and has correct Content-Length
		req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", url, emptyConfigDigest)).
			AddTokenAuth(userToken)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "0", resp.Header().Get("Content-Length")) // This was the main fix
		assert.Equal(t, emptyConfigDigest, resp.Header().Get("Docker-Content-Digest"))

		// Upload a small data blob (e.g., artifacthub metadata)
		artifactData := `{"name":"test-artifact","version":"1.0.0"}`
		artifactDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(artifactData)))

		req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, artifactDigest), bytes.NewReader([]byte(artifactData))).
			AddTokenAuth(userToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, artifactDigest), resp.Header().Get("Location"))

		// Create OCI artifact manifest
		artifactManifest := fmt.Sprintf(`{
			"schemaVersion": 2,
			"mediaType": "application/vnd.oci.image.manifest.v1+json",
			"artifactType": "application/vnd.cncf.artifacthub.config.v1+yaml",
			"config": {
				"mediaType": "application/vnd.cncf.artifacthub.config.v1+yaml",
				"digest": "%s",
				"size": %d
			},
			"layers": [
				{
					"mediaType": "application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml",
					"digest": "%s",
					"size": %d
				}
			]
		}`, emptyConfigDigest, len(emptyConfigContent), artifactDigest, len(artifactData))

		artifactManifestDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(artifactManifest)))

		// Upload artifact manifest
		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/artifact-v1", url), bytes.NewReader([]byte(artifactManifest))).
			AddTokenAuth(userToken).
			SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		resp = MakeRequest(t, req, http.StatusCreated)
		assert.Equal(t, fmt.Sprintf("/v2/%s/%s/manifests/artifact-v1", user.Name, image), resp.Header().Get("Location"))
		assert.Equal(t, artifactManifestDigest, resp.Header().Get("Docker-Content-Digest"))

		// Verify manifest can be retrieved
		req = NewRequest(t, "GET", fmt.Sprintf("%s/manifests/artifact-v1", url)).
			AddTokenAuth(userToken).
			SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "application/vnd.oci.image.manifest.v1+json", resp.Header().Get("Content-Type"))
		assert.Equal(t, artifactManifestDigest, resp.Header().Get("Docker-Content-Digest"))

		// Verify package was created with correct metadata
		pvs, err := packages_model.GetVersionsByPackageType(db.DefaultContext, user.ID, packages_model.TypeContainer)
		require.NoError(t, err)

		found := false
		for _, pv := range pvs {
			if pv.LowerVersion == "artifact-v1" {
				found = true
				break
			}
		}
		assert.True(t, found, "ORAS artifact package should be created")
	})

	for _, image := range images {
		t.Run(fmt.Sprintf("[Image:%s]", image), func(t *testing.T) {
			urlv2 := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, image)

			t.Run("UploadBlob/Monolithic", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", urlv2)).
					AddTokenAuth(anonymousToken)
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", urlv2)).
					AddTokenAuth(readUserToken)
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlv2, unknownDigest), bytes.NewReader(blobContent)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusBadRequest)

				req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlv2, blobDigest), bytes.NewReader(blobContent)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				pv, err := packages_model.GetInternalVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, container_model.UploadVersion)
				require.NoError(t, err)

				pfs, err := packages_model.GetFilesByVersionID(db.DefaultContext, pv.ID)
				require.NoError(t, err)
				assert.Len(t, pfs, 1)

				pb, err := packages_model.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
				require.NoError(t, err)
				assert.EqualValues(t, len(blobContent), pb.Size)
			})

			t.Run("UploadBlob/Chunked", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", urlv2)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusAccepted)

				uuid := resp.Header().Get("Docker-Upload-Uuid")
				assert.NotEmpty(t, uuid)

				pbu, err := packages_model.GetBlobUploadByID(db.DefaultContext, uuid)
				require.NoError(t, err)
				assert.EqualValues(t, 0, pbu.BytesReceived)

				uploadURL := resp.Header().Get("Location")
				assert.NotEmpty(t, uploadURL)

				req = NewRequestWithBody(t, "PATCH", setting.AppURL+uploadURL[1:]+"000", bytes.NewReader(blobContent)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequestWithBody(t, "PATCH", setting.AppURL+uploadURL[1:], bytes.NewReader(blobContent)).
					AddTokenAuth(userToken).
					SetHeader("Content-Range", "1-10")
				MakeRequest(t, req, http.StatusRequestedRangeNotSatisfiable)

				contentRange := fmt.Sprintf("0-%d", len(blobContent)-1)
				req.SetHeader("Content-Range", contentRange)
				resp = MakeRequest(t, req, http.StatusAccepted)

				assert.Equal(t, uuid, resp.Header().Get("Docker-Upload-Uuid"))
				assert.Equal(t, contentRange, resp.Header().Get("Range"))

				uploadURL = resp.Header().Get("Location")

				req = NewRequest(t, "GET", setting.AppURL+uploadURL[1:]).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusNoContent)

				assert.Equal(t, uuid, resp.Header().Get("Docker-Upload-Uuid"))
				assert.Equal(t, fmt.Sprintf("0-%d", len(blobContent)), resp.Header().Get("Range"))

				pbu, err = packages_model.GetBlobUploadByID(db.DefaultContext, uuid)
				require.NoError(t, err)
				assert.EqualValues(t, len(blobContent), pbu.BytesReceived)

				req = NewRequest(t, "PUT", fmt.Sprintf("%s?digest=%s", setting.AppURL+uploadURL[1:], blobDigest)).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				t.Run("Cancel", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", urlv2)).
						AddTokenAuth(userToken)
					resp := MakeRequest(t, req, http.StatusAccepted)

					uuid := resp.Header().Get("Docker-Upload-Uuid")
					assert.NotEmpty(t, uuid)

					uploadURL := resp.Header().Get("Location")
					assert.NotEmpty(t, uploadURL)

					req = NewRequest(t, "GET", setting.AppURL+uploadURL[1:]).
						AddTokenAuth(userToken)
					resp = MakeRequest(t, req, http.StatusNoContent)

					assert.Equal(t, uuid, resp.Header().Get("Docker-Upload-Uuid"))
					assert.Equal(t, "0-0", resp.Header().Get("Range"))

					req = NewRequest(t, "DELETE", setting.AppURL+uploadURL[1:]).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNoContent)

					req = NewRequest(t, "GET", setting.AppURL+uploadURL[1:]).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})
			})

			t.Run("UploadBlob/Mount", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				privateBlobDigest := "sha256:6ccce4863b70f258d691f59609d31b4502e1ba5199942d3bc5d35d17a4ce771d"
				req := NewRequestWithBody(t, "POST", fmt.Sprintf("%sv2/%s/%s/blobs/uploads?digest=%s", setting.AppURL, privateUser.Name, image, privateBlobDigest), strings.NewReader("gitea")).
					AddBasicAuth(privateUser.Name)
				MakeRequest(t, req, http.StatusCreated)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s", urlv2, unknownDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusAccepted)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s", urlv2, privateBlobDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusAccepted)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s", urlv2, blobDigest)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s&from=%s", urlv2, unknownDigest, "unknown/image")).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusAccepted)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s&from=%s/%s", urlv2, blobDigest, user.Name, image)).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))
			})

			for _, tag := range tags {
				t.Run(fmt.Sprintf("[Tag:%s]", tag), func(t *testing.T) {
					t.Run("UploadManifest", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()

						req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlv2, configDigest), strings.NewReader(configContent)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusCreated)

						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(anonymousToken).
							SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
						MakeRequest(t, req, http.StatusUnauthorized)

						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(readUserToken).
							SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
						MakeRequest(t, req, http.StatusUnauthorized)

						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(userToken).
							SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
						resp := MakeRequest(t, req, http.StatusCreated)

						assert.Equal(t, manifestDigest, resp.Header().Get("Docker-Content-Digest"))

						pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, tag)
						require.NoError(t, err)

						pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
						require.NoError(t, err)
						assert.Nil(t, pd.SemVer)
						assert.Equal(t, image, pd.Package.Name)
						assert.Equal(t, tag, pd.Version.Version)
						assert.ElementsMatch(t, []string{strings.ToLower(user.LowerName + "/" + image)}, getAllByName(pd.PackageProperties, container_module.PropertyRepository))
						assert.True(t, has(pd.VersionProperties, container_module.PropertyManifestTagged))

						assert.IsType(t, &container_module.Metadata{}, pd.Metadata)
						metadata := pd.Metadata.(*container_module.Metadata)
						assert.Equal(t, container_module.TypeOCI, metadata.Type)
						assert.Len(t, metadata.ImageLayers, 2)
						assert.Empty(t, metadata.Manifests)

						assert.Len(t, pd.Files, 3)
						for _, pfd := range pd.Files {
							switch pfd.File.Name {
							case container_model.ManifestFilename:
								assert.True(t, pfd.File.IsLead)
								assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", pfd.Properties.GetByName(container_module.PropertyMediaType))
								assert.Equal(t, manifestDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
							case strings.Replace(configDigest, ":", "_", 1):
								assert.False(t, pfd.File.IsLead)
								assert.Equal(t, "application/vnd.docker.container.image.v1+json", pfd.Properties.GetByName(container_module.PropertyMediaType))
								assert.Equal(t, configDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
							case strings.Replace(blobDigest, ":", "_", 1):
								assert.False(t, pfd.File.IsLead)
								assert.Equal(t, "application/vnd.docker.image.rootfs.diff.tar.gzip", pfd.Properties.GetByName(container_module.PropertyMediaType))
								assert.Equal(t, blobDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
							default:
								assert.FailNow(t, "unknown file", "name: %s", pfd.File.Name)
							}
						}

						req = NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", urlv2, tag)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusOK)

						pv, err = packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, tag)
						require.NoError(t, err)
						assert.EqualValues(t, 1, pv.DownloadCount)

						// Overwrite existing tag should keep the download count
						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(userToken).
							SetHeader("Content-Type", oci.MediaTypeImageManifest)
						MakeRequest(t, req, http.StatusCreated)

						pv, err = packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, tag)
						require.NoError(t, err)
						assert.EqualValues(t, 1, pv.DownloadCount)

						// Verify the template of the web UI renders correctly
						req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/-/packages/container/%s/%s", setting.AppURL, user.Name, url.QueryEscape(image), tag))
						resp = MakeRequest(t, req, http.StatusOK)
						htmlDoc := NewHTMLParser(t, resp.Body)
						assert.Contains(t, htmlDoc.Find("#container-description").Text(), description)
						assert.Contains(t, htmlDoc.Find("#container-layers").Text(), "CMD")
						assert.Contains(t, htmlDoc.Find("#container-labels").Text(), maintainer)
					})

					t.Run("HeadManifest", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()

						req := NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/unknown-tag", urlv2)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusNotFound)

						req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", urlv2, tag)).
							AddTokenAuth(userToken)
						resp := MakeRequest(t, req, http.StatusOK)

						assert.Equal(t, fmt.Sprintf("%d", len(manifestContent)), resp.Header().Get("Content-Length"))
						assert.Equal(t, manifestDigest, resp.Header().Get("Docker-Content-Digest"))
					})

					t.Run("GetManifest unknown-tag", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()

						req := NewRequest(t, "GET", fmt.Sprintf("%s/manifests/unknown-tag", urlv2)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusNotFound)
					})

					t.Run("GetManifest serv indirect", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()
						defer test.MockVariableValue(&setting.Packages.Storage.MinioConfig.ServeDirect, false)()

						req := NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", urlv2, tag)).
							AddTokenAuth(userToken)
						resp := MakeRequest(t, req, http.StatusOK)

						assert.Equal(t, fmt.Sprintf("%d", len(manifestContent)), resp.Header().Get("Content-Length"))
						assert.Equal(t, oci.MediaTypeImageManifest, resp.Header().Get("Content-Type"))
						assert.Equal(t, manifestDigest, resp.Header().Get("Docker-Content-Digest"))
						assert.Equal(t, manifestContent, resp.Body.String())
					})

					t.Run("GetManifest serv direct", func(t *testing.T) {
						if setting.Packages.Storage.Type != setting.MinioStorageType {
							t.Skip("Test skipped for non-Minio-storage.")
							return
						}

						defer tests.PrintCurrentTest(t)()
						defer test.MockVariableValue(&setting.Packages.Storage.MinioConfig.ServeDirect, true)()

						req := NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", urlv2, tag)).
							AddTokenAuth(userToken)
						resp := MakeRequest(t, req, http.StatusTemporaryRedirect)

						assert.Empty(t, resp.Header().Get("Content-Length"))
						assert.NotEmpty(t, resp.Header().Get("Location"))
						assert.Equal(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))
						assert.Empty(t, resp.Header().Get("Docker-Content-Digest"))
					})
				})
			}

			t.Run("UploadUntaggedManifest", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, untaggedManifestDigest), strings.NewReader(untaggedManifestContent)).
					AddTokenAuth(userToken).
					SetHeader("Content-Type", oci.MediaTypeImageManifest)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, untaggedManifestDigest, resp.Header().Get("Docker-Content-Digest"))

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", urlv2, untaggedManifestDigest)).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, fmt.Sprintf("%d", len(untaggedManifestContent)), resp.Header().Get("Content-Length"))
				assert.Equal(t, untaggedManifestDigest, resp.Header().Get("Docker-Content-Digest"))

				pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, untaggedManifestDigest)
				require.NoError(t, err)

				pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
				require.NoError(t, err)
				assert.Nil(t, pd.SemVer)
				assert.Equal(t, image, pd.Package.Name)
				assert.Equal(t, untaggedManifestDigest, pd.Version.Version)
				assert.ElementsMatch(t, []string{strings.ToLower(user.LowerName + "/" + image)}, getAllByName(pd.PackageProperties, container_module.PropertyRepository))
				assert.False(t, has(pd.VersionProperties, container_module.PropertyManifestTagged))

				assert.IsType(t, &container_module.Metadata{}, pd.Metadata)

				assert.Len(t, pd.Files, 3)
				for _, pfd := range pd.Files {
					if pfd.File.Name == container_model.ManifestFilename {
						assert.True(t, pfd.File.IsLead)
						assert.Equal(t, oci.MediaTypeImageManifest, pfd.Properties.GetByName(container_module.PropertyMediaType))
						assert.Equal(t, untaggedManifestDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
					}
				}
			})

			t.Run("UploadIndexManifest", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, multiTag), strings.NewReader(indexManifestContent)).
					AddTokenAuth(userToken).
					SetHeader("Content-Type", oci.MediaTypeImageIndex)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, indexManifestDigest, resp.Header().Get("Docker-Content-Digest"))

				pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, multiTag)
				require.NoError(t, err)

				pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
				require.NoError(t, err)
				assert.Nil(t, pd.SemVer)
				assert.Equal(t, image, pd.Package.Name)
				assert.Equal(t, multiTag, pd.Version.Version)
				assert.ElementsMatch(t, []string{strings.ToLower(user.LowerName + "/" + image)}, getAllByName(pd.PackageProperties, container_module.PropertyRepository))
				assert.True(t, has(pd.VersionProperties, container_module.PropertyManifestTagged))

				assert.ElementsMatch(t, []string{manifestDigest, untaggedManifestDigest}, getAllByName(pd.VersionProperties, container_module.PropertyManifestReference))

				assert.IsType(t, &container_module.Metadata{}, pd.Metadata)
				metadata := pd.Metadata.(*container_module.Metadata)
				assert.Equal(t, container_module.TypeOCI, metadata.Type)
				assert.Len(t, metadata.Manifests, 2)
				assert.Condition(t, func() bool {
					for _, m := range metadata.Manifests {
						switch m.Platform {
						case "linux/arm/v7":
							assert.Equal(t, manifestDigest, m.Digest)
						case "linux/arm64/v8":
							assert.Equal(t, untaggedManifestDigest, m.Digest)
						default:
							return false
						}
					}
					return true
				})

				assert.Len(t, pd.Files, 1)
				assert.True(t, pd.Files[0].File.IsLead)
				assert.Equal(t, oci.MediaTypeImageIndex, pd.Files[0].Properties.GetByName(container_module.PropertyMediaType))
				assert.Equal(t, indexManifestDigest, pd.Files[0].Properties.GetByName(container_module.PropertyDigest))

				// Verify the template of the web UI renders correctly
				req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/-/packages/container/%s/%s", setting.AppURL, user.Name, url.QueryEscape(image), multiTag))
				resp = MakeRequest(t, req, http.StatusOK)
				htmlDoc := NewHTMLParser(t, resp.Body)
				manifests := htmlDoc.Find("#container-manifests").Text()
				assert.Contains(t, manifests, manifestDigest)
				assert.Contains(t, manifests, untaggedManifestDigest)
			})

			t.Run("HeadBlob", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", urlv2, unknownDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", urlv2, blobDigest)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, fmt.Sprintf("%d", len(blobContent)), resp.Header().Get("Content-Length"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", urlv2, blobDigest)).
					AddTokenAuth(anonymousToken)
				MakeRequest(t, req, http.StatusOK)

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", urlv2, blobDigest)).
					AddTokenAuth(readUserToken)
				MakeRequest(t, req, http.StatusOK)
			})

			t.Run("GetBlob", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/blobs/%s", urlv2, unknownDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequest(t, "GET", fmt.Sprintf("%s/blobs/%s", urlv2, blobDigest)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, fmt.Sprintf("%d", len(blobContent)), resp.Header().Get("Content-Length"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))
				assert.Equal(t, blobContent, resp.Body.Bytes())
			})

			t.Run("GetTagList", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				var cases []struct {
					URL          string
					ExpectedTags []string
					ExpectedLink string
				}

				if image == "oras-artifact" {
					cases = []struct {
						URL          string
						ExpectedTags []string
						ExpectedLink string
					}{
						{
							URL:          fmt.Sprintf("%s/tags/list", urlv2),
							ExpectedTags: []string{"artifact-v1", "latest", "main", "multi"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=multi>; rel="next"`, user.Name, image),
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?n=0", urlv2),
							ExpectedTags: []string{},
							ExpectedLink: "",
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?n=2", urlv2),
							ExpectedTags: []string{"artifact-v1", "latest"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=latest&n=2>; rel="next"`, user.Name, image),
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?last=main", urlv2),
							ExpectedTags: []string{"multi"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=multi>; rel="next"`, user.Name, image),
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?n=1&last=latest", urlv2),
							ExpectedTags: []string{"main"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=main&n=1>; rel="next"`, user.Name, image),
						},
					}
				} else {
					cases = []struct {
						URL          string
						ExpectedTags []string
						ExpectedLink string
					}{
						{
							URL:          fmt.Sprintf("%s/tags/list", urlv2),
							ExpectedTags: []string{"latest", "main", "multi"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=multi>; rel="next"`, user.Name, image),
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?n=0", urlv2),
							ExpectedTags: []string{},
							ExpectedLink: "",
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?n=2", urlv2),
							ExpectedTags: []string{"latest", "main"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=main&n=2>; rel="next"`, user.Name, image),
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?last=main", urlv2),
							ExpectedTags: []string{"multi"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=multi>; rel="next"`, user.Name, image),
						},
						{
							URL:          fmt.Sprintf("%s/tags/list?n=1&last=latest", urlv2),
							ExpectedTags: []string{"main"},
							ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=main&n=1>; rel="next"`, user.Name, image),
						},
					}
				}

				for _, c := range cases {
					req := NewRequest(t, "GET", c.URL).
						AddTokenAuth(userToken)
					resp := MakeRequest(t, req, http.StatusOK)

					type TagList struct {
						Name string   `json:"name"`
						Tags []string `json:"tags"`
					}

					tagList := &TagList{}
					DecodeJSON(t, resp, &tagList)

					assert.Equal(t, user.Name+"/"+image, tagList.Name)
					assert.Equal(t, c.ExpectedTags, tagList.Tags)
					assert.Equal(t, c.ExpectedLink, resp.Header().Get("Link"))
				}

				req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s?type=container&q=%s", user.Name, image)).
					AddTokenAuth(token)
				resp := MakeRequest(t, req, http.StatusOK)

				var apiPackages []*api.Package
				DecodeJSON(t, resp, &apiPackages)
				if image == "oras-artifact" {
					assert.Len(t, apiPackages, 5) // "artifact-v1", "latest", "main", "multi", "sha256:..."
				} else {
					assert.Len(t, apiPackages, 4) // "latest", "main", "multi", "sha256:..."
				}
			})

			t.Run("Delete", func(t *testing.T) {
				t.Run("Blob", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/blobs/%s", urlv2, blobDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusAccepted)

					req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", urlv2, blobDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})

				t.Run("ManifestByDigest", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/manifests/%s", urlv2, untaggedManifestDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusAccepted)

					req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", urlv2, untaggedManifestDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})

				t.Run("ManifestByTag", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/manifests/%s", urlv2, multiTag)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusAccepted)

					req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", urlv2, multiTag)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})
			})
		})
	}

	// https://github.com/go-gitea/gitea/issues/19586
	t.Run("ParallelUpload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("%sv2/%s/parallel", setting.AppURL, user.Name)

		var wg sync.WaitGroup
		for i := range 10 {
			wg.Add(1)

			content := []byte{byte(i)}
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(content))

			go func() {
				defer wg.Done()

				req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, digest), bytes.NewReader(content)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, digest, resp.Header().Get("Docker-Content-Digest"))
			}()
		}
		wg.Wait()
	})

	t.Run("OwnerNameChange", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkCatalog := func(owner string) func(t *testing.T) {
			return func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%sv2/_catalog", setting.AppURL)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusOK)

				type RepositoryList struct {
					Repositories []string `json:"repositories"`
				}

				repoList := &RepositoryList{}
				DecodeJSON(t, resp, &repoList)

				assert.Len(t, repoList.Repositories, len(images))
				names := make([]string, 0, len(images))
				for _, image := range images {
					names = append(names, strings.ToLower(owner+"/"+image))
				}
				assert.ElementsMatch(t, names, repoList.Repositories)
			}
		}

		t.Run(fmt.Sprintf("Catalog[%s]", user.LowerName), checkCatalog(user.LowerName))

		session := loginUser(t, user.Name)

		newOwnerName := "newUsername"

		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"name":     newOwnerName,
			"email":    "user2@example.com",
			"language": "en-US",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		t.Run(fmt.Sprintf("Catalog[%s]", newOwnerName), checkCatalog(newOwnerName))

		req = NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"name":     user.Name,
			"email":    "user2@example.com",
			"language": "en-US",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
	})

	t.Run("AutoLinking", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// create repo which is used for auto-linking
		repo := createTestRepositoryWithPackageRegistry(t, user, "autolink-repo")

		// Test repo for the private user, used to test unauthorized auto-linking.
		// We don't need the repo object, but the name is used in the annotation pushed in the test.
		_ = createTestRepositoryWithPackageRegistry(t, privateUser, "autolink-repo")

		// some paths to push to
		urlExistingRepo := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, repo.Name)
		nameNonexistingRepo1 := "nonexisting-repo"
		urlNonexistingRepo1 := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, nameNonexistingRepo1)
		nameNonexistingRepo2 := "another-nonexisting-repo"
		urlNonexistingRepo2 := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, nameNonexistingRepo2)
		nameNonexistingRepo3 := "secret-repo"
		urlNonexistingRepo3 := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, nameNonexistingRepo3)
		nameNonexistingRepo4 := "more-repo-names-generator"
		urlNonexistingRepo4 := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, nameNonexistingRepo4)
		nameExistingRepoNested := "nested-image1"
		urlExistingRepoNested := fmt.Sprintf("%sv2/%s/%s/%s", setting.AppURL, user.Name, repo.Name, nameExistingRepoNested)

		// variable to hold an auto-linked package, which will be unlinked again in a later test
		var linkedPackage *packages_model.Package

		t.Run("PushToArbitraryRepo", func(t *testing.T) {
			// Upload blobs and manifest
			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo1, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo1, configDigest), strings.NewReader(configContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlNonexistingRepo1, "v1"), strings.NewReader(manifestContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated)

			p, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, nameNonexistingRepo1)
			require.NoError(t, err)
			require.Equal(t, nameNonexistingRepo1, p.Name) // just to make sure we have grabbed the correct package
			assert.Equal(t, int64(0), p.RepoID)
		})

		t.Run("PushToExisingRepo", func(t *testing.T) {
			// Upload blobs and manifest which should create a package with tag "v1"
			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlExistingRepo, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlExistingRepo, configDigest), strings.NewReader(configContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlExistingRepo, "v1"), strings.NewReader(manifestContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated)

			// get the resulting package
			p, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, repo.Name)
			require.NoError(t, err)
			require.Equal(t, repo.Name, p.Name) // just to make sure we have grabbed the correct package
			assert.Equal(t, repo.ID, p.RepoID)
			linkedPackage = p // store auto-linked package for the next test
		})

		t.Run("PushToExistingRepoNested", func(t *testing.T) {
			// Upload blobs and manifest which should create a package with tag "v1"
			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlExistingRepoNested, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlExistingRepoNested, configDigest), strings.NewReader(configContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlExistingRepoNested, "v1"), strings.NewReader(manifestContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated)

			// get the resulting package
			p, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, repo.Name+"/"+nameExistingRepoNested)
			require.NoError(t, err)
			require.Equal(t, repo.Name+"/"+nameExistingRepoNested, p.Name) // just to make sure we have grabbed the correct package
			assert.Equal(t, repo.ID, p.RepoID)
		})

		t.Run("PushVersionToUnlinkedRepo", func(t *testing.T) {
			// unlink previously auto-linked package
			require.NoError(t,
				packages_service.UnlinkFromRepository(t.Context(), linkedPackage, user),
			)
			// test if correctly unlinked
			checkPackageForUnlinked, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, repo.Name)
			require.NoError(t, err)
			require.Equal(t, int64(0), checkPackageForUnlinked.RepoID)

			// push updated version (e.g. tag v2)
			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlExistingRepo, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlExistingRepo, configDigest), strings.NewReader(configContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlExistingRepo, "v2"), strings.NewReader(manifestContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated)

			// test if still unlinked
			checkPackageForStillUnlinked, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, repo.Name)
			require.NoError(t, err)
			assert.Equal(t, int64(0), checkPackageForStillUnlinked.RepoID)
		})

		t.Run("PushWithLabel", func(t *testing.T) {
			// Pushes to non-existing path but tries to link using an image label.

			// same as configContent, but with the added label in config: "org.opencontainers.image.source": "{AppURL}/user2/autolink-repo"
			configWithOpenContainersSourceLabelContent := `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/true"],"ArgsEscaped":true,"Labels":{"org.opencontainers.image.source":"` + setting.AppURL + `user2/autolink-repo"},"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"container":"b89fe92a887d55c0961f02bdfbfd8ac3ddf66167db374770d2d9e9fab3311510","container_config":{"Hostname":"b89fe92a887d","Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/true\"]"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"created":"2022-01-01T00:00:00.000000000Z","docker_version":"20.10.12","history":[{"created":"2022-01-01T00:00:00.000000000Z","created_by":"/bin/sh -c #(nop) COPY file:0e7589b0c800daaf6fa460d2677101e4676dd9491980210cb345480e513f3602 in /true "},{"created":"2022-01-01T00:00:00.000000001Z","created_by":"/bin/sh -c #(nop)  CMD [\"/true\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0ff3b91bdf21ecdf2f2f3d4372c2098a14dbe06cd678e8f0a85fd4902d00e2e2"]}}`
			configWithOpenContainersSourceLabelDigest := "sha256:" + sha256Hash(configWithOpenContainersSourceLabelContent)
			manifestWithOpenContainersSourceLabelContent := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configWithOpenContainersSourceLabelDigest + `","size":` + strconv.Itoa(len(configWithOpenContainersSourceLabelContent)) + `},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}]}`

			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo2, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo2, configWithOpenContainersSourceLabelDigest), strings.NewReader(configWithOpenContainersSourceLabelContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlNonexistingRepo2, "v1"), strings.NewReader(manifestWithOpenContainersSourceLabelContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated)

			p, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, nameNonexistingRepo2)
			require.NoError(t, err)
			require.Equal(t, nameNonexistingRepo2, p.Name) // just to make sure we have grabbed the correct package
			assert.Equal(t, repo.ID, p.RepoID)
		})

		t.Run("PushWithAnnotation", func(t *testing.T) {
			// Pushes to non-existing path but tries to link using a push annotation in the manifest.

			// same as configContent, but with the added annotation directly within the manifest: "org.opencontainers.image.source": "{AppURL}/user2/autolink-repo"
			manifestWithOpenContainersSourceAnnotationContent := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":` + configSize + `},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}],"annotations":{"org.opencontainers.image.source":"` + setting.AppURL + `user2/autolink-repo"}}`

			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo3, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo3, configDigest), strings.NewReader(configContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlNonexistingRepo3, "v1"), strings.NewReader(manifestWithOpenContainersSourceAnnotationContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated)

			p, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, nameNonexistingRepo3)
			require.NoError(t, err)
			require.Equal(t, nameNonexistingRepo3, p.Name) // just to make sure we have grabbed the correct package
			assert.Equal(t, repo.ID, p.RepoID)
		})

		t.Run("PushWithAnnotationNoPermissions", func(t *testing.T) {
			// This tests pushes a manifest as user2, but tries to link to an existing repo of user31.
			// This should fail silently with the created package not automatically getting linked.

			// same as configContent above (also uses blob[Digest/Content]), but with an added annotation to auto-link to a repo of the private user: "org.opencontainers.image.source": "{AppURL}/user31/autolink-repo"
			manifestWithOpenContainersSourceAnnotationPrivateUserContent := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":` + configSize + `},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}],"annotations":{"org.opencontainers.image.source":"` + setting.AppURL + `user31/autolink-repo"}}`

			req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo4, blobDigest), bytes.NewReader(blobContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlNonexistingRepo4, configDigest), strings.NewReader(configContent)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlNonexistingRepo4, "v1"), strings.NewReader(manifestWithOpenContainersSourceAnnotationPrivateUserContent)).
				AddTokenAuth(userToken).
				SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			MakeRequest(t, req, http.StatusCreated) // wrongly annotated pushes still get pushed, but not auto linked

			p, err := packages_model.GetPackageByName(t.Context(), user.ID, packages_model.TypeContainer, nameNonexistingRepo4)
			require.NoError(t, err)
			require.Equal(t, nameNonexistingRepo4, p.Name) // just to make sure we have grabbed the correct package
			assert.Equal(t, int64(0), p.RepoID)            // ensure not linked
		})
	})

	t.Run("ManifestAnnotationsStored", func(t *testing.T) {
		// Pushes a manifest with OCI annotations and verifies they are stored in the package metadata
		// and rendered in the web UI.
		defer tests.PrintCurrentTest(t)()

		annotationImage := "annotations-test"
		annotationTag := "v1"
		urlv2 := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, annotationImage)

		// same as manifestContent, but with added annotations in the manifest
		annotationManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageManifest + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":` + configSize + `},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}],"annotations":{"org.opencontainers.image.created":"2022-01-01T00:00:00Z","custom.test.key":"custom-test-value"}}`

		req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlv2, blobDigest), bytes.NewReader(blobContent)).
			AddTokenAuth(userToken)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", urlv2, configDigest), strings.NewReader(configContent)).
			AddTokenAuth(userToken)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", urlv2, annotationTag), strings.NewReader(annotationManifestContent)).
			AddTokenAuth(userToken).
			SetHeader("Content-Type", oci.MediaTypeImageManifest)
		MakeRequest(t, req, http.StatusCreated)

		pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, annotationImage, annotationTag)
		require.NoError(t, err)

		pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
		require.NoError(t, err)

		metadata := pd.Metadata.(*container_module.Metadata)
		assert.Equal(t, map[string]string{
			"org.opencontainers.image.created": "2022-01-01T00:00:00Z",
			"custom.test.key":                  "custom-test-value",
		}, metadata.Annotations)

		// Verify the template of the web UI renders annotations correctly
		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/-/packages/container/%s/%s", setting.AppURL, user.Name, url.QueryEscape(annotationImage), annotationTag))
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		annotations := htmlDoc.Find("#container-annotations").Text()
		assert.Contains(t, annotations, "org.opencontainers.image.created")
		assert.Contains(t, annotations, "2022-01-01T00:00:00Z")
		assert.Contains(t, annotations, "custom.test.key")
		assert.Contains(t, annotations, "custom-test-value")
	})
}

func createTestRepositoryWithPackageRegistry(t *testing.T, user *user_model.User, name string) *repo_model.Repository {
	ctx := NewAPITestContext(t, user.Name, name, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	t.Run("CreateRepo", doAPICreateRepository(ctx, nil, git.Sha1ObjectFormat, func(t *testing.T, r api.Repository) {
		require.True(t, r.HasPackages)
	}))

	repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, user.Name, name)
	require.NoError(t, err)

	return repo
}

func sha256Hash(in string) string {
	sum := sha256.Sum256([]byte(in))
	return hex.EncodeToString(sum[:])
}
