// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
)

func JobParser(workflow []byte, options ...jobparser.ParseOption) ([]*jobparser.SingleWorkflow, error) {
	singleWorkflows, err := jobparser.Parse(workflow, false, options...)
	if err != nil {
		return nil, err
	}
	nameToSingleWorkflows := make(map[string][]*jobparser.SingleWorkflow, len(singleWorkflows))
	duplicates := make(map[string]int, len(singleWorkflows))
	for _, singleWorkflow := range singleWorkflows {
		id, job := singleWorkflow.Job()
		nameToSingleWorkflows[job.Name] = append(nameToSingleWorkflows[job.Name], singleWorkflow)
		if len(nameToSingleWorkflows[job.Name]) > 1 {
			duplicates[job.Name]++
			job.Name = fmt.Sprintf("%s-%d", job.Name, duplicates[job.Name])
			if err := singleWorkflow.SetJob(id, job); err != nil {
				return nil, err
			}
		}
	}
	return singleWorkflows, nil
}
