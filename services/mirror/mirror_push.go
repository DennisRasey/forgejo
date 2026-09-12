// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/git"
	giturl "forgejo.org/modules/git/url"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/lfs"
	"forgejo.org/modules/log"
	"forgejo.org/modules/process"
	"forgejo.org/modules/repository"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"
	migrations_allowlist "forgejo.org/services/migrations/allowlist"
)

var stripExitStatus = regexp.MustCompile(`exit status \d+ - `)

// AddPushMirrorRemote registers the push mirror remote.
var AddPushMirrorRemote = addPushMirrorRemote

func addPushMirrorRemote(ctx context.Context, m *repo_model.PushMirror, addr string) error {
	addRemoteAndConfig := func(addr, path string) error {
		var cmd *git.Command
		if m.BranchFilter == "" {
			cmd = git.NewCommand(ctx, "remote", "add", "--mirror=push").AddDynamicArguments(m.RemoteName, addr)
		} else {
			cmd = git.NewCommand(ctx, "remote", "add").AddDynamicArguments(m.RemoteName, addr)
		}
		if strings.Contains(addr, "://") && strings.Contains(addr, "@") {
			cmd.SetDescription(fmt.Sprintf("remote add %s %s [repo_path: %s]", m.RemoteName, util.SanitizeCredentialURLs(addr), path))
		} else {
			cmd.SetDescription(fmt.Sprintf("remote add %s %s [repo_path: %s]", m.RemoteName, addr, path))
		}
		if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: path}); err != nil {
			return err
		}
		err := addRemotePushRefSpecs(ctx, path, m)
		if err != nil {
			return err
		}
		return nil
	}

	if err := addRemoteAndConfig(addr, m.Repo.RepoPath()); err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		wikiRemoteURL := repository.WikiRemoteURL(ctx, addr)
		if len(wikiRemoteURL) > 0 {
			if err := addRemoteAndConfig(wikiRemoteURL, m.Repo.WikiPath()); err != nil {
				return err
			}
		}
	}

	return nil
}

func addRemotePushRefSpecs(ctx context.Context, path string, m *repo_model.PushMirror) error {
	if m.BranchFilter == "" {
		// If there is no branch filter, set the push refspecs to mirror all branches and tags.
		if _, _, err := git.NewCommand(ctx, "config", "--add").AddDynamicArguments("remote."+m.RemoteName+".push", "+refs/heads/*:refs/heads/*").RunStdString(&git.RunOpts{Dir: path}); err != nil {
			return err
		}
	} else {
		branches := strings.SplitSeq(m.BranchFilter, ",")
		for branch := range branches {
			branch = strings.TrimSpace(branch)
			if branch == "" {
				continue
			}
			refspec := fmt.Sprintf("+refs/heads/%s:refs/heads/%s", branch, branch)
			if _, _, err := git.NewCommand(ctx, "config", "--add").AddDynamicArguments("remote."+m.RemoteName+".push", refspec).RunStdString(&git.RunOpts{Dir: path}); err != nil {
				return err
			}
		}
	}
	if _, _, err := git.NewCommand(ctx, "config", "--add").AddDynamicArguments("remote."+m.RemoteName+".push", "+refs/tags/*:refs/tags/*").RunStdString(&git.RunOpts{Dir: path}); err != nil {
		return err
	}
	return nil
}

func UpdatePushMirrorBranchFilter(ctx context.Context, m *repo_model.PushMirror) error {
	path := m.Repo.RepoPath()

	// First, remove all existing push refspecs for this remote
	cmd := git.NewCommand(ctx, "config", "--unset-all").AddDynamicArguments("remote." + m.RemoteName + ".push")
	if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: path}); err != nil {
		// Ignore error if the key doesn't exist
		if !strings.Contains(err.Error(), "does not exist") {
			return err
		}
	}
	err := addRemotePushRefSpecs(ctx, path, m)
	if err != nil {
		return err
	}
	return nil
}

// RemovePushMirrorRemote removes the push mirror remote.
func RemovePushMirrorRemote(ctx context.Context, m *repo_model.PushMirror) error {
	cmd := git.NewCommand(ctx, "remote", "rm").AddDynamicArguments(m.RemoteName)
	_ = m.GetRepository(ctx)

	if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: m.Repo.RepoPath()}); err != nil && !git.IsErrorExitCode(err, 2) {
		return err
	}

	if m.Repo.HasWiki() {
		if _, _, err := cmd.RunStdString(&git.RunOpts{Dir: m.Repo.WikiPath()}); err != nil {
			// The wiki remote may not exist
			log.Warn("Wiki Remote[%d] could not be removed: %v", m.ID, err)
		}
	}

	return nil
}

