// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/models/packages"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/json"
	"forgejo.org/modules/packages/pypi"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackagePyPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "test-package"
	packageVersion := "1!1.0.1+r1234"
	packageAuthor := "KN4CK3R"
	packageDescription := "Test Description"
	projectURL := "https://example.com"

	content := "test"
	hashSHA256 := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	root := fmt.Sprintf("/api/packages/%s/pypi", user.Name)

	createBasicMultipartFile := func(filename, packageName, content string) (body *bytes.Buffer, writer *multipart.Writer, closer func() error) {
		body = &bytes.Buffer{}
		writer = multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("content", filename)
		_, _ = io.Copy(part, strings.NewReader(content))

		writer.WriteField("name", packageName)
		writer.WriteField("version", packageVersion)
		writer.WriteField("author", packageAuthor)
		writer.WriteField("summary", packageDescription)
		writer.WriteField("description", packageDescription)
		writer.WriteField("sha256_digest", hashSHA256)
		writer.WriteField("requires_python", "3.6")

		return body, writer, writer.Close
	}

	uploadHelper := func(t *testing.T, body *bytes.Buffer, contentType string, expectedStatus int) {
		req := NewRequestWithBody(t, "POST", root, body).
			SetHeader("Content-Type", contentType).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	uploadFile := func(t *testing.T, filename, content string, expectedStatus int) {
		body, writer, closeFunc := createBasicMultipartFile(filename, packageName, content)

		writer.WriteField("project_urls", "DOCUMENTATION , https://readthedocs.org")
		writer.WriteField("project_urls", fmt.Sprintf("Home-page, %s", projectURL))

		_ = closeFunc()

		uploadHelper(t, body, writer.FormDataContentType(), expectedStatus)
	}

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		filename := "test.whl"
		uploadFile(t, filename, content, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypePyPI)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.IsType(t, &pypi.Metadata{}, pd.Metadata)
		assert.Equal(t, projectURL, pd.Metadata.(*pypi.Metadata).ProjectURL)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		require.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		require.NoError(t, err)
		assert.Equal(t, int64(4), pb.Size)
	})

	t.Run("UploadAddFile", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		filename := "test.tar.gz"
		uploadFile(t, filename, content, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypePyPI)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.IsType(t, &pypi.Metadata{}, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		require.NoError(t, err)
		assert.Len(t, pfs, 2)

		pf, err := packages.GetFileForVersionByName(db.DefaultContext, pvs[0].ID, filename, packages.EmptyFileKey)
		require.NoError(t, err)
		assert.Equal(t, filename, pf.Name)
		assert.True(t, pf.IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
		require.NoError(t, err)
		assert.Equal(t, int64(4), pb.Size)
	})

	t.Run("UploadHashMismatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		filename := "test2.whl"
		uploadFile(t, filename, "dummy", http.StatusBadRequest)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadFile(t, "test.whl", content, http.StatusConflict)
		uploadFile(t, "test.tar.gz", content, http.StatusConflict)
	})

	t.Run("UploadUsingDeprecatedHomepageMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		pkgName := "homepage-package"
		body, writer, closeFunc := createBasicMultipartFile("test.whl", pkgName, content)

		writer.WriteField("home_page", projectURL)

		_ = closeFunc()

		uploadHelper(t, body, writer.FormDataContentType(), http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageName(db.DefaultContext, user.ID, packages.TypePyPI, pkgName)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.IsType(t, &pypi.Metadata{}, pd.Metadata)
		assert.Equal(t, projectURL, pd.Metadata.(*pypi.Metadata).ProjectURL)
	})

	t.Run("UploadWithoutAnyHomepageURLMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		pkgName := "no-project-url-or-homepage-package"
		body, writer, closeFunc := createBasicMultipartFile("test.whl", pkgName, content)

		_ = closeFunc()

		uploadHelper(t, body, writer.FormDataContentType(), http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageName(db.DefaultContext, user.ID, packages.TypePyPI, pkgName)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.IsType(t, &pypi.Metadata{}, pd.Metadata)
		assert.Empty(t, pd.Metadata.(*pypi.Metadata).ProjectURL)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		downloadFile := func(filename string) {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/files/%s/%s/%s", root, packageName, packageVersion, filename)).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, []byte(content), resp.Body.Bytes())
		}

		downloadFile("test.whl")
		downloadFile("test.tar.gz")

		pvs, err := packages.GetVersionsByPackageName(db.DefaultContext, user.ID, packages.TypePyPI, packageName)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(2), pvs[0].DownloadCount)
	})

	hrefMatcher := regexp.MustCompile(fmt.Sprintf(`%s/files/%s/%s/test\..+#sha256=%s`, root, regexp.QuoteMeta(packageName), regexp.QuoteMeta(packageVersion), hashSHA256))

	t.Run("PackageMetadataHTML", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		req.Header["Accept"] = []string{"application/vnd.pypi.simple.v1+html"}
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, []string{"application/vnd.pypi.simple.v1+html"}, resp.Header().Values("Content-Type"))

		htmlDoc := NewHTMLParser(t, resp.Body)
		nodes := htmlDoc.doc.Find("a").Nodes
		assert.Len(t, nodes, 2)

		for _, a := range nodes {
			for _, att := range a.Attr {
				switch att.Key {
				case "href":
					assert.Regexp(t, hrefMatcher, att.Val)
				case "data-requires-python":
					assert.Equal(t, "3.6", att.Val)
				default:
					t.Fail()
				}
			}
		}
	})

	t.Run("PackageMetadataJSON", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		req.Header["Accept"] = []string{"application/vnd.pypi.simple.v1+json"}
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, []string{"application/vnd.pypi.simple.v1+json"}, resp.Header().Values("Content-Type"))
		assert.Greater(t, resp.Body.Len(), 3)
		txt := make([]byte, resp.Body.Len())
		resp.Body.Read(txt)
		var obj pypi.PackageJSON
		require.NoError(t, json.Unmarshal(txt, &obj))
		assert.Equal(t, packageName, obj.Name)
		assert.Equal(t, pypi.PackageMetaJSON{APIVersion: "1.4"}, obj.Meta)
		for _, filed := range obj.Files {
			hrefMatcher = regexp.MustCompile(fmt.Sprintf(`%s/files/%s/%s/test\.(tar\.gz)|(whl)`, root, regexp.QuoteMeta(packageName), regexp.QuoteMeta(packageVersion)))
			assert.Regexp(t, hrefMatcher, filed.URL[21:])
		}
	})

	checkMetadataJSON := func(t *testing.T, accept string) {
		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		req.Header["Accept"] = []string{accept}
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, []string{"application/vnd.pypi.simple.v1+json"}, resp.Header().Values("Content-Type"))
		var obj pypi.PackageJSON
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &obj))
		assert.Equal(t, packageName, obj.Name)
	}

	checkMetadataHTML := func(t *testing.T, accept, expectedContentType string) {
		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		req.Header["Accept"] = []string{accept}
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, []string{expectedContentType}, resp.Header().Values("Content-Type"))
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Len(t, htmlDoc.doc.Find("a").Nodes, 2)
	}

	t.Run("PackageMetadataLatestJSON", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkMetadataJSON(t, "application/vnd.pypi.simple.latest+json")
	})

	t.Run("PackageMetadataLatestHTML", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkMetadataHTML(t, "application/vnd.pypi.simple.latest+html", "application/vnd.pypi.simple.v1+html")
	})

	t.Run("PackageMetadataTextHTML", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkMetadataHTML(t, "text/html", "text/html")
	})

	t.Run("PackageMetadataPreferredByQuality", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// the media type with the higher q value wins, regardless of header order
		checkMetadataJSON(t, "text/html;q=0.5, application/vnd.pypi.simple.v1+json")
		checkMetadataHTML(t, "application/vnd.pypi.simple.v1+json;q=0.5, text/html", "text/html")
	})

	t.Run("PackageMetadataNotAcceptable", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		req.Header["Accept"] = []string{"application/xml"}
		MakeRequest(t, req, http.StatusNotAcceptable)
	})

	t.Run("PackageMetadataWithoutAcceptHeader", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// PEP 691: clients that do not send an Accept header default to HTML
		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, []string{"text/html"}, resp.Header().Values("Content-Type"))
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Len(t, htmlDoc.doc.Find("a").Nodes, 2)
	})

	t.Run("PackageMetadataWildcardAccept", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// `*/*` matches any representation, fall back to HTML
		checkMetadataHTML(t, "*/*", "text/html")
	})
}
