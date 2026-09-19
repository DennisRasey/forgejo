// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"errors"
	"slices"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/util"

	"xorm.io/builder"
)

// ProjectIssue saves relation from issue to a project
type ProjectIssue struct { //revive:disable-line:exported
	ID        int64 `xorm:"pk autoincr"`
	IssueID   int64 `xorm:"INDEX NOT NULL unique(project_issue)"`
	ProjectID int64 `xorm:"INDEX NOT NULL unique(project_issue)"`

	// ProjectColumnID should not be zero since 1.22. If it's zero, the issue will not be displayed on UI and it might result in errors.
	ProjectColumnID int64 `xorm:"'project_board_id' INDEX NOT NULL unique(column_sorting)"`

	// the sorting order on the column
	Sorting int64 `xorm:"NOT NULL DEFAULT 0 unique(column_sorting)"`
}

// FindProjectIssueOptions contains options to find project issues.
type FindProjectIssueOptions struct {
	db.ListOptions
	ProjectID       int64
	ProjectColumnID int64
}

func (opts FindProjectIssueOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.ProjectID != 0 {
		cond = cond.And(builder.Eq{"project_id": opts.ProjectID})
	}
	if opts.ProjectColumnID != 0 {
		cond = cond.And(builder.Eq{"project_board_id": opts.ProjectColumnID})
	}
	return cond
}

func (opts FindProjectIssueOptions) ToOrders() string {
	return "sorting, id"
}

func init() {
	db.RegisterModel(new(ProjectIssue))
}

func deleteProjectIssuesByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&ProjectIssue{})
	return err
}

// NumClosedIssues return counter of closed issues assigned to a project
func (p *Project) NumClosedIssues(ctx context.Context) int {
	c, err := db.GetEngine(ctx).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, true).
		Cols("issue_id").
		Count()
	if err != nil {
		log.Error("NumClosedIssues: %v", err)
		return 0
	}
	return int(c)
}

// NumOpenIssues return counter of open issues assigned to a project
func (p *Project) NumOpenIssues(ctx context.Context) int {
	c, err := db.GetEngine(ctx).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, false).
		Cols("issue_id").
		Count()
	if err != nil {
		log.Error("NumOpenIssues: %v", err)
		return 0
	}
	return int(c)
}

// MoveIssuesOnProjectColumn moves or keeps issues in a column and sorts them inside that column.
// The sortedIssueIDs map keys are sorting positions and values are issue IDs.
// Cards not in the map that already exist in the target column are shifted to
// positions after the highest requested sorting value.
func MoveIssuesOnProjectColumn(ctx context.Context, column *Column, sortedIssueIDs map[int64]int64) error {
	if len(sortedIssueIDs) == 0 {
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)
		issueIDs := util.ValuesOfMap(sortedIssueIDs)

		// Build reverse map: issueID → sorting and validate no duplicate issue IDs
		sortingByIssue := make(map[int64]int64, len(sortedIssueIDs))
		for sorting, issueID := range sortedIssueIDs {
			sortingByIssue[issueID] = sorting
		}
		if len(sortingByIssue) != len(sortedIssueIDs) {
			return errors.New("duplicate issue IDs in reorder request")
		}

		// Validate all issues exist and belong to this project
		count, err := sess.Table(new(ProjectIssue)).
			Where("project_id=?", column.ProjectID).
			In("issue_id", issueIDs).Count()
		if err != nil {
			return err
		}
		if int(count) != len(sortedIssueIDs) {
			return errors.New("all issues must belong to the specified project")
		}

		// Sort issue IDs to ensure consistent lock ordering across concurrent transactions.
		// This prevents deadlocks when multiple transactions update overlapping rows.
		slices.Sort(issueIDs)

		// Phase 1: Negate sorting for ALL cards currently in the target column
		// to free up all positive sorting positions. This prevents collisions
		// when moved cards are assigned their final positions.
		if _, err := sess.Exec("UPDATE `project_issue` SET sorting = -(sorting + 1) WHERE project_board_id=? AND sorting >= 0",
			column.ID); err != nil {
			return err
		}

		// Phase 2: Move the specified cards to the target column with their
		// final sorting values. Since all existing cards in the column now have
		// negative sorting, there are no collisions.
		for _, issueID := range issueIDs {
			_, err := sess.Exec("UPDATE `project_issue` SET project_board_id=?, sorting=? WHERE project_id=? AND issue_id=?",
				column.ID, sortingByIssue[issueID], column.ProjectID, issueID)
			if err != nil {
				return err
			}
		}

		// Phase 3: Re-pack any remaining cards in the column that still have
		// negative sorting (these are pre-existing cards NOT in the move set).
		// Assign them positions after the highest requested sorting value.
		var maxSorting int64
		for sorting := range sortedIssueIDs {
			if sorting > maxSorting {
				maxSorting = sorting
			}
		}

		var remainingCards []ProjectIssue
		if err := sess.Where("project_board_id=? AND sorting < 0", column.ID).
			OrderBy("sorting DESC"). // original order was -(original+1), so DESC gives ascending original order
			Find(&remainingCards); err != nil {
			return err
		}
		nextSorting := maxSorting + 1
		for _, card := range remainingCards {
			if _, err := sess.Exec("UPDATE `project_issue` SET sorting=? WHERE id=?",
				nextSorting, card.ID); err != nil {
				return err
			}
			nextSorting++
		}

		return nil
	})
}

func (c *Column) moveIssuesToAnotherColumn(ctx context.Context, newColumn *Column) error {
	if c.ProjectID != newColumn.ProjectID {
		return errors.New("columns have to be in the same project")
	}

	if c.ID == newColumn.ID {
		return nil
	}

	res := struct {
		MaxSorting int64
		IssueCount int64
	}{}
	if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as issue_count").
		Table("project_issue").
		Where("project_id=?", newColumn.ProjectID).
		And("project_board_id=?", newColumn.ID).
		Get(&res); err != nil {
		return err
	}

	issues, err := db.Find[ProjectIssue](ctx, FindProjectIssueOptions{
		ListOptions: db.ListOptionsAll, ProjectID: c.ProjectID, ProjectColumnID: c.ID,
	})
	if err != nil {
		return err
	}
	if len(issues) == 0 {
		return nil
	}

	nextSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)
	return db.WithTx(ctx, func(ctx context.Context) error {
		for i, issue := range issues {
			issue.ProjectColumnID = newColumn.ID
			issue.Sorting = nextSorting + int64(i)
			if _, err := db.GetEngine(ctx).ID(issue.ID).Cols("project_board_id", "sorting").Update(issue); err != nil {
				return err
			}
		}
		return nil
	})
}
