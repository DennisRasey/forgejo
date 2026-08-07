// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "net/http"

type UserAgentTransport struct {
	*http.Transport
}

func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Forgejo-migration-http-client/1.1")
	return t.Transport.RoundTrip(req)
}
