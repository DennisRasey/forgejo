// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

type IssueLockOption struct {
	Reason string `json:"reason" binding:"Required"`
}
