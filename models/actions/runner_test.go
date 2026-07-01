// SPDX-License-Identifier: MIT

package actions

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSecret checks that ActionRunner.UpdateSecret() sets the Token,
// TokenSalt and TokenHash fields based on the specified token.
func TestUpdateSecret(t *testing.T) {
	runner := ActionRunner{}
	token := "0123456789012345678901234567890123456789"

	err := runner.UpdateSecret(token)

	require.NoError(t, err)
	assert.Equal(t, token, runner.Token)
	assert.Regexp(t, "^[0-9a-f]{32}$", runner.TokenSalt)
	assert.Equal(t, runner.TokenHash, auth_model.HashToken(token, runner.TokenSalt))
}

func TestDeleteRunner(t *testing.T) {
	const recordID = 12345678
	require.NoError(t, unittest.PrepareTestDatabase())
	before := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: recordID})

	err := DeleteRunner(db.DefaultContext, &ActionRunner{ID: recordID})
	require.NoError(t, err)

	var after ActionRunner
	found, err := db.GetEngine(db.DefaultContext).ID(recordID).Unscoped().Get(&after)
	require.NoError(t, err)
	assert.True(t, found)

	// Most fields (namely Name, Version, OwnerID, RepoID, Description, Base, RepoRange,
	// TokenHash, TokenSalt, LastOnline, LastActive, AgentLabels and Created) are unaffected
	assert.Equal(t, before.Name, after.Name)
	assert.Equal(t, before.Version, after.Version)
	assert.Equal(t, before.OwnerID, after.OwnerID)
	assert.Equal(t, before.RepoID, after.RepoID)
	assert.Equal(t, before.Description, after.Description)
	assert.Equal(t, before.Base, after.Base)
	assert.Equal(t, before.RepoRange, after.RepoRange)
	assert.Equal(t, before.TokenHash, after.TokenHash)
	assert.Equal(t, before.TokenSalt, after.TokenSalt)
	assert.Equal(t, before.LastOnline, after.LastOnline)
	assert.Equal(t, before.LastActive, after.LastActive)
	assert.Equal(t, before.AgentLabels, after.AgentLabels)
	assert.Equal(t, before.Created, after.Created)

	// Deleted contains a value
	assert.NotNil(t, after.Deleted)

	// UUID was modified
	assert.NotEqual(t, before.UUID, after.UUID)
	// UUID starts with ffffffff-ffff-ffff-
	assert.Equal(t, "ffffffff-ffff-ffff-", after.UUID[:19])
	// UUID ends with LE binary representation of record ID
	idAsBinary := make([]byte, 8)
	binary.LittleEndian.PutUint64(idAsBinary, uint64(recordID))
	idAsHexadecimal := fmt.Sprintf("%.2x%.2x-%.2x%.2x%.2x%.2x%.2x%.2x", idAsBinary[0],
		idAsBinary[1], idAsBinary[2], idAsBinary[3], idAsBinary[4], idAsBinary[5],
		idAsBinary[6], idAsBinary[7])
	assert.Equal(t, idAsHexadecimal, after.UUID[19:])
}

func TestDeleteOfflineRunnersRunnerGlobalOnly(t *testing.T) {
	baseTime := time.Date(2024, 5, 19, 7, 40, 32, 0, time.UTC)
	timeutil.MockSet(baseTime)
	defer timeutil.MockUnset()

	require.NoError(t, unittest.PrepareTestDatabase())

	olderThan := timeutil.TimeStampNow().Add(-timeutil.Hour)

	require.NoError(t, DeleteOfflineRunners(db.DefaultContext, olderThan, true))

	// create at test base time
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 12345678})
	// last_online test base time
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000001})
	// created one month ago but a repo
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000002})
	// last online one hour ago
	unittest.AssertNotExistsBean(t, &ActionRunner{ID: 10000003})
	// last online 10 seconds ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000004})
	// created 1 month ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000005})
	// created 1 hour ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000006})
	// last online 1 hour ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000007})
}

func TestDeleteOfflineRunnersAll(t *testing.T) {
	baseTime := time.Date(2024, 5, 19, 7, 40, 32, 0, time.UTC)
	timeutil.MockSet(baseTime)
	defer timeutil.MockUnset()

	require.NoError(t, unittest.PrepareTestDatabase())

	olderThan := timeutil.TimeStampNow().Add(-timeutil.Hour)

	require.NoError(t, DeleteOfflineRunners(db.DefaultContext, olderThan, false))

	// create at test base time
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 12345678})
	// last_online test base time
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000001})
	// created one month ago
	unittest.AssertNotExistsBean(t, &ActionRunner{ID: 10000002})
	// last online one hour ago
	unittest.AssertNotExistsBean(t, &ActionRunner{ID: 10000003})
	// last online 10 seconds ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000004})
	// created 1 month ago
	unittest.AssertNotExistsBean(t, &ActionRunner{ID: 10000005})
	// created 1 hour ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000006})
	// last online 1 hour ago
	unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 10000007})
}

