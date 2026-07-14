// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"forgejo.org/modules/auth/password/hash"
	"forgejo.org/modules/generate"
	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/keying"
	"forgejo.org/modules/log"
)

var (
	// Security settings
	InstallLock                        bool
	SecretKey                          string
	InternalToken                      string // internal access token
	LogInRememberDays                  int
	GlobalTwoFactorRequirement         TwoFactorRequirementType
	CookieRememberName                 string
	ReverseProxyAuthUser               string
	ReverseProxyAuthEmail              string
	ReverseProxyAuthFullName           string
	ReverseProxyLimit                  int
	ReverseProxyTrustedProxies         []string
	MinPasswordLength                  int
	ImportLocalPaths                   bool
	DisableGitHooks                    bool
	DisableWebhooks                    bool
	OnlyAllowPushIfGiteaEnvironmentSet bool
	PasswordComplexity                 []string
	PasswordHashAlgo                   string
	PasswordCheckPwn                   bool
	SuccessfulTokensCacheSize          int
	DisableQueryAuthToken              bool
)

/*
 * key loading is a two-stage process to avoid complications for unit tests:
 *
 * For symmetric keys, we want to add a random key to the configuration. We would
 * not want to change the configuration after loading has completed to maintain
 * isolation. So from this perspective, we would want to initialize keys only
 * during setting.load...From()
 *
 * For asymmetric keys, we want to create a random private key _file_.
 * Doing so during the setting load phase, however, creates key files for unit
 * tests, because they usually complete the full settings load phase, but do not
 * initialize modules which they do not need. Depending on the test case, it
 * might not provide a writable AppDataPath, or it would leave private key files
 * in the source tree. All of this can be avoided with
 * specifically adjusting the ini loaded for unit tests, but adds considerable
 * friction.
 *
 * So to avoid all this, we split key loading in two phases:
 * - settings parse the config and save missing symmetric keys
 * - module init takes the parsed config, creates missing asymmetric keys and
 *   creates the actual signingkey objects
 *
 * jwtx.SigningKeyCfg and jwtx.KeyCfg are used for handover
 */

// loadSecret load the secret from ini by uriKey or verbatimKey, only one of them could be set
// If the secret is loaded from uriKey (file), the file should be non-empty, to guarantee the behavior stable and clear.
func loadSecret(sec ConfigSection, uriKey, verbatimKey string) string {
	// don't allow setting both URI and verbatim string
	uri := sec.Key(uriKey).String()
	verbatim := sec.Key(verbatimKey).String()
	if uri != "" && verbatim != "" {
		log.Fatal("Cannot specify both %s and %s", uriKey, verbatimKey)
	}

	// if we have no URI, use verbatim
	if uri == "" {
		return verbatim
	}
	verbatim, err := loadSecretFromURI(uri)
	if err == nil {
		return verbatim
	}
	log.Fatal("%s: %w", uriKey, err)
	// unreached
	return ""
}

func loadSecretFromURI(uri string) (string, error) {
	tempURI, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("Failed to parse %s: %v", uri, err)
	}
	switch tempURI.Scheme {
	case "file":
		path := tempURI.RequestURI()
		if !filepath.IsAbs(path) {
			path = filepath.Join(AppDataPath, path)
		}
		buf, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("Failed to read %s: %v", path, err)
		}
		val := strings.TrimSpace(string(buf))
		if val == "" {
			// The file shouldn't be empty, otherwise we can not know whether the user has ever set the KEY or KEY_URI
			// For example: if INTERNAL_TOKEN_URI=file:///empty-file,
			// Then if the token is re-generated during installation and saved to INTERNAL_TOKEN
			// Then INTERNAL_TOKEN and INTERNAL_TOKEN_URI both exist, that's a fatal error (they shouldn't)
			return "", fmt.Errorf("Failed to read %s: the file is empty", path)
		}
		return val, nil

	// only file URIs are allowed
	default:
		return "", fmt.Errorf("Unsupported URI-Scheme %q in %q", tempURI.Scheme, uri)
	}
}

