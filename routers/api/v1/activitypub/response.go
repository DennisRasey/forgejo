// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
)

// Respond with a ActivityStreams Collection
func responseCollection(ctx *context.APIContext, iri string, listOptions db.ListOptions, items []string, count int64) {
	collection := ap.OrderedCollectionNew(ap.IRI(iri))
	collection.First = ap.IRI(iri + "?page=1")
	collection.TotalItems = uint(count)
	if listOptions.Page == 0 {
		response(ctx, collection)
		return
	}

	page := ap.OrderedCollectionPageNew(collection)
	page.ID = ap.IRI(fmt.Sprintf("%s?page=%d", iri, listOptions.Page))
	if listOptions.Page > 1 {
		page.Prev = ap.IRI(fmt.Sprintf("%s?page=%d", iri, listOptions.Page-1))
	}
	if listOptions.Page*listOptions.PageSize < int(count) {
		page.Next = ap.IRI(fmt.Sprintf("%s?page=%d", iri, listOptions.Page+1))
	}
	for _, item := range items {
		err := page.OrderedItems.Append(ap.IRI(item))
		if err != nil {
			ctx.ServerError("Append", err)
		}
	}

	response(ctx, page)
}

// Respond with an ActivityStreams object
func response(ctx *context.APIContext, v interface{}) {
	binary, err := jsonld.WithContext(
		jsonld.IRI(ap.ActivityBaseURI),
		jsonld.IRI(ap.SecurityContextURI),
		jsonld.IRI(forgefed.ForgeFedNamespaceURI),
	).Marshal(v)
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	ctx.Resp.Header().Add("Content-Type", activitypub.ActivityStreamsContentType)
	ctx.Resp.WriteHeader(http.StatusOK)
	if _, err = ctx.Resp.Write(binary); err != nil {
		log.Error("write to resp err: %v", err)
	}
}
