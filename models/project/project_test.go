// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
	project_module "forgejo.org/modules/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjects(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	projects, err := db.Find[Project](db.DefaultContext, SearchOptions{RepoID: 1})
	require.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, projects, 1)

	projects, err = db.Find[Project](db.DefaultContext, SearchOptions{RepoID: 3})
	require.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, projects, 1)
}

func TestCreateDeleteProject(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// wanted project settings
	wantTitle := "Testproject"
	wantDescription := "Test"
	wantOwnerID := user1.ID
	wantRepoID := int64(0)
	wantIsClosed := false
	wantTemplateType := project_module.TemplateTypeNone
	wantCardType := project_module.CardTypeTextOnly
	wantType := project_module.TypeIndividual

	// create project
	project := &Project{
		Title:        wantTitle,
		Description:  wantDescription,
		OwnerID:      wantOwnerID,
		Owner:        user1,
		RepoID:       wantRepoID,
		Repo:         &repo_model.Repository{},
		CreatorID:    wantOwnerID,
		IsClosed:     wantIsClosed,
		TemplateType: wantTemplateType,
		CardType:     wantCardType,
		Type:         wantType,
	}
	err := CreateProject(t.Context(), project)
	require.NoError(t, err)

	// check project in db
	projects, err := db.Find[Project](db.DefaultContext, SearchOptions{
		OwnerID:  wantOwnerID,
		RepoID:   wantRepoID,
		IsClosed: optional.Some(wantIsClosed),
		Type:     wantType,
		Title:    wantTitle,
	})
	require.NoError(t, err)
	assert.Len(t, projects, 1)
	assert.Equal(t, project.ID, projects[0].ID)
	assert.Equal(t, wantDescription, projects[0].Description)
	assert.Equal(t, wantOwnerID, projects[0].CreatorID)
	assert.Equal(t, wantTemplateType, projects[0].TemplateType)
	assert.Equal(t, wantCardType, projects[0].CardType)

	// try to create duplicate project
	err = CreateProject(t.Context(), project)
	require.ErrorContains(t, err, "unique constraint violation")

	// delete project
	err = DeleteProjectByID(t.Context(), project.ID, optional.None[int64]())
	require.NoError(t, err)
	unittest.AssertNotExistsBean(t, project)
}

func TestProjectsSort(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	tests := []struct {
		sortType string
		wants    []int64
	}{
		{
			sortType: "default",
			wants:    []int64{1, 3, 2, 7, 6, 5, 4},
		},
		{
			sortType: "oldest",
			wants:    []int64{4, 5, 6, 7, 2, 3, 1},
		},
		{
			sortType: "recentupdate",
			wants:    []int64{1, 3, 2, 7, 6, 5, 4},
		},
		{
			sortType: "leastupdate",
			wants:    []int64{4, 5, 6, 7, 2, 3, 1},
		},
	}

	for _, tt := range tests {
		projects, count, err := db.FindAndCount[Project](db.DefaultContext, SearchOptions{
			OrderBy: GetSearchOrderBySortType(tt.sortType),
		})
		require.NoError(t, err)
		assert.EqualValues(t, 7, count)
		if assert.Len(t, projects, 7) {
			for i := range projects {
				assert.Equal(t, tt.wants[i], projects[i].ID)
			}
		}
	}
}

func TestChangeProjectStatus(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Unchanged", func(t *testing.T) {
		project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

		require.NoError(t, ChangeProjectStatus(t.Context(), project, project.IsClosed))

		projectAfter := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
		assert.Equal(t, project.IsClosed, projectAfter.IsClosed)
	})

	t.Run("Normal", func(t *testing.T) {
		project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
		isClosed := !project.IsClosed
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: project.RepoID})

		require.NoError(t, ChangeProjectStatus(t.Context(), project, isClosed))

		projectAfter := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
		repoAfter := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: project.RepoID})
		assert.Equal(t, isClosed, projectAfter.IsClosed)
		assert.Equal(t, repo.NumProjects, repoAfter.NumProjects)
		assert.Equal(t, repo.NumOpenProjects-1, repoAfter.NumOpenProjects)
		assert.Equal(t, repo.NumClosedProjects+1, repoAfter.NumClosedProjects)
	})

	t.Run("Invalid ID", func(t *testing.T) {
		project := &Project{ID: 1001, RepoID: 1}
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: project.RepoID})

		require.NoError(t, ChangeProjectStatus(t.Context(), project, true))

		repoAfter := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: project.RepoID})
		assert.Equal(t, repo.NumProjects, repoAfter.NumProjects)
		assert.Equal(t, repo.NumOpenProjects, repoAfter.NumOpenProjects)
		assert.Equal(t, repo.NumClosedProjects, repoAfter.NumClosedProjects)
	})
}

func TestProjectLink(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	for _, tt := range []struct {
		name    string
		project *Project
		url     string
	}{
		{"User", &Project{OwnerID: user2.ID}, "/user2/-/projects/0"},
		{"Org", &Project{OwnerID: org3.ID}, "/org3/-/projects/0"},
		{"Repo", &Project{RepoID: repo2.ID}, "/user2/repo2/projects/0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.url, tt.project.Link(t.Context()))
		})
	}
}