func TestDeleteOfflineRunnersErrorOnInvalidOlderThanValue(t *testing.T) {
	baseTime := time.Date(2024, 5, 19, 7, 40, 32, 0, time.UTC)
	timeutil.MockSet(baseTime)
	defer timeutil.MockUnset()
	require.Error(t, DeleteOfflineRunners(db.DefaultContext, timeutil.TimeStampNow(), false))
}

func TestRunnerEditable(t *testing.T) {
	testCases := []struct {
		name     string
		runner   *ActionRunner
		ownerID  int64
		repoID   int64
		editable bool
	}{
		{
			name:     "admin-can-edit-global-runner",
			runner:   &ActionRunner{Name: "global-runner", OwnerID: 0, RepoID: 0},
			ownerID:  0,
			repoID:   0,
			editable: true,
		},
		{
			name:     "admin-can-edit-user-runner",
			runner:   &ActionRunner{Name: "user-runner", OwnerID: 36, RepoID: 0},
			ownerID:  0,
			repoID:   0,
			editable: true,
		},
		{
			name:     "admin-can-edit-repository-runner",
			runner:   &ActionRunner{Name: "user-runner", OwnerID: 0, RepoID: 110},
			ownerID:  0,
			repoID:   0,
			editable: true,
		},
		{
			name:     "user-can-edit-its-runner",
			runner:   &ActionRunner{Name: "user-runner", OwnerID: 469, RepoID: 0},
			ownerID:  469,
			repoID:   0,
			editable: true,
		},
		{
			name:     "user-cannot-edit-global-runner",
			runner:   &ActionRunner{Name: "global-runner", OwnerID: 0, RepoID: 0},
			ownerID:  469,
			repoID:   0,
			editable: false,
		},
		{
			name:     "user-cannot-edit-other-users-runner",
			runner:   &ActionRunner{Name: "user-runner", OwnerID: 892, RepoID: 0},
			ownerID:  469,
			repoID:   0,
			editable: false,
		},
		{
			name:     "user-cannot-edit-repo-runner",
			runner:   &ActionRunner{Name: "repo-runner", OwnerID: 0, RepoID: 151},
			ownerID:  469,
			repoID:   0,
			editable: false,
		},
		{
			name:     "repo-can-edit-its-runner",
			runner:   &ActionRunner{Name: "repo-runner", OwnerID: 0, RepoID: 693},
			ownerID:  0,
			repoID:   693,
			editable: true,
		},
		{
			name:     "repo-cannot-edit-other-repo-runner",
			runner:   &ActionRunner{Name: "repo-runner", OwnerID: 0, RepoID: 519},
			ownerID:  0,
			repoID:   693,
			editable: false,
		},
		{
			name:     "repo-cannot-edit-global-runner",
			runner:   &ActionRunner{Name: "global-runner", OwnerID: 0, RepoID: 0},
			ownerID:  0,
			repoID:   693,
			editable: false,
		},
		{
			name:     "repo-cannot-edit-user-runner",
			runner:   &ActionRunner{Name: "user-runner", OwnerID: 6, RepoID: 0},
			ownerID:  0,
			repoID:   693,
			editable: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := testCase.runner.Editable(testCase.ownerID, testCase.repoID)
			assert.Equal(t, testCase.editable, result)
		})
	}
}

