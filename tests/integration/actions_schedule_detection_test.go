// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/migration"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	webhook_module "forgejo.org/modules/webhook"
	"forgejo.org/services/forms"
	mirror_service "forgejo.org/services/mirror"
	repo_service "forgejo.org/services/repository"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsScheduleDetection_WorkflowDirectories(t *testing.T) {
	type expectedSpec struct {
		cron     string
		timeZone optional.Option[string]
	}

	testCases := []struct {
		name                  string
		workflowID            string
		workflowDirectory     string
		workflowContent       string
		expectedWorkflowTitle string
		expectedCronSpecs     []expectedSpec
	}{
		{
			name:              "GitHub",
			workflowID:        "scheduled.yml",
			workflowDirectory: ".github/workflows",
			workflowContent: `
on:
  schedule:
    - cron: "30 5,17 * * *"
jobs:
  test:
    steps:
      - run: echo OK
`,
			expectedWorkflowTitle: ".github/workflows/scheduled.yml",
			expectedCronSpecs:     []expectedSpec{{cron: "30 5,17 * * *", timeZone: optional.None[string]()}},
		},
		{
			name:              "Gitea",
			workflowID:        "test.yml",
			workflowDirectory: ".gitea/workflows",
			workflowContent: `
name: My scheduled workflow
on:
  schedule:
    - cron: "* * * * *"
jobs:
  test:
    steps:
      - run: echo OK
`,
			expectedWorkflowTitle: "My scheduled workflow",
			expectedCronSpecs:     []expectedSpec{{cron: "* * * * *", timeZone: optional.None[string]()}},
		},
		{
			name:              "Forgejo with time zone",
			workflowID:        "tz.yml",
			workflowDirectory: ".forgejo/workflows",
			workflowContent: `
on:
  schedule:
    - cron: "44 10 * * *"
    - cron: "25 19 * * *"
      timezone: Europe/Madrid
jobs:
  test:
    steps:
      - run: echo OK
`,
			expectedWorkflowTitle: ".forgejo/workflows/tz.yml",
			expectedCronSpecs: []expectedSpec{
				{cron: "44 10 * * *", timeZone: optional.None[string]()},
				{cron: "25 19 * * *", timeZone: optional.Some("Europe/Madrid")},
			},
		},
	}
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

				// create the repo
				var sha string
				repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
					Files: forgery.MapFS{
						fmt.Sprintf("%s/%s", testCase.workflowDirectory, testCase.workflowID): forgery.MapFile(testCase.workflowContent),
					},
					LatestSha: &sha,
				})

				schedules, err := db.Find[actions_model.ActionSchedule](t.Context(), actions_model.FindScheduleOptions{RepoID: repo.ID})
				require.NoError(t, err)
				require.Len(t, schedules, 1)

				assert.Equal(t, testCase.expectedWorkflowTitle, schedules[0].Title)
				assert.Equal(t, repo.ID, schedules[0].RepoID)
				assert.Equal(t, repo.OwnerID, schedules[0].OwnerID)
				assert.Equal(t, testCase.workflowID, schedules[0].WorkflowID)
				assert.Equal(t, testCase.workflowDirectory, schedules[0].WorkflowDirectory)
				assert.Equal(t, "refs/heads/main", schedules[0].Ref)
				assert.Equal(t, int64(-2), schedules[0].TriggerUserID)
				assert.Equal(t, sha, schedules[0].CommitSHA)
				assert.Equal(t, webhook_module.HookEventPush, schedules[0].Event)
				assert.Equal(t, []byte(testCase.workflowContent), schedules[0].Content)

				specs, total, err := actions_model.FindSpecs(t.Context(), actions_model.FindSpecOptions{RepoID: repo.ID})
				require.NoError(t, err)

				assert.Equal(t, int64(len(testCase.expectedCronSpecs)), total)

				// The query to return cron specs orders by `id DESC`.
				slices.Reverse(testCase.expectedCronSpecs)

				for i, expected := range testCase.expectedCronSpecs {
					assert.Equal(t, schedules[0].ID, specs[i].ScheduleID)
					assert.Equal(t, expected.cron, specs[i].Spec)
					assert.Equal(t, expected.timeZone, specs[i].TimeZone)
				}
			})
		}
	})
}