// SyncPushMirror starts the sync of the push mirror and schedules the next run.
func SyncPushMirror(ctx context.Context, mirrorID int64) bool {
	log.Trace("SyncPushMirror [mirror: %d]", mirrorID)
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst syncPushMirror...
		log.Error("PANIC whilst syncPushMirror[%d] Panic: %v\nStacktrace: %s", mirrorID, err, log.Stack(2))
	}()

	// TODO: Handle "!exist" better
	m, exist, err := db.GetByID[repo_model.PushMirror](ctx, mirrorID)
	if err != nil || !exist {
		log.Error("GetPushMirrorByID [%d]: %v", mirrorID, err)
		return false
	}

	_ = m.GetRepository(ctx)

	m.LastError = ""

	ctx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("Syncing PushMirror %s/%s to %s", m.Repo.OwnerName, m.Repo.Name, m.RemoteName))
	defer finished()

	log.Trace("SyncPushMirror [mirror: %d][repo: %-v]: Running Sync", m.ID, m.Repo)
	err = runPushSync(ctx, m)
	if err != nil {
		log.Error("SyncPushMirror [mirror: %d][repo: %-v]: %v", m.ID, m.Repo, err)
		m.LastError = stripExitStatus.ReplaceAllLiteralString(err.Error(), "")
	}

	m.LastUpdateUnix = timeutil.TimeStampNow()

	if err := repo_model.UpdatePushMirror(ctx, m); err != nil {
		log.Error("UpdatePushMirror [%d]: %v", m.ID, err)

		return false
	}

	log.Trace("SyncPushMirror [mirror: %d][repo: %-v]: Finished", m.ID, m.Repo)

	return err == nil
}

func recheckPushPermitted(ctx context.Context, m *repo_model.PushMirror, remoteURL *giturl.GitURL) error {
	if err := m.Repo.LoadOwner(ctx); err != nil {
		return err
	}
	return migrations_allowlist.IsPushMirrorURLAllowed(remoteURL.String(), m.Repo.Owner)
}