func TestRunner_GetVisibleRunnerByID(t *testing.T) {
	defer unittest.OverrideFixtures("models/actions/TestRunner_GetVisibleRunnerByID")()
	require.NoError(t, unittest.PrepareTestDatabase())

	repository32 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 32, OwnerID: 3})
	repository1 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 1, OwnerID: 2})

	runner1 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719931, OwnerID: 3, RepoID: 0}) // Owned by org3
	runner2 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719932, OwnerID: 2, RepoID: 0}) // Owned by user2
	runner3 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719933, OwnerID: 0, RepoID: 0})
	runner4 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719934, OwnerID: 0, RepoID: repository32.ID})

	testCases := []struct {
		name          string
		runner        *ActionRunner
		ownerID       int64
		repoID        int64
		expectedError string
	}{
		{
			name:          "Organization runner",
			runner:        runner1,
			ownerID:       3,
			repoID:        0,
			expectedError: "",
		},
		{
			name:          "Organization runner visible to admins",
			runner:        runner1,
			ownerID:       0,
			repoID:        0,
			expectedError: "",
		},
		{
			name:          "Organization runner invisible to different owner",
			runner:        runner1,
			ownerID:       2,
			repoID:        0,
			expectedError: fmt.Sprintf("runner with ID %d: resource does not exist", runner1.ID),
		},
		{
			name:          "Organization runner visible to its repositories",
			runner:        runner1,
			ownerID:       0,
			repoID:        repository32.ID,
			expectedError: "",
		},
		{
			name:          "Organization runner invisible to repositories owned by somebody else",
			runner:        runner1,
			ownerID:       0,
			repoID:        repository1.ID,
			expectedError: fmt.Sprintf("runner with ID %d: resource does not exist", runner1.ID),
		},
		{
			name:          "User runner",
			runner:        runner2,
			ownerID:       2,
			repoID:        0,
			expectedError: "",
		},
		{
			name:          "User runner invisible to different user",
			runner:        runner2,
			ownerID:       1,
			repoID:        0,
			expectedError: fmt.Sprintf("runner with ID %d: resource does not exist", runner2.ID),
		},
		{
			name:          "User runner visible to repository owned by user",
			runner:        runner2,
			ownerID:       0,
			repoID:        repository1.ID,
			expectedError: "",
		},
		{
			name:          "User runner invisible to repository owned by different user",
			runner:        runner2,
			ownerID:       0,
			repoID:        repository32.ID,
			expectedError: fmt.Sprintf("runner with ID %d: resource does not exist", runner2.ID),
		},
		{
			name:          "Global runner",
			runner:        runner3,
			ownerID:       0,
			repoID:        0,
			expectedError: "",
		},
		{
			name:          "Global runner is visible to any user",
			runner:        runner3,
			ownerID:       2,
			repoID:        0,
			expectedError: "",
		},
		{
			name:          "Global runner is visible to any repository",
			runner:        runner3,
			ownerID:       0,
			repoID:        repository32.ID,
			expectedError: "",
		},
		{
			name:          "Repository runner",
			runner:        runner4,
			ownerID:       0,
			repoID:        repository32.ID,
			expectedError: "",
		},
		{
			name:          "Repository runner is visible to admins",
			runner:        runner4,
			ownerID:       0,
			repoID:        0,
			expectedError: "",
		},
		{
			name:          "Repository runner is invisible to repository owner",
			runner:        runner4,
			ownerID:       repository32.OwnerID,
			repoID:        0,
			expectedError: fmt.Sprintf("runner with ID %d: resource does not exist", runner4.ID),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := GetVisibleRunnerByID(t.Context(), testCase.runner.ID, testCase.ownerID, testCase.repoID)
			if testCase.expectedError == "" {
				require.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, testCase.expectedError)
			}
		})
	}
}

