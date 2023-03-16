// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"context"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/forgefed"

	ap "github.com/go-ap/activitypub"
)

// Construct a Note object from a comment
func Note(ctx context.Context, comment *issues_model.Comment) (*ap.Note, error) {
	err := comment.LoadPoster(ctx)
	if err != nil {
		return nil, err
	}
	err = comment.LoadIssue(ctx)
	if err != nil {
		return nil, err
	}
	note := ap.Note{
		Type:         ap.NoteType,
		ID:           ap.IRI(comment.GetIRI(ctx)),
		AttributedTo: ap.IRI(comment.Poster.GetIRI()),
		Context:      ap.IRI(comment.Issue.GetIRI(ctx)),
		To:           ap.ItemCollection{ap.IRI("https://www.w3.org/ns/activitystreams#Public")},
	}
	note.Content = ap.NaturalLanguageValuesNew()
	err = note.Content.Set("en", ap.Content(comment.Content))
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// Construct a Ticket object from an issue
func Ticket(ctx context.Context, issue *issues_model.Issue) (*forgefed.Ticket, error) {
	iri := issue.GetIRI(ctx)
	ticket := forgefed.TicketNew()
	ticket.Type = forgefed.TicketType
	ticket.ID = ap.IRI(iri)

	// Setting a NaturalLanguageValue to a number causes go-ap's JSON parsing to do weird things
	// Workaround: set it to #1 instead of 1
	ticket.Name = ap.NaturalLanguageValuesNew()
	err := ticket.Name.Set("en", ap.Content("#"+strconv.FormatInt(issue.Index, 10)))
	if err != nil {
		return nil, err
	}

	err = issue.LoadRepo(ctx)
	if err != nil {
		return nil, err
	}
	ticket.Context = ap.IRI(issue.Repo.GetIRI())

	err = issue.LoadPoster(ctx)
	if err != nil {
		return nil, err
	}
	ticket.AttributedTo = ap.IRI(issue.Poster.GetIRI())

	ticket.Summary = ap.NaturalLanguageValuesNew()
	err = ticket.Summary.Set("en", ap.Content(issue.Title))
	if err != nil {
		return nil, err
	}

	ticket.Content = ap.NaturalLanguageValuesNew()
	err = ticket.Content.Set("en", ap.Content(issue.Content))
	if err != nil {
		return nil, err
	}

	if issue.IsClosed {
		ticket.IsResolved = true
	}
	return ticket, nil
}
