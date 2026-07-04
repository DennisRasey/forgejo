// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"context"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/models/user"
	"forgejo.org/modules/optional"

	_ "forgejo.org/modules/testimport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testIssueQueryStringOpt struct {
	Keyword string
	Results []Token
}

var testOpts = []testIssueQueryStringOpt{
	{
		Keyword: "Hello",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "Hello World",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "Hello  World",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: " Hello World ",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "+Hello +World",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "+Hello World",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptShould,
			},
		},
	},
	{
		Keyword: "+Hello -World",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptNot,
			},
		},
	},
	{
		Keyword: "Hello -World",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptShould,
			},
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptNot,
			},
		},
	},
	{
		Keyword: "\"Hello World\"",
		Results: []Token{
			{
				Term:  "Hello World",
				Fuzzy: false,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "+\"Hello World\" Hello",
		Results: []Token{
			{
				Term:  "Hello World",
				Fuzzy: false,
				Kind:  BoolOptMust,
			},
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptShould,
			},
		},
	},
	{
		Keyword: "-\"Hello World\"",
		Results: []Token{
			{
				Term:  "Hello World",
				Fuzzy: false,
				Kind:  BoolOptNot,
			},
		},
	},
	{
		Keyword: "\"+Hello -World\"",
		Results: []Token{
			{
				Term:  "+Hello -World",
				Fuzzy: false,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "\\+Hello", // \+Hello => +Hello
		Results: []Token{
			{
				Term:  "+Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "\\\\Hello", // \\Hello => \Hello
		Results: []Token{
			{
				Term:  "\\Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "\\\"Hello", // \"Hello => "Hello
		Results: []Token{
			{
				Term:  "\"Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "\\",
		Results: nil,
	},
	{
		Keyword: "\"",
		Results: nil,
	},
	{
		Keyword: "Hello \\",
		Results: []Token{
			{
				Term:  "Hello",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "\"\"",
		Results: nil,
	},
	{
		Keyword: "\" World \"",
		Results: []Token{
			{
				Term:  " World ",
				Fuzzy: false,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "\"\" World \"\"",
		Results: []Token{
			{
				Term:  "World",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
	{
		Keyword: "Best \"Hello World\" Ever",
		Results: []Token{
			{
				Term:  "Best",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
			{
				Term:  "Hello World",
				Fuzzy: false,
				Kind:  BoolOptMust,
			},
			{
				Term:  "Ever",
				Fuzzy: true,
				Kind:  BoolOptMust,
			},
		},
	},
}

func TestIssueQueryString(t *testing.T) {
	ctx := t.Context()
	for _, res := range testOpts {
		t.Run(res.Keyword, func(t *testing.T) {
			opt := &SearchOptions{}
			require.NoError(t, opt.WithKeyword(ctx, res.Keyword))
			assert.Equal(t, res.Results, opt.Tokens, "failed for keyword `%s`", res.Keyword)
		})
	}
}

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestIssueQueryStringWithFilters(t *testing.T) {
	// we don't need all the fixures
	// insert only one single test user
	require.NoError(t, user.CreateUser(t.Context(), &user.User{
		ID:        2,
		Name:      "test",
		LowerName: "test",
		Email:     "test@localhost",
	}))

	for _, c := range []struct {
		Keyword string
		Opts    *SearchOptions
	}{
		// Generic Cases
		{
			Keyword: "modified:>2025-08-28",
			Opts: &SearchOptions{
				UpdatedAfterUnix: optional.Some(int64(1756339200)),
			},
		},
		{
			Keyword: "modified:<2025-08-28",
			Opts: &SearchOptions{
				UpdatedBeforeUnix: optional.Some(int64(1756339200)),
			},
		},
		{
			Keyword: "modified:>2025-08-28 modified:<2025-08-28",
			Opts: &SearchOptions{
				UpdatedAfterUnix:  optional.Some(int64(1756339200)),
				UpdatedBeforeUnix: optional.Some(int64(1756339200)),
			},
		},
		{
			Keyword: "modified:2025-08-28",
			Opts: &SearchOptions{
				UpdatedAfterUnix:  optional.Some(int64(1756339200)),
				UpdatedBeforeUnix: optional.Some(int64(1756339200)),
			},
		},
		{
			Keyword: "assignee:test",
			Opts: &SearchOptions{
				AssigneeID: optional.Some(int64(2)),
			},
		},
		{
			Keyword: "assignee:test hi",
			Opts: &SearchOptions{
				AssigneeID: optional.Some(int64(2)),
				Tokens: []Token{
					{
						Term:  "hi",
						Kind:  BoolOptMust,
						Fuzzy: true,
					},
				},
			},
		},
		{
			Keyword: "mentions:test",
			Opts: &SearchOptions{
				MentionID: optional.Some(int64(2)),
			},
		},
		{
			Keyword: "review:test",
			Opts: &SearchOptions{
				ReviewedID: optional.Some(int64(2)),
			},
		},
		{
			Keyword: "author:test",
			Opts: &SearchOptions{
				PosterID: optional.Some(int64(2)),
			},
		},
		{
			Keyword: "sort:updated:asc",
			Opts: &SearchOptions{
				SortBy: SortByUpdatedAsc,
			},
		},
		{
			Keyword: "sort:test",
			Opts: &SearchOptions{
				SortBy: SortByScore,
			},
		},
		{
			Keyword: "test author:test mentions:test modified:<2025-08-28 sort:comments:desc",
			Opts: &SearchOptions{
				Tokens: []Token{
					{
						Term:  "test",
						Kind:  BoolOptMust,
						Fuzzy: true,
					},
				},
				MentionID:         optional.Some(int64(2)),
				PosterID:          optional.Some(int64(2)),
				UpdatedBeforeUnix: optional.Some(int64(1756339200)),
				SortBy:            SortByCommentsDesc,
			},
		},

		// Edge Cases
		{
			Keyword: "author:",
			Opts: &SearchOptions{
				Tokens: []Token{
					{
						Term:  "author:",
						Kind:  BoolOptMust,
						Fuzzy: true,
					},
				},
			},
		},
		{
			Keyword: "author:testt",
			Opts:    &SearchOptions{},
		},
		{
			Keyword: "author: test",
			Opts: &SearchOptions{
				Tokens: []Token{
					{
						Term:  "author:",
						Kind:  BoolOptMust,
						Fuzzy: true,
					},
					{
						Term:  "test",
						Kind:  BoolOptMust,
						Fuzzy: true,
					},
				},
			},
		},
		{
			Keyword: "modified:",
			Opts: &SearchOptions{
				Tokens: []Token{
					{
						Term:  "modified:",
						Kind:  BoolOptMust,
						Fuzzy: true,
					},
				},
			},
		},
	} {
		t.Run(c.Keyword, func(t *testing.T) {
			opts := &SearchOptions{}
			require.NoError(t, opts.WithKeyword(context.Background(), c.Keyword))
			assert.Equal(t, c.Opts, opts)
		})
	}
}

func TestToken_ParseIssueReference(t *testing.T) {
	var tk Token
	{
		tk.Term = "123"
		id, err := tk.ParseIssueReference()
		require.NoError(t, err)
		assert.Equal(t, int64(123), id)
	}
	{
		tk.Term = "#123"
		id, err := tk.ParseIssueReference()
		require.NoError(t, err)
		assert.Equal(t, int64(123), id)
	}
	{
		tk.Term = "!123"
		id, err := tk.ParseIssueReference()
		require.NoError(t, err)
		assert.Equal(t, int64(123), id)
	}
	{
		tk.Term = "text"
		_, err := tk.ParseIssueReference()
		require.Error(t, err)
	}
	{
		tk.Term = ""
		_, err := tk.ParseIssueReference()
		require.Error(t, err)
	}
}