func TestActionsScheduleDetection_PullRequest(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// MergeStyleRebaseUpdate and MergeStyleManuallyMerged do not actually merge anything.
		mergeStyles := []repo_model.MergeStyle{
			repo_model.MergeStyleMerge,
			repo_model.MergeStyleRebase,
			repo_model.MergeStyleRebaseMerge,
			repo_model.MergeStyleSquash,
			repo_model.MergeStyleFastForwardOnly,
		}
		prBranch := "add-schedule"
		workflowPath := ".forgejo/workflows/test.yaml"
		workflowContent := `on:
  schedule:
    - cron: '* * * * *'
jobs:
  test:
    steps:
      - run: echo OK`

		for _, mergeStyle := range mergeStyles {
			t.Run(string(mergeStyle), func(t *testing.T) {
				repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
					Files: forgery.MapFS{"README.md": forgery.MapFile("Hello world!")},
				})
				testContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)

				doAPIEditRepository(testContext, &api.EditRepoOption{
					HasPullRequests:  new(true),
					AllowManualMerge: new(true),
				})(t)

				unittest.AssertCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}, 0)

				createFileOpts := &api.CreateFileOptions{
					ContentBase64: base64.StdEncoding.EncodeToString([]byte(workflowContent)),
					FileOptions: api.FileOptions{
						Message:       "Create workflow",
						NewBranchName: prBranch,
						Author:        api.Identity{Name: user2.Name, Email: user2.Email},
						Committer:     api.Identity{Name: user2.Name, Email: user2.Email},
					},
				}
				doAPICreateFile(testContext, workflowPath, createFileOpts)(t)
				pr, err := doAPICreatePullRequest(testContext, user2.Name, repo.Name, repo.DefaultBranch, prBranch)(t)
				require.NoError(t, err)

				// Merge the PR *without* deleting the branch, so that, depending on the merge style, the commit can
				// exist in two separate branches. That is necessary to verify that Forgejo looks at the correct branch.
				mergeOpts := &forms.MergePullRequestForm{Do: string(mergeStyle), DeleteBranchAfterMerge: false}
				doAPIMergePullRequestForm(t, testContext, user2.Name, repo.Name, pr.Index, mergeOpts)

				schedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID})
				scheduleSpec := unittest.AssertExistsAndLoadBean(t,
					&actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: schedule.ID})

				assert.Equal(t, "* * * * *", scheduleSpec.Spec)
			})
		}
	})
}

func TestActionsScheduleDetection_ActionsUnit(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		workflowPath := ".forgejo/workflows/test.yaml"
		workflowContent := `on:
  schedule:
    - cron: '* * * * *'
jobs:
  test:
    steps:
      - run: echo OK`

		repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{workflowPath: forgery.MapFile(workflowContent)},
		})

		schedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID})
		scheduleSpec := unittest.AssertExistsAndLoadBean(t,
			&actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: schedule.ID})

		assert.Equal(t, "* * * * *", scheduleSpec.Spec)

		forgery.DisableRepoUnits(t, repo, unit_model.TypeActions)

		unittest.AssertCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}, 0)
		unittest.AssertCount(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID}, 0)

		forgery.EnableRepoUnits(t, repo, unit_model.TypeActions)

		schedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t,
			&actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: schedule.ID})

		assert.Equal(t, "* * * * *", scheduleSpec.Spec)
	})
}

