// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/modules/container"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
)

//  _____          __  .__                 .__                  .___
// /  _  \  __ ___/  |_|  |__   ___________|__|_______ ____   __| _/
// /  /_\  \|  |  \   __\  |  \ /  _ \_  __ \  \___   // __ \ / __ |
// /    |    \  |  /|  | |   Y  (  <_> )  | \/  |/    /\  ___// /_/ |
// \____|__  /____/ |__| |___|  /\____/|__|  |__/_____ \\___  >____ |
//         \/                 \/                      \/    \/     \/
// ____  __.
// |    |/ _|____ ___.__. ______
// |      <_/ __ <   |  |/  ___/
// |    |  \  ___/\___  |\___ \
// |____|__ \___  > ____/____  >
//         \/   \/\/         \/
//
// This file contains functions for creating authorized_keys files
//
// There is a dependence on the database within RegeneratePublicKeys however most of these functions probably belong in a module

const (
	tplCommentPrefix = `# gitea public key`
	tplPublicKey     = tplCommentPrefix + "\n" + `command=%s,no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty,no-user-rc,restrict %s` + "\n"
)

var sshOpLocker sync.Mutex

// AuthorizedStringForKey creates the authorized keys string appropriate for the provided key
func AuthorizedStringForKey(key *PublicKey) string {
	sb := &strings.Builder{}
	_ = setting.SSH.AuthorizedKeysCommandTemplateTemplate.Execute(sb, map[string]any{
		"AppPath":     util.ShellEscape(setting.AppPath),
		"AppWorkPath": util.ShellEscape(setting.AppWorkPath),
		"CustomConf":  util.ShellEscape(setting.CustomConf),
		"CustomPath":  util.ShellEscape(setting.CustomPath),
		"Key":         key,
	})

	return fmt.Sprintf(tplPublicKey, util.ShellEscape(sb.String()), key.Content)
}

// appendAuthorizedKeysToFile appends new SSH keys' content to authorized_keys file.
func appendAuthorizedKeysToFile(keys ...*PublicKey) error {
	// Don't need to rewrite this file if builtin SSH server is enabled.
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return nil
	}

	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	if setting.SSH.RootPath != "" {
		// First of ensure that the RootPath is present, and if not make it with 0700 permissions
		// This of course doesn't guarantee that this is the right directory for authorized_keys
		// but at least if it's supposed to be this directory and it doesn't exist and we're the
		// right user it will at least be created properly.
		err := os.MkdirAll(setting.SSH.RootPath, 0o700)
		if err != nil {
			log.Error("Unable to MkdirAll(%s): %v", setting.SSH.RootPath, err)
			return err
		}
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	f, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	// .ssh directory should have mode 700, and authorized_keys file should have mode 600.
	if fi.Mode().Perm() > 0o600 {
		log.Error("authorized_keys file has unusual permission flags: %s - setting to -rw-------", fi.Mode().Perm().String())
		if err = f.Chmod(0o600); err != nil {
			return err
		}
	}

	for _, key := range keys {
		if key.Type == KeyTypePrincipal {
			continue
		}
		if _, err = f.WriteString(key.AuthorizedString()); err != nil {
			return err
		}
	}
	return nil
}

type InspectionFinding struct {
	Type    InspectionFindingType
	Comment string
}

type InspectionFindingType int

const (
	InspectionResultFileMissing        InspectionFindingType = iota // authorized_keys is absent, must regenerate
	InspectionResultUnexpectedKey                                   // authorized_keys contains at least one unexpected key
	InspectionResultMissingExpectedKey                              // authorized_keys does not contain a key that is in the DB
)

