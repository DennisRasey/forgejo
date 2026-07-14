// Copyright 2026 The Forgejo Authors. All rights reserved
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// error tests
var testVKCerr = []struct {
	name      string
	cfgline   string
	complaint string
}{
	{
		name:      "nosep",
		cfgline:   "X_KEYS_ACCEPTED = foobar",
		complaint: "Seperator : not found in",
	},
	{
		name:      "badalg",
		cfgline:   "X_KEYS_ACCEPTED = abc:def",
		complaint: "Invalid algorithm",
	},
	{
		name:      "SymSchemeNotFile",
		cfgline:   "X_KEYS_ACCEPTED = HS256:http://not.yet/supported",
		complaint: "Unsupported URI-Scheme \"http\"",
	},
	{
		name:      "AsymSchemeNotFile",
		cfgline:   "X_KEYS_ACCEPTED = RS256:http://not.yet/supported",
		complaint: "Unsupported URI-Scheme \"http\"",
	},
	{
		name:      "SymDecodeErr",
		cfgline:   "X_KEYS_ACCEPTED = HS256:abcdef",
		complaint: "invalid base64 decoded length",
	},
}

func TestLoadVerificationKeyCfgErr(t *testing.T) {
	cfgSec := "foo"
	cfgBase := fmt.Sprintf("[%s]\n", cfgSec)

	for _, spec := range testVKCerr {
		t.Run(spec.name, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(cfgBase + spec.cfgline)
			require.NoError(t, err)
			assert.NotNil(t, cfg)
			_, err = loadVerificationKeyCfg(cfg, cfgSec, "X_")
			require.Error(t, err)
			require.ErrorContains(t, err, spec.complaint)
		})
	}
}

