// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"testing"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	project_module "forgejo.org/modules/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultColumn(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	projectWithoutDefault, err := GetProjectByID(db.DefaultContext, 5)
	require.NoError(t, err)

	// check if default column was added
	column, err := projectWithoutDefault.GetDefaultColumn(db.DefaultContext)
	require.NoError(t, err)
	assert.EqualValues(t, 5, column.ProjectID)
	assert.Equal(t, "Uncategorized", column.Title)

	projectWithMultipleDefaults, err := GetProjectByID(db.DefaultContext, 6)
	require.NoError(t, err)

	// check if multiple defaults were removed
	column, err = projectWithMultipleDefaults.GetDefaultColumn(db.DefaultContext)
	require.NoError(t, err)
	assert.EqualValues(t, 6, column.ProjectID)
	assert.EqualValues(t, 9, column.ID)

	// set 8 as default column
	require.NoError(t, SetDefaultColumn(db.DefaultContext, column.ProjectID, 8))

	// then 9 will become a non-default column
	column, err = GetColumn(db.DefaultContext, 9)
	require.NoError(t, err)
	assert.EqualValues(t, 6, column.ProjectID)
	assert.False(t, column.Default)
}

func Test_moveIssuesToAnotherColumn(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	column1 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 1, ProjectID: 1})

	issues, total, err := db.FindAndCount[ProjectIssue](db.DefaultContext, FindProjectIssueOptions{
		ListOptions: db.ListOptionsAll, ProjectID: column1.ProjectID, ProjectColumnID: column1.ID,
	})
	require.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.EqualValues(t, 1, issues[0].ID)
	assert.EqualValues(t, 1, total)

	column2 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 2, ProjectID: 1})
	issues, total, err = db.FindAndCount[ProjectIssue](db.DefaultContext, FindProjectIssueOptions{
		ListOptions: db.ListOptionsAll, ProjectID: column2.ProjectID, ProjectColumnID: column2.ID,
	})
	require.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.EqualValues(t, 3, issues[0].ID)
	assert.EqualValues(t, 1, total)

	err = column1.moveIssuesToAnotherColumn(db.DefaultContext, column2)
	require.NoError(t, err)

	issues, total, err = db.FindAndCount[ProjectIssue](db.DefaultContext, FindProjectIssueOptions{
		ListOptions: db.ListOptionsAll, ProjectID: column1.ProjectID, ProjectColumnID: column1.ID,
	})
	require.NoError(t, err)
	assert.Empty(t, issues)
	assert.EqualValues(t, 0, total)

	issues, total, err = db.FindAndCount[ProjectIssue](db.DefaultContext, FindProjectIssueOptions{
		ListOptions: db.ListOptionsAll, ProjectID: column2.ProjectID, ProjectColumnID: column2.ID,
	})
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.EqualValues(t, 3, issues[0].ID)
	assert.EqualValues(t, 0, issues[0].Sorting)
	assert.EqualValues(t, 1, issues[1].ID)
	assert.EqualValues(t, 1, issues[1].Sorting)
	assert.EqualValues(t, 2, total)
}

func Test_MoveColumnsOnProject(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, total, err := db.FindAndCount[Column](db.DefaultContext, FindColumnOptions{
		ListOptions: db.ListOptionsAll, ProjectID: project1.ID,
	})
	require.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting)
	assert.EqualValues(t, 1, columns[1].Sorting)
	assert.EqualValues(t, 2, columns[2].Sorting)
	assert.EqualValues(t, 3, total)

	err = MoveColumnsOnProject(db.DefaultContext, project1.ID, map[int64]int64{
		0: columns[1].ID,
		1: columns[2].ID,
		2: columns[0].ID,
	})
	require.NoError(t, err)

	columnsAfter, total, err := db.FindAndCount[Column](db.DefaultContext, FindColumnOptions{
		ListOptions: db.ListOptionsAll, ProjectID: project1.ID,
	})
	require.NoError(t, err)
	assert.Len(t, columnsAfter, 3)
	assert.Equal(t, columns[1].ID, columnsAfter[0].ID)
	assert.Equal(t, columns[2].ID, columnsAfter[1].ID)
	assert.Equal(t, columns[0].ID, columnsAfter[2].ID)
	assert.EqualValues(t, 3, total)
}

func TestMoveColumnsOnProjectSwap(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, total, err := db.FindAndCount[Column](db.DefaultContext, FindColumnOptions{
		ListOptions: db.ListOptionsAll, ProjectID: project1.ID,
	})
	require.NoError(t, err)
	require.Len(t, columns, 3)
	require.EqualValues(t, 3, total)

	// First give them distinct positions
	err = MoveColumnsOnProject(db.DefaultContext, project1.ID, map[int64]int64{
		0: columns[0].ID,
		1: columns[1].ID,
		2: columns[2].ID,
	})
	require.NoError(t, err)

	// Now swap columns 0 and 1 (would collide under single-phase update)
	err = MoveColumnsOnProject(db.DefaultContext, project1.ID, map[int64]int64{
		0: columns[1].ID,
		1: columns[0].ID,
		2: columns[2].ID,
	})
	require.NoError(t, err)

	columnsAfter, total, err := db.FindAndCount[Column](db.DefaultContext, FindColumnOptions{
		ListOptions: db.ListOptionsAll, ProjectID: project1.ID,
	})
	require.NoError(t, err)
	assert.Len(t, columnsAfter, 3)
	assert.Equal(t, columns[1].ID, columnsAfter[0].ID)
	assert.Equal(t, columns[0].ID, columnsAfter[1].ID)
	assert.Equal(t, columns[2].ID, columnsAfter[2].ID)
	assert.EqualValues(t, 3, total)
}