func TestActionsScheduleDetection_Archive(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		workflowPath := ".forgejo/workflows/test.yaml"
		workflowContent := `on:
  schedule:
    - cron: '* * * * *'
jobs:
  test:
    steps:
      - run: echo OK`

		repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{workflowPath: forgery.MapFile(workflowContent)},
		})

		testContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)

		schedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID})
		scheduleSpec := unittest.AssertExistsAndLoadBean(t,
			&actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: schedule.ID})

		assert.Equal(t, "* * * * *", scheduleSpec.Spec)

		doAPIEditRepository(testContext, &api.EditRepoOption{Archived: new(true)})(t)

		unittest.AssertCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}, 0)
		unittest.AssertCount(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID}, 0)

		doAPIEditRepository(testContext, &api.EditRepoOption{Archived: new(false)})(t)

		schedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t,
			&actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: schedule.ID})

		assert.Equal(t, "* * * * *", scheduleSpec.Spec)
	})
}

func TestActionsScheduleDetection_DefaultBranchChange(t *testing.T) {
	type expectedSpec struct {
		cron     string
		timeZone optional.Option[string]
	}

	expectedMainSpec := expectedSpec{
		cron:     "30 5,17 * * *",
		timeZone: optional.None[string](),
	}

	expectedTestSpec := expectedSpec{
		cron:     "0 * * * *",
		timeZone: optional.None[string](),
	}

	testWorkflow := struct {
		name                   string
		workflowID             string
		workflowDirectory      string
		workflowContent        string
		updatedWorkflowContent string
		expectedWorkflowTitle  string
	}{
		name:              "Forgejo",
		workflowID:        "scheduled.yml",
		workflowDirectory: ".forgejo/workflows",
		workflowContent: `
on:
  schedule:
    - cron: "30 5,17 * * *"
jobs:
  test:
    steps:
      - run: echo OK
`,
		updatedWorkflowContent: `
on:
  schedule:
    - cron: "0 * * * *"
jobs:
  test:
    steps:
      - run: echo updated
`,
		expectedWorkflowTitle: ".forgejo/workflows/scheduled.yml",
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create repo
		var sha string
		repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				fmt.Sprintf("%s/%s", testWorkflow.workflowDirectory, testWorkflow.workflowID): forgery.MapFile(testWorkflow.workflowContent),
			},
			LatestSha: &sha,
		})

		gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()

		// create new branch
		err = repo_service.CreateNewBranch(t.Context(), user, repo, gitRepo, repo.DefaultBranch, "test")
		require.NoError(t, err)

		commit, err := gitRepo.GetBranchCommit("test")
		require.NoError(t, err)

		_, err = files_service.ChangeRepoFiles(
			t.Context(),
			repo,
			user,
			&files_service.ChangeRepoFilesOptions{
				LastCommitID: commit.ID.String(),
				OldBranch:    "test",
				NewBranch:    "test",
				Message:      "update workflow",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      testWorkflow.expectedWorkflowTitle,
						ContentReader: strings.NewReader(testWorkflow.updatedWorkflowContent),
					},
				},
			},
		)
		require.NoError(t, err)

		assertSchedule := func(t *testing.T, expectedRef, content string, spec expectedSpec) {
			t.Helper()
			schedules, err := db.Find[actions_model.ActionSchedule](t.Context(), actions_model.FindScheduleOptions{RepoID: repo.ID})

			require.NoError(t, err)
			require.Len(t, schedules, 1)

			assert.Equal(t, expectedRef, schedules[0].Ref)
			assert.Equal(t, testWorkflow.expectedWorkflowTitle, schedules[0].Title)
			assert.Equal(t, repo.ID, schedules[0].RepoID)
			assert.Equal(t, testWorkflow.workflowID, schedules[0].WorkflowID)
			assert.Equal(t, testWorkflow.workflowDirectory, schedules[0].WorkflowDirectory)
			assert.Equal(t, []byte(content), schedules[0].Content)

			specs, total, err := actions_model.FindSpecs(t.Context(), actions_model.FindSpecOptions{RepoID: repo.ID})

			require.NoError(t, err)
			require.Equal(t, int64(1), total)
			require.Len(t, specs, 1)

			assert.Equal(t, schedules[0].ID, specs[0].ScheduleID)
			assert.Equal(t, spec.cron, specs[0].Spec)
			assert.Equal(t, spec.timeZone, specs[0].TimeZone)
		}

		// change default branch to test
		err = repo_service.SetRepoDefaultBranch(t.Context(), repo, gitRepo, "test")
		require.NoError(t, err)

		assertSchedule(t, "refs/heads/test", testWorkflow.updatedWorkflowContent, expectedTestSpec)

		// change default branch to main
		err = repo_service.SetRepoDefaultBranch(t.Context(), repo, gitRepo, "main")
		require.NoError(t, err)

		assertSchedule(t, "refs/heads/main", testWorkflow.workflowContent, expectedMainSpec)
	})
}

