// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"sort"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	"forgejo.org/models/perm"
	"forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/services/convert"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPITeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	teamUser := unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{ID: 1})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamUser.TeamID})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: teamUser.OrgID})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: teamUser.UID})

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiTeam api.Team
	DecodeJSON(t, resp, &apiTeam)
	assert.Equal(t, team.ID, apiTeam.ID)
	assert.Equal(t, team.Name, apiTeam.Name)

	toOrg := convert.ToOrganization(db.DefaultContext, org)
	assert.Equal(t, toOrg.ID, apiTeam.Organization.ID)
	assert.Equal(t, toOrg.AvatarURL, apiTeam.Organization.AvatarURL)
	assert.Equal(t, toOrg.Name, apiTeam.Organization.Name)
	assert.Equal(t, toOrg.FullName, apiTeam.Organization.FullName)
	assert.Equal(t, toOrg.Description, apiTeam.Organization.Description)
	assert.Equal(t, toOrg.Website, apiTeam.Organization.Website)
	assert.Equal(t, toOrg.Location, apiTeam.Organization.Location)
	assert.Equal(t, toOrg.Visibility, apiTeam.Organization.Visibility)
	assert.Equal(t, toOrg.RepoAdminChangeTeamAccess, apiTeam.Organization.RepoAdminChangeTeamAccess)
	assert.Equal(t, toOrg.Created.Local(), apiTeam.Organization.Created.Local())

	// non team member user will not access the teams details
	teamUser2 := unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: teamUser2.UID})

	session = loginUser(t, user2.Name)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID).
		AddTokenAuth(token)
	_ = MakeRequest(t, req, http.StatusForbidden)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID)
	_ = MakeRequest(t, req, http.StatusUnauthorized)

	// Get an admin user able to create, update and delete teams.
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session = loginUser(t, user.Name)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)

	org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})

	// Create team.
	teamToCreate := &api.CreateTeamOption{
		Name:                    "team1",
		Description:             "team one",
		IncludesAllRepositories: true,
		Permission:              "write",
		Units:                   []string{"repo.code", "repo.issues"},
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), teamToCreate).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "CreateTeam1", &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		teamToCreate.Permission, teamToCreate.Units, nil)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		teamToCreate.Permission, teamToCreate.Units, nil)
	teamID := apiTeam.ID

	// Edit team.
	editDescription := "team 1"
	editFalse := false
	teamToEdit := &api.EditTeamOption{
		Name:                    "teamone",
		Description:             &editDescription,
		Permission:              "admin",
		IncludesAllRepositories: &editFalse,
		Units:                   []string{"repo.code", "repo.pulls", "repo.releases"},
	}

	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEdit).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam1", &apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)

	// Edit team Description only
	editDescription = "first team"
	teamToEditDesc := api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEditDesc).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam1_DescOnly", &apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)

	// Read team.
	teamRead := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
	require.NoError(t, teamRead.LoadUnits(db.DefaultContext))
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "ReadTeam1", &apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		teamRead.AccessMode.String(), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})

	// create team again via UnitsMap
	// Create team.
	teamToCreate = &api.CreateTeamOption{
		Name:                    "team2",
		Description:             "team two",
		IncludesAllRepositories: true,
		Permission:              "write",
		UnitsMap:                map[string]string{"repo.code": "read", "repo.issues": "write", "repo.wiki": "none"},
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), teamToCreate).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "CreateTeam2", &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		"read", nil, teamToCreate.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		"read", nil, teamToCreate.UnitsMap)
	teamID = apiTeam.ID

	// Edit team.
	editDescription = "team 1"
	editFalse = false
	teamToEdit = &api.EditTeamOption{
		Name:                    "teamtwo",
		Description:             &editDescription,
		Permission:              "write",
		IncludesAllRepositories: &editFalse,
		UnitsMap:                map[string]string{"repo.code": "read", "repo.pulls": "read", "repo.releases": "write"},
	}

	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEdit).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam2", &apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)

	// Edit team Description only
	editDescription = "second team"
	teamToEditDesc = api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEditDesc).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam2_DescOnly", &apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)

	// Read team.
	teamRead = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	require.NoError(t, teamRead.LoadUnits(db.DefaultContext))
	checkTeamResponse(t, "ReadTeam2", &apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		teamRead.AccessMode.String(), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})

	// Create admin team
	teamToCreate = &api.CreateTeamOption{
		Name:                    "teamadmin",
		Description:             "team admin",
		IncludesAllRepositories: true,
		Permission:              "admin",
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), teamToCreate).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	for _, ut := range unit.AllRepoUnitTypes {
		up := perm.AccessModeAdmin
		if ut == unit.TypeExternalTracker || ut == unit.TypeExternalWiki {
			up = perm.AccessModeRead
		}
		unittest.AssertExistsAndLoadBean(t, &organization.TeamUnit{
			OrgID:      org.ID,
			TeamID:     apiTeam.ID,
			Type:       ut,
			AccessMode: up,
		})
	}
	teamID = apiTeam.ID

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})
}

