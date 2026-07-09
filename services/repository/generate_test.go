// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var giteaTemplate = []byte(`
# Header

# All .go files
**.go

# All text files in /text/
text/*.txt

# All files in modules folders
**/modules/*
`)

func TestGiteaTemplate(t *testing.T) {
	gt := GiteaTemplate{Content: giteaTemplate}
	assert.Len(t, gt.Globs(), 3)

	tt := []struct {
		Path  string
		Match bool
	}{
		{Path: "main.go", Match: true},
		{Path: "a/b/c/d/e.go", Match: true},
		{Path: "main.txt", Match: false},
		{Path: "a/b.txt", Match: false},
		{Path: "text/a.txt", Match: true},
		{Path: "text/b.txt", Match: true},
		{Path: "text/c.json", Match: false},
		{Path: "a/b/c/modules/README.md", Match: true},
		{Path: "a/b/c/modules/d/README.md", Match: false},
	}

	for _, tc := range tt {
		t.Run(tc.Path, func(t *testing.T) {
			match := false
			for _, g := range gt.Globs() {
				if g.Match(tc.Path) {
					match = true
					break
				}
			}
			assert.Equal(t, tc.Match, match)
		})
	}
}

func TestFileNameSanitize(t *testing.T) {
	assert.Equal(t, "test_CON", fileNameSanitize("test_CON"))
	assert.Equal(t, "test CON", fileNameSanitize("test CON "))
	assert.Equal(t, "__traverse__", fileNameSanitize("../traverse/.."))
	assert.Equal(t, "http___localhost_3003_user_test.git", fileNameSanitize("http://localhost:3003/user/test.git"))
	assert.Equal(t, "_", fileNameSanitize("CON"))
	assert.Equal(t, "_", fileNameSanitize("con"))
	assert.Equal(t, "_", fileNameSanitize("\u0000"))
	assert.Equal(t, "目标", fileNameSanitize("目标"))
}

func TestExpandTemplateVars(t *testing.T) {
	expansionMap := map[string]string{
		"REPO_NAME":           "my-repo",
		"REPO_NAME_SNAKE":     "my_repo",
		"REPO_OWNER":          "alice",
		"REPO_OWNER_NOT_SANE": "../alice/..",
	}

	perlBareVariable := "perl -e '$REPO_NAME::suffix=value ; print $REPO_NAME::suffix=value'"
	perlBracedVariable := "perl -e '${REPO_NAME::suffix=value} ; print ${REPO_NAME::suffix=value}'"

	tests := []struct {
		name             string
		src              string
		sanitizeFileName bool
		expected         string
	}{
		{"bare known var", "$REPO_NAME", false, "my-repo"},
		{"braced known var", "${REPO_NAME}", false, "my-repo"},
		{"transformer suffix", "${REPO_NAME_SNAKE}", false, "my_repo"},
		{"mixed", "$REPO_OWNER/${REPO_NAME}", false, "alice/my-repo"},
		{"unknown braced var", "${UNKNOWN_VAR}", false, "${UNKNOWN_VAR}"},
		{"unknown bare var", "$UNKNOWN_VAR", false, "$UNKNOWN_VAR"},
		{"unknown in context", "Authorization: token ${AUTH_TOKEN}", false, "Authorization: token ${AUTH_TOKEN}"},
		{"known prefix of unknown token", "$REPO_NAMEX", false, "$REPO_NAMEX"},
		{"double dollar unknown", "$$UNKNOWN_VAR", false, "$$UNKNOWN_VAR"},
		{"double dollar known", "$$REPO_NAME", false, "$my-repo"},
		{"one letter variable", "cost is $5", false, "cost is $5"},
		{"lone dollar", "lonely $", false, "lonely $"},
		{"sanitize sane filename in known var", "${REPO_OWNER}", true, "alice"},
		{"sanitize not sane filename in known var", "${REPO_OWNER_NOT_SANE}", true, "__alice__"},
		{"bare perl variables may be replaced", perlBareVariable, false, "perl -e 'my-repo::suffix=value ; print my-repo::suffix=value'"},
		{"braced perl variables will not be replaced", perlBracedVariable, false, perlBracedVariable},
		{"multiline", "line0\nline1 $REPO_OWNER \nline2 ${REPO_NAME}\n", false, "line0\nline1 alice \nline2 my-repo\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, expandTemplateVars(tt.src, expansionMap, tt.sanitizeFileName))
		})
	}
}

func TestTransformers(t *testing.T) {
	input := "Foo_Forgejo-BAR"

	tests := []struct {
		name     string
		expected string
	}{
		{"SNAKE", "foo_forgejo_bar"},
		{"KEBAB", "foo-forgejo-bar"},
		{"CAMEL", "fooForgejoBar"},
		{"PASCAL", "FooForgejoBar"},
		{"LOWER", "foo_forgejo-bar"},
		{"UPPER", "FOO_FORGEJO-BAR"},
		{"TITLE", "Foo_forgejo-Bar"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := defaultTransformers[i]
			assert.Equal(t, tt.name, transform.Name)

			got := transform.Transform(input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
