// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testDiscoveredInfo struct{}

func (s *testDiscoveredInfo) ClaimedID() string {
	return "claimedID"
}

func (s *testDiscoveredInfo) OpEndpoint() string {
	return "opEndpoint"
}

func (s *testDiscoveredInfo) OpLocalID() string {
	return "opLocalID"
}

func TestTimedDiscoveryCache(t *testing.T) {
	dc := newTimedDiscoveryCache(100 * time.Millisecond)

	// Put some initial values
	dc.Put("foo", &testDiscoveredInfo{}) // openid.opEndpoint: "a", openid.opLocalID: "b", openid.claimedID: "c"})

	// Make sure we can retrieve them
	if di := dc.Get("foo"); di == nil {
		t.Error("Expected a result, got nil")
	} else if di.OpEndpoint() != "opEndpoint" || di.OpLocalID() != "opLocalID" || di.ClaimedID() != "claimedID" {
		t.Errorf("Expected opEndpoint opLocalID claimedID, got %v %v %v", di.OpEndpoint(), di.OpLocalID(), di.ClaimedID())
	}

	// Attempt to get a non-existent value
	di := dc.Get("bar")
	assert.Nil(t, di)

	// Sleep 100 ms and try retrieve again
	time.Sleep(100 * time.Millisecond)

	di = dc.Get("foo")
	assert.Nil(t, di)
}
