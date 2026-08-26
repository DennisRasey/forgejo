// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package funding

import (
	"strings"

	"go.yaml.in/yaml/v3"
)

// CouldBeFundingConfig returns true if the given path could be that of a
// repository funding config document.
func CouldBeFundingConfig(path string) bool {
	for _, name := range fundingCandidates {
		if strings.EqualFold(path, name) {
			return true
		}
	}
	return false
}

// Represents a key-value pair in a FUNDING.yml document. The value may be a
// list containing one or more strings.
type rawRepoFundingConfigEntry struct {
	Key   string
	Value any
}

// A funding config consists of unique key-value pairs, considered in the order
// they appear in the document. Each key corresponds to exactly one value,
// which should be either a bare string or a list of strings.
type rawRepoFundingConfig []rawRepoFundingConfigEntry

// called by `yaml.Unmarshal` when decoding file data
func (c *rawRepoFundingConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		tag := strings.TrimPrefix(value.ShortTag(), "!!")
		return &NotYAMLMappingError{ShortTag: tag}
	}

	// in a mapping, Content contains pairs of nodes, so we iterate two at a time
	for i := 0; i < len(value.Content); i += 2 {
		// peeking ahead at the value, store it if it's there
		var entryData any
		if err := value.Content[i+1].Decode(&entryData); err != nil {
			return err // not sure what we hit that won't fit into `any`, but it wasn't good :/
		}

		// since we have a value, grab the key
		key := value.Content[i].Value
		for _, alreadyEntry := range *c {
			if alreadyEntry.Key == key {
				return &DuplicateYAMLKeyError{Key: key}
			}
		}

		// record the pair
		*c = append(*c, rawRepoFundingConfigEntry{Key: key, Value: entryData})
	}

	return nil
}
