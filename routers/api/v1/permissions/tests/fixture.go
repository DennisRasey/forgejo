// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

// See README.md for a documentation of the test logic

import (
	"fmt"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	org_model "forgejo.org/models/organization"
	"forgejo.org/models/perm"
	access_model "forgejo.org/models/perm/access"
	repo_model "forgejo.org/models/repo"
	unit_model "forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/util"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
	"forgejo.org/services/auth"
	"forgejo.org/services/authz"
	issue_service "forgejo.org/services/issue"
	packages_service "forgejo.org/services/packages"
	pull_service "forgejo.org/services/pull"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/require"
)

func fixtureCreateToken(t *testing.T, user *user_model.User, scope auth_model.AccessTokenScope, repoIDs ...int64) (*auth_model.AccessToken, error) {
	t.Helper()
	scope, err := scope.Normalize()
	require.NoError(t, err)
	resourceAllRepos := len(repoIDs) == 0
	accessToken := &auth_model.AccessToken{
		UID:              user.ID,
		Name:             util.CryptoRandomString(10),
		Scope:            scope,
		ResourceAllRepos: resourceAllRepos,
	}
	require.NoError(t, auth_model.NewAccessToken(t.Context(), accessToken))
	if len(repoIDs) > 0 {
		var resourceRepos []*auth_model.AccessTokenResourceRepo
		for _, repoID := range repoIDs {
			resourceRepos = append(resourceRepos, &auth_model.AccessTokenResourceRepo{
				TokenID: accessToken.ID,
				RepoID:  repoID,
			})
		}
		require.NoError(t, auth_model.InsertAccessTokenResourceRepos(t.Context(), accessToken.ID, resourceRepos))
	}
	return accessToken, nil
}

func fixtureCreateIssue(t *testing.T, user *user_model.User, repo *repo_model.Repository, title, content string) *issues_model.Issue {
	t.Helper()
	issue := &issues_model.Issue{
		RepoID:   repo.ID,
		Title:    title,
		Content:  content,
		PosterID: user.ID,
		Poster:   user,
	}

	err := issue_service.NewIssue(t.Context(), repo, issue, nil, nil, nil)
	require.NoError(t, err)

	return issue
}

func fixtureGetUser(t *testing.T, name string) *user_model.User {
	t.Helper()
	existingUser, err := user_model.GetUserByName(t.Context(), name)
	if err == nil {
		return existingUser
	} else if !user_model.IsErrUserNotExist(err) {
		require.NoError(t, err)
	}
	return nil
}

func fixtureGetOrg(t *testing.T, name string) *org_model.Organization {
	t.Helper()
	return (*org_model.Organization)(fixtureGetUser(t, name))
}

func fixtureCreateUser(t *testing.T, user *user_model.User) *user_model.User {
	t.Helper()
	if existingUser := fixtureGetUser(t, user.Name); existingUser != nil {
		return existingUser
	}
	user.Email = user.Name + "@test.forgejo.org"
	user.Passwd = "password"
	overwriteDefault := &user_model.CreateUserOverwriteOptions{}
	visibility := user.Visibility
	overwriteDefault.Visibility = &visibility
	require.NoError(t, user_model.CreateUser(t.Context(), user, overwriteDefault))
	return user
}

func fixtureCreateOrg(t *testing.T, org *org_model.Organization, owner *user_model.User) *org_model.Organization {
	t.Helper()
	if existing := fixtureGetOrg(t, org.Name); existing != nil {
		return existing
	}
	owner = fixtureCreateUser(t, owner)
	require.NoError(t, org_model.CreateOrganization(t.Context(), org, owner))
	return org
}

func fixtureCreateTeams(t *testing.T, org *org_model.Organization, teams string) {
	t.Helper()

	for team := range strings.SplitSeq(teams, ",") {
		teamName, memberName, found := strings.Cut(team, ":")
		require.True(t, found)
		fixtureCreateTeam(t, org, memberName, &forgery.CreateTeamOptions{
			Name: teamName,
			Mode: perm.AccessModeWrite,
		})
	}
}