func runPushSync(ctx context.Context, m *repo_model.PushMirror) error {
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	performPush := func(repo *repo_model.Repository, isWiki bool) error {
		path := repo.RepoPath()
		if isWiki {
			path = repo.WikiPath()
		}
		// We used to erronously configure repos with `--mirror`, which would set both push and fetch refspecs into the
		// git config.  We need to remove any `fetch` configurations that were leftover from that error, as they caused
		// source repository references to rollback: https://codeberg.org/forgejo/forgejo/issues/14273 Exit code 5 is
		// ignored; that occurs when `git config --unset-all` finds nothing to remove.
		if _, _, err := git.NewCommand(ctx, "config", "--unset-all").
			AddDynamicArguments("remote." + m.RemoteName + ".fetch").
			RunStdString(&git.RunOpts{Dir: path}); err != nil && !git.IsErrorExitCode(err, 5) {
			return err
		}
		remoteURL, err := git.GetRemoteURL(ctx, path, m.RemoteName)
		if err != nil {
			log.Error("GetRemoteAddress(%s) Error %v", path, err)
			return errors.New("Unexpected error")
		}

		useSSHAuthentication := len(m.PublicKey) != 0

		// Recheck that the remote address is still permitted before pushing. The address passed IsPushMirrorURLAllowed
		// at creation time, but the allow/block lists may have changed since, or DNS for the remote host may now
		// resolve to an internal address (DNS rebinding).
		if err := recheckPushPermitted(ctx, m, remoteURL); err != nil {
			log.Error("SyncPushMirror [repo: %-v]: push mirror failed to meet migration URL requirements: %v", m.Repo, err)
			return util.SanitizeErrorCredentialURLs(err)
		}

		if setting.LFS.StartServer && !useSSHAuthentication {
			log.Trace("SyncMirrors [repo: %-v]: syncing LFS objects...", m.Repo)

			var gitRepo *git.Repository
			if isWiki {
				gitRepo, err = gitrepo.OpenWikiRepository(ctx, repo)
			} else {
				gitRepo, err = gitrepo.OpenRepository(ctx, repo)
			}
			if err != nil {
				log.Error("OpenRepository: %v", err)
				return errors.New("Unexpected error")
			}
			defer gitRepo.Close()

			endpoint := lfs.DetermineEndpoint(remoteURL.String(), "")
			lfsClient := lfs.NewClient(endpoint, migrations_allowlist.NewMigrationHTTPTransport())
			if err := pushAllLFSObjects(ctx, gitRepo, lfsClient); err != nil {
				return util.SanitizeErrorCredentialURLs(err)
			}
		}

		log.Trace("Pushing %s mirror[%d] remote %s", path, m.ID, m.RemoteName)

		// OpenSSH isn't very intuitive when you want to specify a specific keypair.
		// Therefore, we need to create a temporary file that stores the private key, so that OpenSSH can use it.
		// We delete the temporary file afterwards.
		privateKeyPath := ""
		if useSSHAuthentication {
			f, err := os.CreateTemp(os.TempDir(), m.RemoteName)
			if err != nil {
				log.Error("os.CreateTemp: %v", err)
				return errors.New("unexpected error")
			}

			defer func() {
				f.Close()
				if err := os.Remove(f.Name()); err != nil {
					log.Error("os.Remove: %v", err)
				}
			}()

			privateKey, err := m.Privatekey()
			if err != nil {
				log.Error("Privatekey: %v", err)
				return errors.New("unexpected error")
			}

			if _, err := f.Write(privateKey); err != nil {
				log.Error("f.Write: %v", err)
				return errors.New("unexpected error")
			}

			privateKeyPath = f.Name()
		}
		if err := git.Push(ctx, path, git.PushOptions{
			Remote:         m.RemoteName,
			Force:          true,
			Mirror:         true,
			Timeout:        timeout,
			PrivateKeyPath: privateKeyPath,
		}); err != nil {
			log.Error("Error pushing %s mirror[%d] remote %s: %v", path, m.ID, m.RemoteName, err)

			return util.SanitizeErrorCredentialURLs(err)
		}
		return nil
	}

	err := performPush(m.Repo, false)
	if err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		_, err := git.GetRemoteAddress(ctx, m.Repo.WikiPath(), m.RemoteName)
		if err == nil {
			err := performPush(m.Repo, true)
			if err != nil {
				return err
			}
		} else {
			log.Trace("Skipping wiki: No remote configured")
		}
	}

	return nil
}

func pushAllLFSObjects(ctx context.Context, gitRepo *git.Repository, lfsClient lfs.Client) error {
	contentStore := lfs.NewContentStore()

	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go lfs.SearchPointerBlobs(ctx, gitRepo, pointerChan, errChan)

	uploadObjects := func(pointers []lfs.Pointer) error {
		err := lfsClient.Upload(ctx, pointers, func(p lfs.Pointer, objectError error) (io.ReadCloser, error) {
			if objectError != nil {
				return nil, objectError
			}

			content, err := contentStore.Get(p)
			if err != nil {
				log.Error("Error reading LFS object %v: %v", p, err)
			}
			return content, err
		})
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}
		return err
	}

	var batch []lfs.Pointer
	for pointerBlob := range pointerChan {
		exists, err := contentStore.Exists(pointerBlob.Pointer)
		if err != nil {
			log.Error("Error checking if LFS object %v exists: %v", pointerBlob.Pointer, err)
			return err
		}
		if !exists {
			log.Trace("Skipping missing LFS object %v", pointerBlob.Pointer)
			continue
		}

		batch = append(batch, pointerBlob.Pointer)
		if len(batch) >= lfsClient.BatchSize() {
			if err := uploadObjects(batch); err != nil {
				return err
			}
			batch = nil
		}
	}
	if len(batch) > 0 {
		if err := uploadObjects(batch); err != nil {
			return err
		}
	}

	err, has := <-errChan
	if has {
		log.Error("Error enumerating LFS objects for repository: %v", err)
		return err
	}

	return nil
}

func syncPushMirrorWithSyncOnCommit(ctx context.Context, repoID int64) {
	pushMirrors, err := repo_model.GetPushMirrorsSyncedOnCommit(ctx, repoID)
	if err != nil {
		log.Error("repo_model.GetPushMirrorsSyncedOnCommit failed: %v", err)
		return
	}

	for _, mirror := range pushMirrors {
		AddPushMirrorToQueue(mirror.ID)
	}
}