func TestRunner_FindRunnerOptionsToConds(t *testing.T) {
	defer unittest.OverrideFixtures("models/actions/TestRunner_FindRunnerOptionsToConds")()
	require.NoError(t, unittest.PrepareTestDatabase())

	runner1 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719931, OwnerID: 3, RepoID: 0}) // Owned by org3
	runner2 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719932, OwnerID: 2, RepoID: 0}) // Owned by user2
	runner3 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719933, OwnerID: 0, RepoID: 0})
	runner4 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719934, OwnerID: 0, RepoID: 32})
	runner5 := unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: 719935, OwnerID: 0, RepoID: 36})

	testCases := []struct {
		name              string
		opts              FindRunnerOptions
		expectedRunners   RunnerList
		unexpectedRunners RunnerList
	}{
		{
			name:              "Only runners owned by instance",
			opts:              FindRunnerOptions{OwnerID: 0, RepoID: 0, WithVisible: false},
			expectedRunners:   RunnerList{runner3},
			unexpectedRunners: RunnerList{runner1, runner2, runner4, runner5},
		},
		{
			name:              "All runners on instance",
			opts:              FindRunnerOptions{OwnerID: 0, RepoID: 0, WithVisible: true},
			expectedRunners:   RunnerList{runner1, runner2, runner3, runner4, runner5},
			unexpectedRunners: RunnerList{},
		},
		{
			name:              "Only runners owned by organization",
			opts:              FindRunnerOptions{OwnerID: 3, RepoID: 0, WithVisible: false},
			expectedRunners:   RunnerList{runner1},
			unexpectedRunners: RunnerList{runner2, runner3, runner4, runner5},
		},
		{
			name:              "Runners available to organization",
			opts:              FindRunnerOptions{OwnerID: 3, RepoID: 0, WithVisible: true},
			expectedRunners:   RunnerList{runner1, runner3},
			unexpectedRunners: RunnerList{runner2, runner4, runner5},
		},
		{
			name:              "Only runners owned by user",
			opts:              FindRunnerOptions{OwnerID: 2, RepoID: 0, WithVisible: false},
			expectedRunners:   RunnerList{runner2},
			unexpectedRunners: RunnerList{runner1, runner3, runner4, runner5},
		},
		{
			name:              "Runners available to user",
			opts:              FindRunnerOptions{OwnerID: 2, RepoID: 0, WithVisible: true},
			expectedRunners:   RunnerList{runner2, runner3},
			unexpectedRunners: RunnerList{runner1, runner4, runner5},
		},
		{
			name:              "Only runners owned by organization repository",
			opts:              FindRunnerOptions{OwnerID: 0, RepoID: 32, WithVisible: false},
			expectedRunners:   RunnerList{runner4},
			unexpectedRunners: RunnerList{runner1, runner2, runner3, runner5},
		},
		{
			name:              "Runners available to organization repository",
			opts:              FindRunnerOptions{OwnerID: 0, RepoID: 32, WithVisible: true},
			expectedRunners:   RunnerList{runner1, runner3, runner4},
			unexpectedRunners: RunnerList{runner2, runner5},
		},
		{
			name:              "Only runners owned by user repository",
			opts:              FindRunnerOptions{OwnerID: 0, RepoID: 36, WithVisible: false},
			expectedRunners:   RunnerList{runner5},
			unexpectedRunners: RunnerList{runner1, runner2, runner3, runner4},
		},
		{
			name:              "Runners available to user repository",
			opts:              FindRunnerOptions{OwnerID: 0, RepoID: 36, WithVisible: true},
			expectedRunners:   RunnerList{runner2, runner3, runner5},
			unexpectedRunners: RunnerList{runner1, runner4},
		},
		{
			name:              "Runners with partially matching name",
			opts:              FindRunnerOptions{Filter: "er-3"},
			expectedRunners:   RunnerList{runner3},
			unexpectedRunners: RunnerList{runner1, runner2, runner4, runner5},
		},
		{
			name:              "Runners with partially matching UUID",
			opts:              FindRunnerOptions{Filter: "21f75233798b"},
			expectedRunners:   RunnerList{runner4},
			unexpectedRunners: RunnerList{runner1, runner2, runner3, runner5},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runners, err := db.Find[ActionRunner](t.Context(), testCase.opts)
			require.NoError(t, err)

			for _, expectedRunner := range testCase.expectedRunners {
				assert.Contains(t, runners, expectedRunner)
			}
			for _, unexpectedRunner := range testCase.unexpectedRunners {
				assert.NotContains(t, runners, unexpectedRunner)
			}
		})
	}
}

func TestUpdateRunner(t *testing.T) {
	t.Run("ownership is not altered", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		runnerUUID := "86b2f19a-3fbb-410b-ace6-2a2ace078a28"

		require.NoError(t, CreateRunner(t.Context(), &ActionRunner{UUID: runnerUUID, Name: "old name"}))

		runner := unittest.AssertExistsAndLoadBean(t, &ActionRunner{UUID: runnerUUID})

		assert.Zero(t, runner.OwnerID)
		assert.Zero(t, runner.RepoID)
		assert.Equal(t, "old name", runner.Name)

		runner.Name = "new name"

		require.NoError(t, UpdateRunner(t.Context(), runner))

		runner = unittest.AssertExistsAndLoadBean(t, &ActionRunner{UUID: runnerUUID})

		assert.Zero(t, runner.OwnerID)
		assert.Zero(t, runner.RepoID)
		assert.Equal(t, "new name", runner.Name)
	})

	t.Run("OwnerID and RepoID cannot be set simultaneously", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo62 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 62, OwnerID: user2.ID})
		runnerUUID := "86b2f19a-3fbb-410b-ace6-2a2ace078a28"

		require.NoError(t, CreateRunner(t.Context(), &ActionRunner{UUID: runnerUUID, OwnerID: user2.ID, Name: "old name"}))

		runner := unittest.AssertExistsAndLoadBean(t, &ActionRunner{UUID: runnerUUID})

		assert.Equal(t, user2.ID, runner.OwnerID)
		assert.Zero(t, runner.RepoID)
		assert.Equal(t, "old name", runner.Name)

		// Attempt to violate scoping rules by simultaneously setting OwnerID and RepoID.
		runner.RepoID = repo62.ID
		runner.Name = "new name"

		err := UpdateRunner(t.Context(), runner)
		require.ErrorContains(t, err, "OwnerID (2) and RepoID (62) of runner")

		// Verify that runner has not been changed.
		runner = unittest.AssertExistsAndLoadBean(t, &ActionRunner{UUID: runnerUUID})

		assert.Equal(t, user2.ID, runner.OwnerID)
		assert.Zero(t, runner.RepoID)
		assert.Equal(t, "old name", runner.Name)
	})
}