func fixtureCreateTeam(t *testing.T, org *org_model.Organization, memberName string, opts *forgery.CreateTeamOptions) *org_model.Team {
	t.Helper()

	member := fixtureCreateUser(t, &user_model.User{Name: memberName})
	opts.Members = []*user_model.User{member}
	team := forgery.CreateTeam(t, org, opts)
	require.NotNil(t, team)
	return team
}

func fixtureSetPackageOwner(t *testing.T, permissions *apiv1_permissions.Permissions, packageOwner, visibility string) {
	t.Helper()
	if packageOwner == "" {
		return
	}
	owner := fixtureCreateUser(t, &user_model.User{Name: packageOwner, Visibility: stringToVisibility(visibility)})
	permissions.SetPackageOwner(owner)
	mode, err := packages_service.DeterminePackageAccessMode(permissions.Context(), permissions.PackageOwner(), permissions.Doer())
	require.NoError(t, err)
	permissions.SetPackageAccessMode(mode)
}

func fixtureSetDoer(t *testing.T, permissions *apiv1_permissions.Permissions, testData *testData) {
	t.Helper()
	if testData.shared.Anonymous() {
		permissions.SetAuthentication(&auth.UnauthenticatedResult{})
		return
	}
	name := testData.shared.DoerName()
	if doer := permissions.Doer(); doer != nil {
		if doer.Name != name {
			panic(fmt.Sprintf("attempting to override doer %s with %s", doer.Name, name))
		}
		return
	}
	if name == user_model.ActionsUserName {
		fixtureSetDoerActionsUser(t, permissions, testData.shared)
	} else {
		fixtureSetDoerRegularUser(t, permissions, testData.shared)
	}
}

var _ auth.AuthenticationResult = &actionsTaskTokenAuthenticationResult{}

type actionsTaskTokenAuthenticationResult struct {
	*auth.BaseAuthenticationResult
	user   *user_model.User
	taskID int64
}

func (r *actionsTaskTokenAuthenticationResult) Scope() optional.Option[auth_model.AccessTokenScope] {
	return optional.None[auth_model.AccessTokenScope]()
}

func (r *actionsTaskTokenAuthenticationResult) User() *user_model.User {
	return r.user
}

func (r *actionsTaskTokenAuthenticationResult) ActionsTaskID() optional.Option[int64] {
	return optional.Some(r.taskID)
}

func fixtureSetDoerActionsUser(t *testing.T, permissions *apiv1_permissions.Permissions, data *sharedData) {
	permissions.SetDoer(user_model.NewActionsUser())
	repository := permissions.Repository()
	require.NotNil(t, repository)
	repositoryID := repository.ID
	if data.HasDoerActionsRepoID() {
		repositoryID = data.DoerActionsRepoID()
	}
	task := &actions_model.ActionTask{
		RepoID: repositoryID,
	}
	task.IsForkPullRequest = data.DoerActionsIsForkPullRequest()
	task.GenerateToken()
	{
		_, err := db.GetEngine(t.Context()).Insert(task)
		require.NoError(t, err)
		require.NotZero(t, task.ID)
	}

	permissions.SetAuthentication(&actionsTaskTokenAuthenticationResult{user: permissions.Doer(), taskID: task.ID})
	permissions.SetReducer(&authz.AllAccessAuthorizationReducer{})
	permission, err := access_model.GetUserRepoPermissionWithReducer(permissions.Context(), permissions.Repository(), permissions.Doer(), permissions.Reducer())
	require.NoError(t, err)
	permissions.SetPermission(&permission)
}

var _ auth.AuthenticationResult = &basicPasswordAuthenticationResult{}

type basicPasswordAuthenticationResult struct {
	*auth.BaseAuthenticationResult
	user *user_model.User
}

func (*basicPasswordAuthenticationResult) IsPasswordAuthentication() bool {
	return true
}

func (r *basicPasswordAuthenticationResult) User() *user_model.User {
	return r.user
}

