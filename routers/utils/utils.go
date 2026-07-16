// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"html"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
	"forgejo.org/services/gitdiff"
)

// SanitizeFlashErrorString will sanitize a flash error string
func SanitizeFlashErrorString(x string) string {
	return strings.ReplaceAll(html.EscapeString(x), "\n", "<br>")
}

func PaginateDiffFiles(ctx *context.Context, diffFileMetadata []*gitdiff.DiffFileMetadata) (*context.Pagination, []string) {
	page := max(ctx.FormInt("diff-page"), 1)
	totalFiles := len(diffFileMetadata)
	pager := context.NewPagination(totalFiles, setting.UI.DiffPagingNum, page, 5)

	listOpts := db.ListOptions{
		Page:     page,
		PageSize: setting.UI.DiffPagingNum,
	}

	pagedFiles := gitdiff.GetDiffFilePage(diffFileMetadata, listOpts.Page, listOpts.PageSize, totalFiles)

	return pager, pagedFiles
}
