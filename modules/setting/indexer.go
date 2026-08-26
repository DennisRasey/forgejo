// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"forgejo.org/modules/log"

	"github.com/gobwas/glob"
)

// Indexer settings
var Indexer = struct {
	IssueType        string
	IssuePath        string
	IssueConnStr     string
	IssueConnAuth    string
	IssueIndexerName string
	StartupTimeout   time.Duration

	RepoIndexerEnabled     bool
	RepoIndexerRepoTypes   []string
	RepoIndexerEnableFuzzy bool
	RepoType               string
	RepoPath               string
	RepoConnStr            string
	RepoIndexerName        string
	MaxIndexerFileSize     int64
	IncludePatterns        []Glob
	ExcludePatterns        []Glob
	ExcludeVendored        bool
}{
	IssueType:        "bleve",
	IssuePath:        "indexers/issues.bleve",
	IssueConnStr:     "",
	IssueConnAuth:    "",
	IssueIndexerName: "gitea_issues",

	RepoIndexerEnabled:     false,
	RepoIndexerRepoTypes:   []string{"sources", "forks", "mirrors", "templates"},
	RepoIndexerEnableFuzzy: false,
	RepoType:               "bleve",
	RepoPath:               "indexers/repos.bleve",
	RepoConnStr:            "",
	RepoIndexerName:        "gitea_codes",
	MaxIndexerFileSize:     1024 * 1024,
	ExcludeVendored:        true,
}

type Glob struct {
	glob    glob.Glob
	pattern string
}

func (g *Glob) Match(s string) bool {
	return g.glob.Match(s)
}

func (g *Glob) Pattern() string {
	return g.pattern
}

func requiresConnectionString(indexerType string) bool {
	return (indexerType == "elasticsearch") || (indexerType == "meilisearch")
}

func buildConnectionString(sec ConfigSection) string {
	connURL := &url.URL{}
	connURL.Scheme = sec.Key("ISSUE_INDEXER_PROTOCOL").String()
	connURL.Host = sec.Key("ISSUE_INDEXER_HOST").String()
	connURL.Path = sec.Key("ISSUE_INDEXER_PATH").String()

	username := sec.Key("ISSUE_INDEXER_USER").String()
	passwd := loadSecret(sec, "ISSUE_INDEXER_PASSWD_URI", "ISSUE_INDEXER_PASSWD")
	if passwd != "" {
		// If password is set we need to generate the user info part of the URL, the username being empty/unset is a feature (for instance with meilisearch)
		connURL.User = url.UserPassword(username, passwd)
	} else if username != "" {
		// If password is not set but the username is we'll include the username in the URL
		connURL.User = url.User(username)
	} else {
		// Neither auth configs were set so don't generate that part of the connection string
		connURL.User = nil
	}

	return connURL.String()
}

func getIndexerConnStr(sec ConfigSection) (string, error) {
	hasConnString := sec.HasKey("ISSUE_INDEXER_CONN_STR")
	hasBuilderConfigs := sec.HasKey("ISSUE_INDEXER_HOST") && sec.HasKey("ISSUE_INDEXER_PROTOCOL")

	if hasConnString && hasBuilderConfigs {
		return "", errors.New("cannot define both ISSUE_INDEXER_CONN_STR and builder configs")
	} else if hasConnString {
		return sec.Key("ISSUE_INDEXER_CONN_STR").MustString(""), nil
	} else if hasBuilderConfigs {
		return buildConnectionString(sec), nil
	}
	return "", fmt.Errorf("must define either ISSUE_INDEXER_CONN_STR or builder configs for %q", Indexer.IssueType)
}

// Bleve requires a default path but meilisearch and elastic search should default to an empty path, db doesn't use path so doesn't care
func getIssueIndexerPath(sec ConfigSection) string {
	switch Indexer.IssueType {
	case "bleve":
		issuePath := filepath.ToSlash(sec.Key("ISSUE_INDEXER_PATH").MustString(filepath.ToSlash(filepath.Join(AppDataPath, "indexers/issues.bleve"))))
		if !filepath.IsAbs(issuePath) {
			issuePath = filepath.ToSlash(filepath.Join(AppWorkPath, Indexer.IssuePath))
		}
		return issuePath
	case "meilisearch", "elasticsearch":
		return sec.Key("ISSUE_INDEXER_PATH").MustString("")
	default:
		return ""
	}
}

func loadIndexerFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("indexer")
	Indexer.IssueType = sec.Key("ISSUE_INDEXER_TYPE").MustString("bleve")
	Indexer.IssuePath = getIssueIndexerPath(sec)

	if requiresConnectionString(Indexer.IssueType) {
		connStr, err := getIndexerConnStr(sec)
		if err != nil {
			log.Fatal(err.Error())
		}
		Indexer.IssueConnStr = connStr
	}
	if Indexer.IssueType == "meilisearch" {
		u, err := url.Parse(Indexer.IssueConnStr)
		if err != nil {
			log.Warn("Failed to parse ISSUE_INDEXER_CONN_STR: %v", err)
			u = &url.URL{}
		}
		Indexer.IssueConnAuth, _ = u.User.Password()
		u.User = nil
		Indexer.IssueConnStr = u.String()
	}

	Indexer.IssueIndexerName = sec.Key("ISSUE_INDEXER_NAME").MustString(Indexer.IssueIndexerName)

	Indexer.RepoIndexerEnabled = sec.Key("REPO_INDEXER_ENABLED").MustBool(false)
	Indexer.RepoIndexerRepoTypes = strings.Split(sec.Key("REPO_INDEXER_REPO_TYPES").MustString("sources,forks,mirrors,templates"), ",")
	Indexer.RepoIndexerEnableFuzzy = sec.Key("REPO_INDEXER_FUZZY_ENABLED").MustBool(false)
	Indexer.RepoType = sec.Key("REPO_INDEXER_TYPE").MustString("bleve")
	Indexer.RepoPath = filepath.ToSlash(sec.Key("REPO_INDEXER_PATH").MustString(filepath.ToSlash(filepath.Join(AppDataPath, "indexers", "repos."+Indexer.RepoType))))
	if !filepath.IsAbs(Indexer.RepoPath) {
		Indexer.RepoPath = filepath.ToSlash(filepath.Join(AppWorkPath, Indexer.RepoPath))
	}
	Indexer.RepoConnStr = sec.Key("REPO_INDEXER_CONN_STR").MustString("")
	Indexer.RepoIndexerName = sec.Key("REPO_INDEXER_NAME").MustString("gitea_codes")

	Indexer.IncludePatterns = IndexerGlobFromString(sec.Key("REPO_INDEXER_INCLUDE").MustString(""))
	Indexer.ExcludePatterns = IndexerGlobFromString(sec.Key("REPO_INDEXER_EXCLUDE").MustString(""))
	Indexer.ExcludeVendored = sec.Key("REPO_INDEXER_EXCLUDE_VENDORED").MustBool(true)
	Indexer.MaxIndexerFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(1024 * 1024)
	var err error
	Indexer.StartupTimeout, err = sec.Key("STARTUP_TIMEOUT").MustDuration(30 * time.Second)
	if err != nil {
		log.Fatal("Failed to parse duration for [indexer].STARTUP_TIMEOUT: %v", err)
	}
}

// IndexerGlobFromString parses a comma separated list of patterns and returns a glob.Glob slice suited for repo indexing
func IndexerGlobFromString(globstr string) []Glob {
	extarr := make([]Glob, 0, 10)
	for expr := range strings.SplitSeq(strings.ToLower(globstr), ",") {
		expr = strings.TrimSpace(expr)
		if expr != "" {
			if g, err := glob.Compile(expr, '.', '/'); err != nil {
				log.Warn("Invalid glob expression '%s' (skipped): %v", expr, err)
			} else {
				extarr = append(extarr, Glob{glob: g, pattern: expr})
			}
		}
	}
	return extarr
}
