// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"

	"forgejo.org/modules/container"

	"github.com/stretchr/testify/assert"
)

func TestActionJobList_GetJobIDs(t *testing.T) {
	jobs := ActionJobList{
		&ActionRunJob{JobNamespace: "ns1", JobID: "job 1"},
		&ActionRunJob{JobNamespace: "ns1", JobID: "job 2"},
		&ActionRunJob{JobNamespace: "ns2", JobID: "job 1"},
		&ActionRunJob{JobNamespace: "ns2", JobID: "job 2"},
	}

	assert.Equal(t, container.SetOf(
		NamespacedJobIdentifier{Namespace: "ns1", Identifier: "job 2"},
		NamespacedJobIdentifier{Namespace: "ns1", Identifier: "job 1"},
		NamespacedJobIdentifier{Namespace: "ns2", Identifier: "job 2"},
		NamespacedJobIdentifier{Namespace: "ns2", Identifier: "job 1"},
	), jobs.GetJobIDs())
}