var _ auth.AuthenticationResult = &accessTokenAuthenticationResult{}

type accessTokenAuthenticationResult struct {
	*auth.BaseAuthenticationResult
	user    *user_model.User
	scope   auth_model.AccessTokenScope
	reducer authz.AuthorizationReducer
}

func (r *accessTokenAuthenticationResult) User() *user_model.User {
	return r.user
}

func (r *accessTokenAuthenticationResult) Scope() optional.Option[auth_model.AccessTokenScope] {
	return optional.Some(r.scope)
}

func (r *accessTokenAuthenticationResult) Reducer() authz.AuthorizationReducer {
	return r.reducer
}

var _ auth.AuthenticationResult = &reverseProxyAuthenticationResult{}

type reverseProxyAuthenticationResult struct {
	*auth.BaseAuthenticationResult
	user *user_model.User
}

func (r *reverseProxyAuthenticationResult) User() *user_model.User {
	return r.user
}

func (*reverseProxyAuthenticationResult) IsReverseProxyAuthentication() bool {
	return true
}

func fixtureSetDoerRegularUser(t *testing.T, permissions *apiv1_permissions.Permissions, data *sharedData) {
	var scope auth_model.AccessTokenScope
	if data.HasDoerScope() {
		scope = auth_model.AccessTokenScope(data.DoerScope())
	} else {
		scope = auth_model.AccessTokenScopeAll
	}
	if !data.Anonymous() {
		user := &user_model.User{
			Name:    data.DoerName(),
			IsAdmin: data.DoerAdmin(),
		}
		fixtureCreateUser(t, user)
		permissions.SetDoer(user)
	} else {
		panic(fmt.Errorf("attempting to set doer with no name"))
	}

	token, err := fixtureCreateToken(t, permissions.Doer(), scope)
	require.NoError(t, err)
	tokenReducer, err := authz.GetAuthorizationReducerForAccessToken(t.Context(), token)
	require.NoError(t, err)
	permissions.SetIsSigned(true)
	switch data.DoerAuthentication() {
	case "basic":
		permissions.SetAuthentication(&basicPasswordAuthenticationResult{user: permissions.Doer()})
	case "proxy":
		permissions.SetAuthentication(&reverseProxyAuthenticationResult{user: permissions.Doer()})
	default:
		permissions.SetToken(token)
		permissions.SetAuthentication(&accessTokenAuthenticationResult{user: permissions.Doer(), scope: token.Scope, reducer: tokenReducer})
	}
}

func fixtureCreateBranch(t *testing.T, permissions *apiv1_permissions.Permissions, branch string) {
	t.Helper()
	repository := permissions.Repository()
	require.NotNil(t, repository)

	gitRepo, err := git.OpenRepository(t.Context(), repository.RepoPath())
	require.NoError(t, err)
	defaultBranch, err := git.GetDefaultBranch(t.Context(), repository.RepoPath())
	require.NoError(t, err)
	require.NoError(t, gitRepo.CreateBranch(branch, defaultBranch))
}

func fixtureCreatePullRequest(t *testing.T, permissions *apiv1_permissions.Permissions, pullRequest, pullRequestAuthor, pullRequestBranch string) {
	t.Helper()
	if pullRequest == "" {
		return
	}

	repository := permissions.Repository()
	require.NotNil(t, repository)

	poster := fixtureGetUser(t, pullRequestAuthor)
	require.NotNil(t, poster)

	ctx, committer, err := db.TxContext(t.Context())
	require.NoError(t, err)
	defer committer.Close()

	idx, err := db.GetNextResourceIndex(ctx, "issue_index", repository.ID)
	if err != nil {
		panic(fmt.Errorf("generate issue index failed: %w", err))
	}
	issue := &issues_model.Issue{
		Index:    idx,
		RepoID:   repository.ID,
		IsPull:   true,
		Title:    pullRequest,
		PosterID: poster.ID,
		Poster:   poster,
	}

	sess := db.GetEngine(ctx)

	if _, err = sess.NoAutoTime().Insert(issue); err != nil {
		panic(err)
	}
	issue.PullRequest = &issues_model.PullRequest{}

	pr := issue.PullRequest
	pr.Index = issue.Index
	pr.IssueID = issue.ID
	pr.HeadRepoID = repository.ID
	pr.BaseRepoID = repository.ID
	pr.HeadBranch = pullRequestBranch
	_, err = sess.NoAutoTime().Insert(pr)
	require.NoError(t, err)
	require.NoError(t, committer.Commit())
	require.NoError(t, pr.LoadBaseRepo(ctx))
	require.NoError(t, pr.LoadHeadRepo(ctx))
	require.NoError(t, pull_service.PushToBaseRepo(ctx, pr))
}