func InspectPublicKeys(ctx context.Context) ([]InspectionFinding, error) {
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return []InspectionFinding{}, nil
	}

	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	path := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	file, err := os.OpenFile(path, os.O_RDONLY, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return []InspectionFinding{{Type: InspectionResultFileMissing}}, nil
	} else if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create a set of all the expected output in the `authorized_keys` file.
	expectedKeys := make(container.Set[string])
	if err := db.GetEngine(ctx).Where("type != ?", KeyTypePrincipal).Iterate(new(PublicKey), func(idx int, bean any) (err error) {
		keyWithComment := bean.(*PublicKey).AuthorizedString()
		if !strings.HasPrefix(keyWithComment, tplCommentPrefix) {
			return fmt.Errorf("unexpected AuthorizedString")
		}
		key := strings.TrimSpace(keyWithComment[len(tplCommentPrefix):])
		expectedKeys.Add(key)
		return nil
	}); err != nil {
		return nil, err
	}

	findings := []InspectionFinding{}

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++

		line := scanner.Text()
		if strings.HasPrefix(line, tplCommentPrefix) {
			// After the public key comment, we expect a public key
			lineNumber++
			if !scanner.Scan() {
				return nil, fmt.Errorf("authorized_keys file %s had a prefix comment but no key on line %d", path, lineNumber)
			}

			key := strings.TrimSpace(scanner.Text())
			if !expectedKeys.Remove(key) {
				findings = append(findings, InspectionFinding{
					Type:    InspectionResultUnexpectedKey,
					Comment: fmt.Sprintf("Key on line %d of %s does not exist in database", lineNumber, path),
				})
			}
		} else {
			findings = append(findings, InspectionFinding{
				Type:    InspectionResultUnexpectedKey,
				Comment: fmt.Sprintf("Unexpected key on line %d of %s", lineNumber, path),
			})
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	for key := range expectedKeys {
		findings = append(findings, InspectionFinding{
			Type:    InspectionResultMissingExpectedKey,
			Comment: fmt.Sprintf("Key in database is not present in %s: %s", path, key),
		})
	}

	return findings, nil
}

// RewriteAllPublicKeys removes any authorized key and rewrite all keys from database again.
// Note: db.GetEngine(ctx).Iterate does not get latest data after insert/delete, so we have to call this function
// outside any session scope independently.
func RewriteAllPublicKeys(ctx context.Context) error {
	// Don't rewrite key if internal server
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return nil
	}

	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	if setting.SSH.RootPath != "" {
		// First of ensure that the RootPath is present, and if not make it with 0700 permissions
		// This of course doesn't guarantee that this is the right directory for authorized_keys
		// but at least if it's supposed to be this directory and it doesn't exist and we're the
		// right user it will at least be created properly.
		err := os.MkdirAll(setting.SSH.RootPath, 0o700)
		if err != nil {
			log.Error("Unable to MkdirAll(%s): %v", setting.SSH.RootPath, err)
			return err
		}
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	tmpPath := fPath + ".tmp"
	t, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		t.Close()
		if err := util.Remove(tmpPath); err != nil {
			log.Warn("Unable to remove temporary authorized keys file: %s: Error: %v", tmpPath, err)
		}
	}()

	if setting.SSH.AuthorizedKeysBackup {
		isExist, err := util.IsExist(fPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", fPath, err)
			return err
		}
		if isExist {
			bakPath := fmt.Sprintf("%s_%d.gitea_bak", fPath, time.Now().Unix())
			if err = util.CopyFile(fPath, bakPath); err != nil {
				return err
			}
		}
	}

	if err := regeneratePublicKeys(ctx, t); err != nil {
		return err
	}

	if err := t.Sync(); err != nil {
		return err
	}
	if err := t.Close(); err != nil {
		return err
	}
	return util.Rename(tmpPath, fPath)
}

// regeneratePublicKeys regenerates the authorized_keys file
func regeneratePublicKeys(ctx context.Context, t io.StringWriter) error {
	if err := db.GetEngine(ctx).Where("type != ?", KeyTypePrincipal).Iterate(new(PublicKey), func(idx int, bean any) (err error) {
		_, err = t.WriteString(bean.(*PublicKey).AuthorizedString())
		return err
	}); err != nil {
		return err
	}

	// Preserve any authorized_keys contents that are not generated from Forgejo
	if setting.SSH.AllowUnexpectedAuthorizedKeys {
		fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
		isExist, err := util.IsExist(fPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", fPath, err)
			return err
		}
		if isExist {
			f, err := os.Open(fPath)
			if err != nil {
				return err
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, tplCommentPrefix) {
					scanner.Scan()
					continue
				}
				_, err = t.WriteString(line + "\n")
				if err != nil {
					return err
				}
			}
			if err = scanner.Err(); err != nil {
				return fmt.Errorf("regeneratePublicKeys scan: %w", err)
			}
		}
	}

	return nil
}
