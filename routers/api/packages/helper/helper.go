// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helper

import (
	"fmt"
	"net/http"
	"net/url"

	packages_model "forgejo.org/models/packages"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
	packages_service "forgejo.org/services/packages"
)

// LogAndProcessError logs an error and calls a custom callback with the processed error message.
// If the error is an InternalServerError the message is stripped if the user is not an admin.
func LogAndProcessError(ctx *context.Context, status int, obj any, cb func(string)) {
	var message string
	if err, ok := obj.(error); ok {
		message = err.Error()
	} else if obj != nil {
		message = fmt.Sprintf("%s", obj)
	}
	if status == http.StatusInternalServerError {
		// LogAndProcessError is always wrapped in a `apiError` call, so we need to skip two frames
		log.ErrorWithSkip(2, message)

		if setting.IsProd && (ctx.Doer == nil || !ctx.IsUserSiteAdmin()) {
			message = ""
		}
	} else {
		log.Debug(message)
	}

	if cb != nil {
		cb(message)
	}
}

// ServePackageFile Serves the content of the package file
// If the url is set it will redirect the request, otherwise the content is copied to the response.
func ServePackageFile(ctx *context.Context, s packages_service.ReadSeekCloseSizer, u *url.URL, pf *packages_model.PackageFile, forceOpts ...*context.ServeHeaderOptions) {
	var opts *context.ServeHeaderOptions
	if len(forceOpts) > 0 {
		opts = forceOpts[0]
	} else {
		opts = &context.ServeHeaderOptions{
			Filename:     pf.Name,
			LastModified: pf.CreatedUnix.AsLocalTime(),
		}
	}

	if u != nil {
		ctx.Redirect(u.String(), opts.RedirectStatusCode)
		return
	}

	defer s.Close()

	if opts.ContentLength == nil {
		opts.ContentLength = new(s.Size())
	}
	ctx.ServeContent(s, opts)
}