func fixtureSetRepository(t *testing.T, permissions *apiv1_permissions.Permissions, name string, init, private, archived bool) {
	t.Helper()
	if name == "" {
		return
	}
	if repository := permissions.Repository(); repository != nil {
		if repository.FullName() != name {
			panic(fmt.Sprintf("attempting to override already repository %s with %s", repository.FullName(), name))
		}
		return
	}
	ownerName, repoName, found := strings.Cut(name, "/")
	require.True(t, found)
	owner := fixtureCreateUser(t, &user_model.User{Name: ownerName})
	opts := &forgery.CreateRepositoryOptions{
		Name:      repoName,
		IsPrivate: private,
	}
	if init {
		opts.Files = forgery.FilesInit{}
	}
	repository := forgery.CreateRepository(t, owner, opts)
	// some of it is redundant with the config but that makes
	// the tests immune to changes in the defaults
	for _, unitType := range unit_model.DefaultRepoUnits {
		forgery.EnableRepoUnit(t, repository, unitType, nil)
	}
	if archived {
		require.NoError(t, repo_model.SetArchiveRepoState(t.Context(), repository, true))
	}
	permissions.SetRepository(repository)
}

func fixtureGetIssue(t *testing.T, issueName string) *issues_model.Issue {
	t.Helper()
	var issue issues_model.Issue
	found, err := db.GetEngine(t.Context()).Where("name = ?", issueName).Get(&issue)
	require.NoError(t, err)
	if !found {
		return nil
	}
	issue.LoadPoster(t.Context())
	return &issue
}

func fixtureSetIssue(t *testing.T, permissions *apiv1_permissions.Permissions, issueName, issueAuthor string) *issues_model.Issue {
	t.Helper()
	issue := fixtureGetIssue(t, issueName)
	if issue == nil {
		author := fixtureCreateUser(t, &user_model.User{Name: issueAuthor})
		issue = fixtureCreateIssue(t, author, permissions.Repository(), issueName, "issue description")
	}
	return issue
}

func fixtureGetComment(t *testing.T, content string) *issues_model.Comment {
	var comment issues_model.Comment
	found, err := db.GetEngine(t.Context()).Where("content = ?", content).Get(&comment)
	require.NoError(t, err)
	if !found {
		return nil
	}
	_ = comment.LoadIssue(t.Context())
	return &comment
}

func fixtureCreateComment(t *testing.T, permissions *apiv1_permissions.Permissions, issue *issues_model.Issue, comment string) {
	t.Helper()
	if fixtureGetComment(t, comment) == nil {
		_, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypeComment,
			Doer:    issue.Poster,
			Issue:   issue,
			Repo:    permissions.Repository(),
			Content: comment,
		})
		require.NoError(t, err)
	}
}

func fixtureDisableRepoUnit(t *testing.T, permissions *apiv1_permissions.Permissions, unitType unit_model.Type) {
	t.Helper()
	repo := permissions.Repository()
	require.NotNil(t, repo)
	forgery.DisableRepoUnits(t, repo, unitType)
}

func fixtureDisableUnits(t *testing.T, permissions *apiv1_permissions.Permissions, unitTypes []unit_model.Type) {
	t.Helper()
	for _, unitType := range unitTypes {
		fixtureDisableRepoUnit(t, permissions, unitType)
	}
}