func checkTeamResponse(t *testing.T, testName string, apiTeam *api.Team, name, description string, includesAllRepositories bool, permission string, units []string, unitsMap map[string]string) {
	t.Run(testName, func(t *testing.T) {
		assert.Equal(t, name, apiTeam.Name, "name")
		assert.Equal(t, description, apiTeam.Description, "description")
		assert.Equal(t, includesAllRepositories, apiTeam.IncludesAllRepositories, "includesAllRepositories")
		assert.Equal(t, permission, apiTeam.Permission, "permission")
		if units != nil {
			sort.StringSlice(units).Sort()
			sort.StringSlice(apiTeam.Units).Sort()
			assert.Equal(t, units, apiTeam.Units, "units")
		}
		if unitsMap != nil {
			assert.Equal(t, unitsMap, apiTeam.UnitsMap, "unitsMap")
		}
	})
}

func checkTeamBean(t *testing.T, id int64, name, description string, includesAllRepositories bool, permission string, units []string, unitsMap map[string]string) {
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: id})
	require.NoError(t, team.LoadUnits(db.DefaultContext), "LoadUnits")
	apiTeam, err := convert.ToTeam(db.DefaultContext, team)
	require.NoError(t, err)
	checkTeamResponse(t, fmt.Sprintf("checkTeamBean/%s_%s", name, description), apiTeam, name, description, includesAllRepositories, permission, units, unitsMap)
}

type TeamSearchResults struct {
	OK   bool        `json:"ok"`
	Data []*api.Team `json:"data"`
}

func TestAPITeamSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	var results TeamSearchResults

	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s", org.Name, "_team").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	assert.Len(t, results.Data, 1)
	assert.Equal(t, "test_team", results.Data[0].Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	token5 := getUserToken(t, user5.Name, auth_model.AccessTokenScopeReadOrganization)

	req = NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s", org.Name, "team").
		AddTokenAuth(token5)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIGetTeamReposAccessTokenResources(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestAPIGetTeamReposAccessTokenResources")()
	defer tests.PrepareTestEnv(t)()

	var repos []api.Repository

	// Test cases org3/repo21 (public), org3/repo3 (private), org3/repo5 (private) --
	// TestAPIGetTeamReposAccessTokenResources fixtures create a team w/ ID=26 that contains all three repos.
	session := loginUser(t, "user2")

	find := func() (bool, bool, bool) {
		foundRepo21 := false // public org3/repo21
		foundRepo3 := false  // private org3/repo3
		foundRepo5 := false  // second private repo org3/repo5 used in fine-grain testing, included as baseline
		for _, repo := range repos {
			switch repo.Name {
			case "repo21":
				foundRepo21 = true
			case "repo3":
				foundRepo3 = true
			case "repo5":
				foundRepo5 = true
			}
		}
		return foundRepo21, foundRepo3, foundRepo5
	}

	t.Run("all access token", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		allToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization)

		req := NewRequest(t, "GET", "/api/v1/teams/26/repos").AddTokenAuth(allToken)
		resp := MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repos)
		foundRepo21, foundRepo3, foundRepo5 := find()

		assert.True(t, foundRepo21) // public org3/repo21
		assert.True(t, foundRepo3)  // private org3/repo3
		assert.True(t, foundRepo5)  // private org3/repo5, used in fine-grain testing, included as baseline
	})

	t.Run("public-only access token", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		publicOnlyToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopePublicOnly, auth_model.AccessTokenScopeReadOrganization)

		req := NewRequest(t, "GET", "/api/v1/teams/26/repos").AddTokenAuth(publicOnlyToken)
		resp := MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repos)
		foundRepo21, foundRepo3, foundRepo5 := find()

		assert.True(t, foundRepo21) // public org3/repo21
		assert.False(t, foundRepo3) // private org3/repo3
		assert.False(t, foundRepo5) // private org3/repo5
	})

	t.Run("specific repo access token", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo2OnlyToken := createFineGrainedRepoAccessToken(t, "user2",
			[]auth_model.AccessTokenScope{auth_model.AccessTokenScopeReadOrganization},
			[]int64{3},
		)

		req := NewRequest(t, "GET", "/api/v1/teams/26/repos").AddTokenAuth(repo2OnlyToken)
		resp := MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repos)
		foundRepo21, foundRepo3, foundRepo5 := find()

		assert.True(t, foundRepo21) // public org3/repo21, allowed as it's public and read-access only
		assert.True(t, foundRepo3)  // private org3/repo3, allowed inside fine-grain
		assert.False(t, foundRepo5) // private org3/repo5, denied outside fine-grain
	})
}

func TestAPITeamRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	createSessionAndToken := func(username string) (*TestSession, string) {
		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)
		return session, token
	}
	ownerName := "user2"
	ownerSession, ownerToken := createSessionAndToken(ownerName)
	nonMemberName := "user5"
	nonMemberSession, nonMemberToken := createSessionAndToken(nonMemberName)
	memberName := "user4"
	memberSession, memberToken := createSessionAndToken(memberName)

	orgName := "testOrg"
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &api.CreateOrgOption{
		UserName:   orgName,
		Visibility: "public",
	}).
		AddTokenAuth(ownerToken)
	ownerSession.MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", orgName), &api.CreateTeamOption{
		Name:             "test",
		Permission:       "read",
		Units:            []string{"repo.code", "repo.issues", "repo.wiki", "repo.pulls", "repo.releases"},
		CanCreateOrgRepo: true,
	}).
		AddTokenAuth(ownerToken)
	resp := ownerSession.MakeRequest(t, req, http.StatusCreated)
	var team api.Team
	DecodeJSON(t, resp, &team)

	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/members/%s", team.ID, memberName)).
		AddTokenAuth(ownerToken)
	ownerSession.MakeRequest(t, req, http.StatusNoContent)

	repoName := "repoName"
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &api.CreateRepoOption{
		Name: repoName,
	}).AddTokenAuth(ownerToken)
	ownerSession.MakeRequest(t, req, http.StatusCreated)

	url := fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", team.ID, orgName, repoName)

	t.Run("AddTeamRepo", func(t *testing.T) {
		method := http.MethodPut

		req := NewRequest(t, method, url).AddTokenAuth(memberToken)
		resp := memberSession.MakeRequest(t, req, http.StatusForbidden)
		var err api.APIError
		DecodeJSON(t, resp, &err)
		assert.Equal(t, "user should be an owner or a collaborator with admin write of a repository", err.Message)

		req = NewRequest(t, method, url).AddTokenAuth(nonMemberToken)
		nonMemberSession.MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, method, url).AddTokenAuth(ownerToken)
		ownerSession.MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("GetTeamRepo", func(t *testing.T) {
		method := http.MethodGet

		req := NewRequest(t, method, url).AddTokenAuth(memberToken)
		memberSession.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, method, url).AddTokenAuth(nonMemberToken)
		nonMemberSession.MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, method, url).AddTokenAuth(ownerToken)
		ownerSession.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("RemoveTeamRepo", func(t *testing.T) {
		method := http.MethodDelete

		req := NewRequest(t, method, url).AddTokenAuth(memberToken)
		resp := memberSession.MakeRequest(t, req, http.StatusForbidden)
		var err api.APIError
		DecodeJSON(t, resp, &err)
		assert.Equal(t, "user should be an owner or a collaborator with admin write of a repository", err.Message)

		req = NewRequest(t, method, url).AddTokenAuth(nonMemberToken)
		nonMemberSession.MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, method, url).AddTokenAuth(ownerToken)
		ownerSession.MakeRequest(t, req, http.StatusNoContent)

		// verify the deletion is effective
		req = NewRequest(t, http.MethodGet, url).AddTokenAuth(ownerToken)
		ownerSession.MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIGetTeamRepoAccessTokenResources(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestAPIGetTeamRepoAccessTokenResources")()
	defer tests.PrepareTestEnv(t)()

	// Test cases org3/repo21 (public), org3/repo3 (private), org3/repo5 (private) --
	// TestAPIGetTeamRepoAccessTokenResources fixtures create a team w/ ID=26 that contains all three repos.
	session := loginUser(t, "user2")

	var repo api.Repository

	t.Run("all access token", func(t *testing.T) {
		allToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization)

		t.Run("allowed public repo21", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo21").AddTokenAuth(allToken)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &repo)
			assert.False(t, repo.Private)
		})
		t.Run("allowed private repo3", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo3").AddTokenAuth(allToken)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &repo)
			assert.True(t, repo.Private)
		})
		// org3/repo5 is a second repo used in fine-grain testing below, so we include it in other tests as a baseline
		t.Run("allowed private repo5", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo5").AddTokenAuth(allToken)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &repo)
			assert.True(t, repo.Private)
		})
	})

	t.Run("public-only access token", func(t *testing.T) {
		publicOnlyToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopePublicOnly, auth_model.AccessTokenScopeReadOrganization)

		t.Run("allowed public repo21", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo21").AddTokenAuth(publicOnlyToken)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &repo)
			assert.False(t, repo.Private)
		})
		t.Run("denied private repo3", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo3").AddTokenAuth(publicOnlyToken)
			MakeRequest(t, req, http.StatusNotFound)
		})
		t.Run("denied private repo5", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo5").AddTokenAuth(publicOnlyToken)
			MakeRequest(t, req, http.StatusNotFound)
		})
	})

	t.Run("specific repo access token", func(t *testing.T) {
		repo2OnlyToken := createFineGrainedRepoAccessToken(t, "user2",
			[]auth_model.AccessTokenScope{auth_model.AccessTokenScopeReadOrganization},
			[]int64{3},
		)

		t.Run("allowed public repo21", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo21").AddTokenAuth(repo2OnlyToken)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &repo)
			assert.False(t, repo.Private)
		})
		t.Run("allowed inside fine-grain repo3", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo3").AddTokenAuth(repo2OnlyToken)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &repo)
			assert.True(t, repo.Private)
		})
		t.Run("denied private outside fine-grain repo5", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/teams/26/repos/org3/repo5").AddTokenAuth(repo2OnlyToken)
			MakeRequest(t, req, http.StatusNotFound)
		})
	})
}

func TestAPIAddMemberDirectly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.AddMembersByInvitations, false)()
	token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteOrganization)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	req := NewRequestf(t, "PUT", "/api/v1/teams/%d/members/%s", team.ID, user5.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user5.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestAPIAddMemberGeneratesInvite(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.AddMembersByInvitations, true)()

	token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteOrganization)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	req := NewRequestf(t, "PUT", "/api/v1/teams/%d/members/%s", team.ID, user5.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user5.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	unittest.AssertExistsAndLoadBean(t, &organization.TeamInvite{TeamID: team.ID, InviterID: 1, InvitedID: optional.Some(user5.ID)})
}