// createSymmeticSigningKey creates a new symmetric signing key and saves it to
// the setting named cfgSecret (usually [PFX_]SECRET) in section cfgSection
func createSymmeticSigningKeyCfg(rootCfg ConfigProvider, cfgSection, cfgSecret string) (*[]byte, error) {
	jwtSecretBytes, jwtSecretBase64 := generate.NewJwtSecret()
	saveCfg, err := rootCfg.PrepareSaving()
	if err == nil {
		rootCfg.Section(cfgSection).Key(cfgSecret).SetValue(jwtSecretBase64)
		saveCfg.Section(cfgSection).Key(cfgSecret).SetValue(jwtSecretBase64)
		err = saveCfg.Save()
	}
	if err != nil {
		return nil, fmt.Errorf("save %s.%s failed: %v", cfgSection, cfgSecret, err)
	}
	return &jwtSecretBytes, nil
}

// loadSymmeticSigningKey loads a signing key and creates it unless present
// loads from [pfx]SECRET_URI
// loads from or saves to [pfx]SECRET
// in section sec
func loadSymmeticSigningKeyCfg(rootCfg ConfigProvider, sec ConfigSection, pfx string) (*[]byte, error) {
	cfgSecretURI := pfx + "SECRET_URI"
	cfgSecret := pfx + "SECRET"

	secretBase64 := loadSecret(sec, cfgSecretURI, cfgSecret)
	secret, err := generate.DecodeJwtSecret(secretBase64)
	if err == nil {
		return &secret, nil
	}

	log.Info("[%s] %s or %s failed loading: %v - creating new key", sec.Name(), cfgSecret, cfgSecretURI, err)
	return createSymmeticSigningKeyCfg(rootCfg, sec.Name(), cfgSecret)
}

// loadAsymmeticSigningKey loads a signing key from [pfx]SIGNING_PRIVATE_KEY_FILE
// or creates it if it does not exist
func loadAsymmeticSigningKeyPath(sec ConfigSection, pfx, defaultFile string) *string {
	cfgFile := pfx + "SIGNING_PRIVATE_KEY_FILE"
	keyPath := sec.Key(cfgFile).MustString(defaultFile)
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(AppDataPath, keyPath)
	}
	return &keyPath
}

type (
	checkFunc   func(rootCfg ConfigProvider, cfgSection, pfx string) error
	checkKeyCfg struct {
		signing      checkFunc
		verification checkFunc
	}
)

func checkSigningOnlyAsymmetric(rootCfg ConfigProvider, cfgSection, pfx string) error {
	sec := rootCfg.Section(cfgSection)
	cfgAlg := pfx + "SIGNING_ALGORITHM"

	if sec.HasKey(cfgAlg) {
		alg := sec.Key(cfgAlg).String()
		if !jwtx.IsValidAsymmetricAlgorithm(alg) {
			return fmt.Errorf("Unexpected algorithm: %s = %s, needs to be one of %v",
				cfgAlg, alg, jwtx.ValidAsymmetricAlgorithms)
		}
	}

	noCfg := []string{
		pfx + "SECRET_URI",
		pfx + "SECRET",
	}

	for _, cfg := range noCfg {
		if sec.HasKey(cfg) {
			return fmt.Errorf("Invalid config key: %s - must be removed", cfg)
		}
	}

	return nil
}

func checkValidationOnlyAsymmetric(rootCfg ConfigProvider, cfgSection, pfx string) error {
	sec := rootCfg.Section(cfgSection)
	cfg := pfx + "KEYS_ACCEPTED"

	if !sec.HasKey(cfg) {
		return nil
	}

	for _, spec := range sec.Key(cfg).Strings(" ") {
		algo, _, found := strings.Cut(spec, ":")
		if !found {
			// error to be handled by loadVerificationKeyCfg
			continue
		}

		if !jwtx.IsValidAsymmetricAlgorithm(algo) {
			return fmt.Errorf("Unexpected algorithm: %s = %s, needs to be one of %v",
				cfg, algo, jwtx.ValidAsymmetricAlgorithms)
		}
	}
	return nil
}