func TestActionsScheduleDetection_MirrorSync(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		workflowPath := ".forgejo/workflows/test.yaml"
		workflowContent := `on:
  schedule:
    - cron: '* * * * *'
jobs:
  test:
    steps:
      - run: echo OK`

		var sha string
		repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
			Files:     forgery.MapFS{workflowPath: forgery.MapFile(workflowContent)},
			LatestSha: &sha,
		})

		defer test.MockVariableValue(&setting.ImportLocalPaths, true)()
		_, err := db.GetEngine(t.Context()).ID(user2.ID).SetExpr("allow_import_local", true).Update(user2)
		require.NoError(t, err)
		defer func() {
			_, err := db.GetEngine(t.Context()).ID(user2.ID).SetExpr("allow_import_local", false).Update(user2)
			require.NoError(t, err)
		}()

		// Mirror repository
		migrateOptions := migration.MigrateOptions{
			RepoName:  "actions-schedule-mirror",
			Private:   false,
			Mirror:    true,
			CloneAddr: repo_model.RepoPath(user2.Name, repo.Name),
		}
		mirrorRepo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:          migrateOptions.RepoName,
			IsPrivate:     migrateOptions.Private,
			IsMirror:      migrateOptions.Mirror,
			DefaultBranch: repo.DefaultBranch,
			Status:        repo_model.RepositoryBeingMigrated,
		})
		require.NoError(t, err)

		mirrorRepo, err = repo_service.MigrateRepositoryGitData(t.Context(), user2, mirrorRepo, migrateOptions, nil)
		require.NoError(t, err)

		assert.True(t, mirrorRepo.IsMirror)

		// Enable Actions on mirror repository because they are disabled on mirrors.
		forgery.EnableRepoUnit(t, mirrorRepo, unit_model.TypeActions, nil)

		schedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: mirrorRepo.ID})
		scheduleSpec := unittest.AssertExistsAndLoadBean(t,
			&actions_model.ActionScheduleSpec{RepoID: mirrorRepo.ID, ScheduleID: schedule.ID})

		assert.Equal(t, "* * * * *", scheduleSpec.Spec)

		workflowContent = `on:
  schedule:
    - cron: '@every 1m'
jobs:
  test:
    steps:
      - run: echo OK`

		// Change workflow schedule in mirrored repository, sync it, and verify that the schedule in the **mirror**
		// repository has been updated after the synchronization has completed.
		opts := files_service.ChangeRepoFilesOptions{
			LastCommitID: sha,
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "update",
					TreePath:      workflowPath,
					ContentReader: strings.NewReader(workflowContent),
				},
			},
		}
		_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &opts)
		require.NoError(t, err)

		ok := mirror_service.SyncPullMirror(t.Context(), mirrorRepo.ID)
		assert.True(t, ok)

		schedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: mirrorRepo.ID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t,
			&actions_model.ActionScheduleSpec{RepoID: mirrorRepo.ID, ScheduleID: schedule.ID})

		assert.Equal(t, "@every 1m", scheduleSpec.Spec)
	})
}
