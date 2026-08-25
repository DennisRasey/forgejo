// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"fmt"

	"forgejo.org/models/gitea_migrations/base"
	"forgejo.org/modules/setting"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "Fix duplicate project sorting values and add unique constraints",
		Upgrade:     fixProjectSortingUniqueConstraints,
	})
}

func fixProjectSortingUniqueConstraints(x *xorm.Engine) error {
	// Step 1: Fix existing duplicates in project_issue (cards)
	// Reassign sequential sorting values within each column
	if err := fixProjectIssueDuplicates(x); err != nil {
		return err
	}

	// Step 2: Fix existing duplicates in project_board (columns)
	// Reassign sequential sorting values within each project
	if err := fixProjectBoardDuplicates(x); err != nil {
		return err
	}

	// Step 3: Remove duplicate (project_id, issue_id) rows keeping the lowest id
	if err := fixProjectIssueDuplicateAssignments(x); err != nil {
		return err
	}

	// Step 4: Add unique constraints (idempotent — skip if already exists)
	if err := createUniqueIndexIfNotExists(x, "project_issue", "UQE_project_issue_column_sorting", "project_board_id, sorting"); err != nil {
		return err
	}
	if err := createUniqueIndexIfNotExists(x, "project_issue", "UQE_project_issue_project_issue", "project_id, issue_id"); err != nil {
		return err
	}
	if err := createUniqueIndexIfNotExists(x, "project_board", "UQE_project_board_project_sorting", "project_id, sorting"); err != nil {
		return err
	}

	// Step 5: Enforce NOT NULL on project_issue columns.
	// The struct tags declare NOT NULL but the DB may not enforce it.
	// On SQLite, RecreateTables rebuilds the table with NOT NULL and unique constraints.
	return enforceProjectIssueNotNull(x)
}

func createUniqueIndexIfNotExists(x *xorm.Engine, tableName, indexName, columns string) error {
	exists, err := indexExists(x, tableName, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = x.Exec(fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", indexName, tableName, columns))
	return err
}

func fixProjectIssueDuplicates(x *xorm.Engine) error {
	switch {
	case setting.Database.Type.IsSQLite3():
		// SQLite: Use UPDATE with subquery
		_, err := x.Exec(`
			UPDATE project_issue SET sorting = (
				SELECT new_sort FROM (
					SELECT id, ROW_NUMBER() OVER (PARTITION BY project_board_id ORDER BY sorting, id) - 1 as new_sort
					FROM project_issue
				) ranked WHERE ranked.id = project_issue.id
			)
		`)
		return err

	case setting.Database.Type.IsPostgreSQL():
		// PostgreSQL: Use UPDATE FROM with subquery
		_, err := x.Exec(`
			UPDATE project_issue pi SET sorting = ranked.new_sort
			FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY project_board_id ORDER BY sorting, id) - 1 as new_sort
				FROM project_issue
			) ranked
			WHERE pi.id = ranked.id
		`)
		return err

	case setting.Database.Type.IsMySQL():
		// MySQL: Use UPDATE with JOIN
		_, err := x.Exec(`
			UPDATE project_issue pi
			INNER JOIN (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY project_board_id ORDER BY sorting, id) - 1 as new_sort
				FROM project_issue
			) ranked ON pi.id = ranked.id
			SET pi.sorting = ranked.new_sort
		`)
		return err

	default:
		return fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
	}
}

func fixProjectBoardDuplicates(x *xorm.Engine) error {
	switch {
	case setting.Database.Type.IsSQLite3():
		// SQLite: Use UPDATE with subquery
		_, err := x.Exec(`
			UPDATE project_board SET sorting = (
				SELECT new_sort FROM (
					SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY sorting, id) - 1 as new_sort
					FROM project_board
				) ranked WHERE ranked.id = project_board.id
			)
		`)
		return err

	case setting.Database.Type.IsPostgreSQL():
		// PostgreSQL: Use UPDATE FROM with subquery
		_, err := x.Exec(`
			UPDATE project_board pb SET sorting = ranked.new_sort
			FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY sorting, id) - 1 as new_sort
				FROM project_board
			) ranked
			WHERE pb.id = ranked.id
		`)
		return err

	case setting.Database.Type.IsMySQL():
		// MySQL: Use UPDATE with JOIN
		_, err := x.Exec(`
			UPDATE project_board pb
			INNER JOIN (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY sorting, id) - 1 as new_sort
				FROM project_board
			) ranked ON pb.id = ranked.id
			SET pb.sorting = ranked.new_sort
		`)
		return err

	default:
		return fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
	}
}

func enforceProjectIssueNotNull(x *xorm.Engine) error {
	switch {
	case setting.Database.Type.IsSQLite3():
		type ProjectIssue struct {
			ID              int64 `xorm:"pk autoincr"`
			IssueID         int64 `xorm:"INDEX NOT NULL unique(project_issue)"`
			ProjectID       int64 `xorm:"INDEX NOT NULL unique(project_issue)"`
			ProjectColumnID int64 `xorm:"'project_board_id' INDEX NOT NULL unique(column_sorting)"`
			Sorting         int64 `xorm:"NOT NULL DEFAULT 0 unique(column_sorting)"`
		}
		return base.RecreateTables(new(ProjectIssue))(x)

	case setting.Database.Type.IsPostgreSQL():
		for _, col := range []string{"issue_id", "project_id", "project_board_id"} {
			if _, err := x.Exec(fmt.Sprintf("ALTER TABLE project_issue ALTER COLUMN %s SET NOT NULL", col)); err != nil {
				return err
			}
		}
		return nil

	case setting.Database.Type.IsMySQL():
		for _, col := range []string{"issue_id", "project_id", "project_board_id"} {
			if _, err := x.Exec(fmt.Sprintf("ALTER TABLE project_issue MODIFY COLUMN %s BIGINT NOT NULL", col)); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
	}
}

// fixProjectIssueDuplicateAssignments removes duplicate (project_id, issue_id) rows,
// keeping only the row with the lowest id for each pair.
func fixProjectIssueDuplicateAssignments(x *xorm.Engine) error {
	switch {
	case setting.Database.Type.IsSQLite3():
		_, err := x.Exec(`
			DELETE FROM project_issue WHERE id NOT IN (
				SELECT MIN(id) FROM project_issue GROUP BY project_id, issue_id
			)
		`)
		return err

	case setting.Database.Type.IsPostgreSQL():
		_, err := x.Exec(`
			DELETE FROM project_issue pi USING project_issue pi2
			WHERE pi.project_id = pi2.project_id AND pi.issue_id = pi2.issue_id AND pi.id > pi2.id
		`)
		return err

	case setting.Database.Type.IsMySQL():
		_, err := x.Exec(`
			DELETE pi FROM project_issue pi
			INNER JOIN project_issue pi2
			ON pi.project_id = pi2.project_id AND pi.issue_id = pi2.issue_id AND pi.id > pi2.id
		`)
		return err

	default:
		return fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
	}
}