func onlyAsymmetric() checkKeyCfg {
	return checkKeyCfg{
		signing:      checkSigningOnlyAsymmetric,
		verification: checkValidationOnlyAsymmetric,
	}
}

// loadSigningKey() loads a or creates signing key based on settings in section cfgSection
// [pfx]SIGNING_ALGORITHM determines the algorithm
// [pfx]SECRET is a literal secret for symmetric algorithms
// [pfx]SECRET_URI is the uri of a secret for symmetric algorithms
// [pfx]SIGNING_PRIVATE_KEY_FILE is a file with a private key for asymmetric algorithms
//
// [pfx]SECRET might get written to literally in the config if needed but not present

func loadSigningKeyCfg(rootCfg ConfigProvider, cfgSection, pfx, defaultAlg, defaultPrivateKeyFile string, checks ...checkKeyCfg) (*jwtx.SigningKeyCfg, error) {
	for _, check := range checks {
		if check.signing == nil {
			continue
		}
		err := check.signing(rootCfg, cfgSection, pfx)
		if err != nil {
			return nil, err
		}
	}

	sec := rootCfg.Section(cfgSection)
	cfgAlg := pfx + "SIGNING_ALGORITHM"

	algorithm := sec.Key(cfgAlg).MustString(defaultAlg)

	cfg := jwtx.SigningKeyCfg{Algorithm: algorithm}
	var err error

	if jwtx.IsValidSymmetricAlgorithm(algorithm) {
		cfg.SecretBytes, err = loadSymmeticSigningKeyCfg(rootCfg, sec, pfx)
	} else if jwtx.IsValidAsymmetricAlgorithm(algorithm) {
		cfg.PrivateKeyPath = loadAsymmeticSigningKeyPath(sec, pfx, defaultPrivateKeyFile)
	} else {
		err = fmt.Errorf("invalid algorithm: %s = %s", cfgAlg, algorithm)
	}

	return &cfg, err
}

// expand file URLs with wildcards (see filepath.Glob())
//
// NB: via url.Parse() / .RequestURI() we already support a non-standard file:<name>
// URL scheme (no authority), this is another extension
func expandFileURItoPaths(uri string) ([]string, error) {
	var parsedURI *url.URL

	parsedURI, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if parsedURI.Scheme != "file" {
		return nil, fmt.Errorf("Unsupported URI-Scheme %q in %q", parsedURI.Scheme, uri)
	}

	file := parsedURI.RequestURI()
	if !filepath.IsAbs(file) {
		file = filepath.Join(AppDataPath, file)
	}

	files, err := filepath.Glob(file)
	// for no matches/malformed patterh, return the input, ENOFILE will be hit later
	if files == nil || err != nil {
		return []string{file}, nil
	}
	return files, nil
}

// for passing expandFileURItoPaths to loadSecretFromURI, which we use to retain
// compatibility
func prependFile(in []string) (out []string) {
	for _, f := range in {
		out = append(out, "file:"+f)
	}
	return out
}