func TestUpdateColumnSortingZero(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	column := unittest.AssertExistsAndLoadBean(t, &Column{ID: 1})
	column.Sorting = 5
	require.NoError(t, UpdateColumn(db.DefaultContext, column))

	// Verify it was set to 5
	updated, err := GetColumn(db.DefaultContext, column.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 5, updated.Sorting)

	// Now set it back to 0
	column.Sorting = 0
	require.NoError(t, UpdateColumn(db.DefaultContext, column))

	updated, err = GetColumn(db.DefaultContext, column.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, updated.Sorting)
}

func Test_NewColumn(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, total, err := db.FindAndCount[Column](db.DefaultContext, FindColumnOptions{
		ListOptions: db.ListOptionsAll, ProjectID: project1.ID,
	})
	require.NoError(t, err)
	assert.Len(t, columns, 3)
	require.EqualValues(t, 3, total)

	for i := range maxProjectColumns - 3 {
		err := CreateColumn(db.DefaultContext, &Column{
			Title:     fmt.Sprintf("column-%d", i+4),
			ProjectID: project1.ID,
		})
		require.NoError(t, err)
	}
	err = CreateColumn(db.DefaultContext, &Column{
		Title:     "column-21",
		ProjectID: project1.ID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "maximum number of columns reached")
}

// TestCreateColumnDefault tests CreateColumn in an empty project without a default column.
func TestCreateColumnDefault(t *testing.T) {
	// create empty project
	require.NoError(t, unittest.PrepareTestDatabase())
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	project := &Project{
		Title:        "Testproject",
		Description:  "Test",
		OwnerID:      user1.ID,
		Owner:        user1,
		RepoID:       0,
		Repo:         &repo_model.Repository{},
		CreatorID:    user1.ID,
		IsClosed:     false,
		TemplateType: project_module.TemplateTypeNone,
		CardType:     project_module.CardTypeTextOnly,
		Type:         project_module.TypeIndividual,
	}
	err := CreateProject(t.Context(), project)
	require.NoError(t, err)

	// create column
	column := &Column{
		Title:     "new column",
		ProjectID: project.ID,
	}
	err = CreateColumn(t.Context(), column)
	require.NoError(t, err)

	// new column should be default column
	assert.True(t, column.Default)
}

// TestGetColumnsPagination tests getting columns with pagination.
func TestGetColumnsPagination(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	for _, tt := range []struct {
		name     string
		pageSize int
		page     int
		columns  int
	}{
		// one per page
		{name: "one per page, first page", pageSize: 1, page: 1, columns: 1},
		{name: "one per page, second page", pageSize: 1, page: 2, columns: 1},
		{name: "one per page, third page", pageSize: 1, page: 3, columns: 1},
		// two per page
		{name: "two per page, first page", pageSize: 2, page: 1, columns: 2},
		{name: "two per page, second page", pageSize: 2, page: 2, columns: 1},
		// three+ per page
		{name: "three per page", pageSize: 3, page: 1, columns: 3},
		{name: "four per page", pageSize: 4, page: 1, columns: 3},
		{name: "30 per page", pageSize: 30, page: 1, columns: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			columns, total, err := db.FindAndCount[Column](t.Context(), FindColumnOptions{
				ListOptions: db.ListOptions{
					PageSize: tt.pageSize,
					Page:     tt.page,
				},
				ProjectID: project1.ID,
			})
			require.NoError(t, err)
			assert.Len(t, columns, tt.columns)
			assert.EqualValues(t, 3, total)
		})
	}
}

// TestColumnGetIssuesPagination tests GetIssues of Column with pagination.
func TestColumnGetIssuesPagination(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// get columns
	column1 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 1, ProjectID: 1})
	column2 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 2, ProjectID: 1})
	column3 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 3, ProjectID: 1})

	// move all issues to one column to have more issues in one column
	require.NoError(t, column2.moveIssuesToAnotherColumn(db.DefaultContext, column1))
	require.NoError(t, column3.moveIssuesToAnotherColumn(db.DefaultContext, column1))

	// test getting issues with pagination
	for _, tt := range []struct {
		name     string
		pageSize int
		page     int
		issues   int
	}{
		// one per page
		{name: "one per page, first page", pageSize: 1, page: 1, issues: 1},
		{name: "one per page, second page", pageSize: 1, page: 2, issues: 1},
		{name: "one per page, third page", pageSize: 1, page: 3, issues: 1},
		// two per page
		{name: "two per page, first page", pageSize: 2, page: 1, issues: 2},
		{name: "two per page, second page", pageSize: 2, page: 2, issues: 1},
		// three+ per page
		{name: "three per page", pageSize: 3, page: 1, issues: 3},
		{name: "four per page", pageSize: 4, page: 1, issues: 3},
		{name: "30 per page", pageSize: 30, page: 1, issues: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issues, total, err := db.FindAndCount[ProjectIssue](t.Context(),
				FindProjectIssueOptions{
					ListOptions: db.ListOptions{
						PageSize: tt.pageSize,
						Page:     tt.page,
					},
					ProjectID:       column1.ProjectID,
					ProjectColumnID: column1.ID,
				},
			)
			require.NoError(t, err)
			assert.Len(t, issues, tt.issues)
			assert.EqualValues(t, 3, total)
		})
	}
}

// TestDeleteColumnByID tests DeleteColumnByID.
func TestDeleteColumnByID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	column := unittest.AssertExistsAndLoadBean(t, &Column{ID: 3, ProjectID: 1})

	// delete existing column
	err := DeleteColumnByID(t.Context(), column.ID)
	require.NoError(t, err)
	unittest.AssertNotExistsBean(t, column)

	// delete not existing column
	err = DeleteColumnByID(t.Context(), column.ID)
	require.NoError(t, err)
}