// OK tests
var (
	// symmetric key test data b64 / binary
	testVKCs1 = "ForgejoForgejoForgejoForgejoForgejoForgejo_"
	testVKCs2 = "forgejoforgejoforgejoforgejoforgejoforgejo_"
	testVKCb1 = []byte{
		0x16, 0x8a, 0xe0, 0x7a, 0x3a, 0x05, 0xa2, 0xb8, 0x1e, 0x8e, 0x81, 0x68, 0xae, 0x07, 0xa3, 0xa0, 0x5a, 0x2b, 0x81, 0xe8, 0xe8, 0x16, 0x8a, 0xe0, 0x7a, 0x3a, 0x05, 0xa2, 0xb8, 0x1e, 0x8e, 0x8f,
	}
	testVKCb2 = []byte{
		0x7e, 0x8a, 0xe0, 0x7a, 0x3a, 0x1f, 0xa2, 0xb8, 0x1e, 0x8e, 0x87, 0xe8, 0xae, 0x07, 0xa3, 0xa1, 0xfa, 0x2b, 0x81, 0xe8, 0xe8, 0x7e, 0x8a, 0xe0, 0x7a, 0x3a, 0x1f, 0xa2, 0xb8, 0x1e, 0x8e, 0x8f,
	}
	// files
	testVKCfiles = map[string]*struct {
		content string // file does not get created if content == ""
		path    string // set by initializer
	}{
		"s1":             {testVKCs1, "X1"},
		"s2":             {testVKCs2, "X2"},
		"file1.pub":      {"fileisnotread", "X3"},
		"file2.pub":      {"sodoesntmatter", "X4"},
		"file3.notexist": {"", "X5"},
	}
	testVKC = []struct {
		name    string
		cfgline string
		vcfgs   []*jwtx.VerificationKeyCfg
	}{
		{
			name:    "noConfig",
			cfgline: "",
			vcfgs:   nil,
		},
		{
			name:    "emptyConfig",
			cfgline: "X_KEYS_ACCEPTED = ",
			vcfgs:   nil,
		},
		{
			name:    "1LiteralSecret",
			cfgline: "X_KEYS_ACCEPTED = HS256:" + testVKCs1,
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS256",
					SecretBytes: &testVKCb1,
				},
			},
		},
		{
			name:    "2LiteralSecrets",
			cfgline: "X_KEYS_ACCEPTED = HS512:" + testVKCs1 + " HS256:" + testVKCs2,
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS512",
					SecretBytes: &testVKCb1,
				},
				{
					Algorithm:   "HS256",
					SecretBytes: &testVKCb2,
				},
			},
		},
		{
			name:    "SecretFiles",
			cfgline: "X_KEYS_ACCEPTED = HS512:file:s1 HS256:file:s2",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS512",
					SecretBytes: &testVKCb1,
				},
				{
					Algorithm:   "HS256",
					SecretBytes: &testVKCb2,
				},
			},
		},
		{
			name:    "SecretFiles Glob",
			cfgline: "X_KEYS_ACCEPTED = HS512:file:s?",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS512",
					SecretBytes: &testVKCb1,
				},
				{
					Algorithm:   "HS512",
					SecretBytes: &testVKCb2,
				},
			},
		},
		{
			name:    "SecretFile ./",
			cfgline: "X_KEYS_ACCEPTED = HS384:file:./s1",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS384",
					SecretBytes: &testVKCb1,
				},
			},
		},
		{
			name:    "SecretFile //localhost",
			cfgline: "X_KEYS_ACCEPTED = HS512:file://localhost%{AppDataPath}/s1",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS512",
					SecretBytes: &testVKCb1,
				},
			},
		},
		{
			name:    "SecretFile // (empty host)",
			cfgline: "X_KEYS_ACCEPTED = HS512:file://%{AppDataPath}/s1",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:   "HS512",
					SecretBytes: &testVKCb1,
				},
			},
		},
		{
			name:    "PublicKeyFile",
			cfgline: "X_KEYS_ACCEPTED = RS256:file:file1.pub",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:     "RS256",
					PublicKeyPath: &testVKCfiles["file1.pub"].path,
				},
			},
		},
		{
			name:    "PublicKeyFile Glob",
			cfgline: "X_KEYS_ACCEPTED = RS256:file:file*.pub",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:     "RS256",
					PublicKeyPath: &testVKCfiles["file1.pub"].path,
				},
				{
					Algorithm:     "RS256",
					PublicKeyPath: &testVKCfiles["file2.pub"].path,
				},
			},
		},
		{
			name:    "PublicKeyFile ./",
			cfgline: "X_KEYS_ACCEPTED = EdDSA:file:./file1.pub",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:     "EdDSA",
					PublicKeyPath: &testVKCfiles["file1.pub"].path,
				},
			},
		},
		{
			name:    "PublicKeyFile //localhost",
			cfgline: "X_KEYS_ACCEPTED = RS256:file://localhost%{AppDataPath}/file1.pub",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:     "RS256",
					PublicKeyPath: &testVKCfiles["file1.pub"].path,
				},
			},
		},
		{
			name:    "PublicKeyFile // (empty host)",
			cfgline: "X_KEYS_ACCEPTED = ES256:file://%{AppDataPath}/file1.pub",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:     "ES256",
					PublicKeyPath: &testVKCfiles["file1.pub"].path,
				},
			},
		},
		{
			name:    "PublicKeyFile does not exist (failing glob match keeps file)",
			cfgline: "X_KEYS_ACCEPTED = ES256:file:file3.notexist",
			vcfgs: []*jwtx.VerificationKeyCfg{
				{
					Algorithm:     "ES256",
					PublicKeyPath: &testVKCfiles["file3.notexist"].path,
				},
			},
		},
	}
)

func TestLoadVerificationKeyCfg(t *testing.T) {
	defer test.MockVariableValue(&AppDataPath, t.TempDir())()

	// init files
	for fn, spec := range testVKCfiles {
		path := filepath.Join(AppDataPath, fn)
		spec.path = path
		if spec.content == "" {
			continue
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		require.NoError(t, err)
		l, err := f.Write([]byte(spec.content))
		require.NoError(t, err)
		require.Equal(t, len(spec.content), l)
		require.NoError(t, f.Close())
	}

	cfgSec := "foo"
	cfgBase := fmt.Sprintf("[%s]\n", cfgSec)

	for _, spec := range testVKC {
		t.Run(spec.name, func(t *testing.T) {
			cfgString := cfgBase + strings.ReplaceAll(spec.cfgline, "%{AppDataPath}", AppDataPath)
			cfg, err := NewConfigProviderFromData(cfgString)
			require.NoError(t, err)
			assert.NotNil(t, cfg)
			vcfgs, err := loadVerificationKeyCfg(cfg, cfgSec, "X_")
			require.NoError(t, err)
			require.Equal(t, spec.vcfgs, vcfgs)
		})
	}
}