// loadVerificationKeyCfg() loads additional keys to accept for verification
//
// [pfx]KEYS_ACCEPTED = <alg>:[<base64>|<uri>] <alg>:[<base64>|<uri>] ...
func loadVerificationKeyCfg(rootCfg ConfigProvider, cfgSection, pfx string, checks ...checkKeyCfg) ([]*jwtx.VerificationKeyCfg, error) {
	for _, check := range checks {
		if check.verification == nil {
			continue
		}
		err := check.verification(rootCfg, cfgSection, pfx)
		if err != nil {
			return nil, err
		}
	}

	sec := rootCfg.Section(cfgSection)
	cfg := pfx + "KEYS_ACCEPTED"

	if !sec.HasKey(cfg) {
		return nil, nil
	}
	var vcfgs []*jwtx.VerificationKeyCfg
	for _, spec := range sec.Key(cfg).Strings(" ") {
		algo, uri, found := strings.Cut(spec, ":")
		if !found {
			return nil, fmt.Errorf("Seperator : not found in %s spec %s", cfg, spec)
		}

		err := func() error {
			if jwtx.IsValidSymmetricAlgorithm(algo) {
				var b64s []string

				if strings.ContainsRune(uri, ':') {
					paths, err := expandFileURItoPaths(uri)
					// ignore error here, let loadSecretFromURI generate it
					if err == nil {
						paths = prependFile(paths)
					} else {
						paths = []string{uri}
					}
					for _, path := range paths {
						b64, err := loadSecretFromURI(path)
						if err != nil {
							return err
						}
						b64s = append(b64s, b64)
					}
				} else {
					b64s = []string{uri}
				}
				for _, b64 := range b64s {
					secret, err := generate.DecodeJwtSecret(b64)
					if err != nil {
						return err
					}
					vcfgs = append(vcfgs, &jwtx.VerificationKeyCfg{
						Algorithm:   algo,
						SecretBytes: &secret,
					})
				}
			} else if jwtx.IsValidAsymmetricAlgorithm(algo) {
				paths, err := expandFileURItoPaths(uri)
				if err != nil {
					return err
				}
				for _, path := range paths {
					vcfgs = append(vcfgs, &jwtx.VerificationKeyCfg{
						Algorithm:     algo,
						PublicKeyPath: &path,
					})
				}
			} else {
				return fmt.Errorf("Invalid algorithm %s in %s", algo, cfg)
			}
			return nil
		}()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cfg, err)
		}
	}
	return vcfgs, nil
}

func loadKeyCfg(rootCfg ConfigProvider, cfgSection, pfx, defaultAlg, defaultPrivateKeyFile string, checks ...checkKeyCfg) (*jwtx.KeyCfg, error) {
	var err error
	kCfg := &jwtx.KeyCfg{}

	kCfg.Signing, err = loadSigningKeyCfg(rootCfg, cfgSection, pfx, defaultAlg, defaultPrivateKeyFile, checks...)

	if err == nil {
		kCfg.Verification, err = loadVerificationKeyCfg(rootCfg, cfgSection, pfx, checks...)
	}

	if err == nil {
		return kCfg, nil
	}
	err = fmt.Errorf("[%s] %v", cfgSection, err)
	return nil, err
}

// generateSaveInternalToken generates and saves the internal token to app.ini
func generateSaveInternalToken(rootCfg ConfigProvider) {
	token, err := generate.NewInternalToken()
	if err != nil {
		log.Fatal("Error generate internal token: %v", err)
	}

	InternalToken = token
	saveCfg, err := rootCfg.PrepareSaving()
	if err != nil {
		log.Fatal("Error saving internal token: %v", err)
	}
	rootCfg.Section("security").Key("INTERNAL_TOKEN").SetValue(token)
	saveCfg.Section("security").Key("INTERNAL_TOKEN").SetValue(token)
	if err = saveCfg.Save(); err != nil {
		log.Fatal("Error saving internal token: %v", err)
	}
}

func loadSecurityFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("security")
	InstallLock = HasInstallLock(rootCfg)
	LogInRememberDays = sec.Key("LOGIN_REMEMBER_DAYS").MustInt(31)
	SecretKey = loadSecret(sec, "SECRET_KEY_URI", "SECRET_KEY")
	if SecretKey == "" {
		// FIXME: https://github.com/go-gitea/gitea/issues/16832
		// Until it supports rotating an existing secret key, we shouldn't move users off of the widely used default value
		SecretKey = "!#@FDEWREWR&*(" //nolint:gosec
	}
	keying.Init([]byte(SecretKey))

	GlobalTwoFactorRequirement = NewTwoFactorRequirementType(sec.Key("GLOBAL_TWO_FACTOR_REQUIREMENT").String())

	CookieRememberName = sec.Key("COOKIE_REMEMBER_NAME").MustString("persistent")

	ReverseProxyAuthUser = sec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")
	ReverseProxyAuthEmail = sec.Key("REVERSE_PROXY_AUTHENTICATION_EMAIL").MustString("X-WEBAUTH-EMAIL")
	ReverseProxyAuthFullName = sec.Key("REVERSE_PROXY_AUTHENTICATION_FULL_NAME").MustString("X-WEBAUTH-FULLNAME")

	ReverseProxyLimit = sec.Key("REVERSE_PROXY_LIMIT").MustInt(1)
	ReverseProxyTrustedProxies = sec.Key("REVERSE_PROXY_TRUSTED_PROXIES").Strings(",")
	if len(ReverseProxyTrustedProxies) == 0 {
		ReverseProxyTrustedProxies = []string{"127.0.0.0/8", "::1/128"}
	}

	MinPasswordLength = sec.Key("MIN_PASSWORD_LENGTH").MustInt(8)
	ImportLocalPaths = sec.Key("IMPORT_LOCAL_PATHS").MustBool(false)
	DisableGitHooks = sec.Key("DISABLE_GIT_HOOKS").MustBool(true)
	DisableWebhooks = sec.Key("DISABLE_WEBHOOKS").MustBool(false)
	OnlyAllowPushIfGiteaEnvironmentSet = sec.Key("ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET").MustBool(true)

	// Ensure that the provided default hash algorithm is a valid hash algorithm
	var algorithm *hash.PasswordHashAlgorithm
	PasswordHashAlgo, algorithm = hash.SetDefaultPasswordHashAlgorithm(sec.Key("PASSWORD_HASH_ALGO").MustString(""))
	if algorithm == nil {
		log.Fatal("The provided password hash algorithm was invalid: %s", sec.Key("PASSWORD_HASH_ALGO").MustString(""))
	}

	PasswordCheckPwn = sec.Key("PASSWORD_CHECK_PWN").MustBool(false)
	SuccessfulTokensCacheSize = sec.Key("SUCCESSFUL_TOKENS_CACHE_SIZE").MustInt(20)

	InternalToken = loadSecret(sec, "INTERNAL_TOKEN_URI", "INTERNAL_TOKEN")
	if InstallLock && InternalToken == "" {
		// if Gitea has been installed but the InternalToken hasn't been generated (upgrade from an old release), we should generate
		// some users do cluster deployment, they still depend on this auto-generating behavior.
		generateSaveInternalToken(rootCfg)
	}

	cfgdata := sec.Key("PASSWORD_COMPLEXITY").Strings(",")
	if len(cfgdata) == 0 {
		cfgdata = []string{"off"}
	}
	PasswordComplexity = make([]string, 0, len(cfgdata))
	for _, name := range cfgdata {
		name := strings.ToLower(strings.Trim(name, `"`))
		if name != "" {
			PasswordComplexity = append(PasswordComplexity, name)
		}
	}

	sectionHasDisableQueryAuthToken := sec.HasKey("DISABLE_QUERY_AUTH_TOKEN")

	// TODO: default value should be true in future releases
	DisableQueryAuthToken = sec.Key("DISABLE_QUERY_AUTH_TOKEN").MustBool(false)

	// warn if the setting is set to false explicitly
	if sectionHasDisableQueryAuthToken && !DisableQueryAuthToken {
		log.Warn("Enabling Query API Auth tokens is not recommended. DISABLE_QUERY_AUTH_TOKEN will be removed in Forgejo v13.0.0.")
	}
}

type TwoFactorRequirementType string

// llu:TrKeysSuffix admin.config.global_2fa_requirement.
const (
	NoneTwoFactorRequirement  TwoFactorRequirementType = "none"
	AllTwoFactorRequirement   TwoFactorRequirementType = "all"
	AdminTwoFactorRequirement TwoFactorRequirementType = "admin"
)

func NewTwoFactorRequirementType(twoFactorRequirement string) TwoFactorRequirementType {
	switch twoFactorRequirement {
	case AllTwoFactorRequirement.String():
		return AllTwoFactorRequirement
	case AdminTwoFactorRequirement.String():
		return AdminTwoFactorRequirement
	default:
		return NoneTwoFactorRequirement
	}
}

func (r TwoFactorRequirementType) String() string {
	return string(r)
}

func (r TwoFactorRequirementType) IsNone() bool {
	return r == NoneTwoFactorRequirement
}

func (r TwoFactorRequirementType) IsAll() bool {
	return r == AllTwoFactorRequirement
}

func (r TwoFactorRequirementType) IsAdmin() bool {
	return r == AdminTwoFactorRequirement
}
