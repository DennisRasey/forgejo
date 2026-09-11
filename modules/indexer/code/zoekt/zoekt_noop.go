// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !unix || openbsd

package zoekt

import (
	"context"

	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/indexer/code/internal"
	indexer_internal "forgejo.org/modules/indexer/internal"
	inner_zoekt "forgejo.org/modules/indexer/internal/zoekt"
)

type Indexer struct {
	indexer_internal.Indexer // do not composite inner_zoekt.Indexer directly to avoid exposing too much
	inner                    *inner_zoekt.Indexer
}

func NewIndexer(_ string) *Indexer {
	idxer := inner_zoekt.NewIndexer()
	return &Indexer{
		Indexer: idxer,
		inner:   idxer,
	}
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	return inner_zoekt.ErrNotImplemented
}

// Delete entries by repoId
func (b *Indexer) Delete(_ context.Context, repoID int64) error {
	return inner_zoekt.ErrNotImplemented
}

func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	return 0, nil, nil, inner_zoekt.ErrNotImplemented
}

func (b *Indexer) Formatter() internal.ResultFormatter {
	return nil
}
