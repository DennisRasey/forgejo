// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path"
	"strings"
)

func (t *Tree) getTreeEntryByPathWithEqualName(relpath string, namesEqual func(a, b string) bool) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ptree:     t,
			ID:        t.ID,
			name:      "",
			entryMode: EntryModeTree,
		}, nil
	}

	// FIXME: This should probably use git cat-file --batch to be a bit more efficient
	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")
	var err error
	tree := t
	for i, name := range parts {
		if i == len(parts)-1 {
			entries, err := tree.ListEntries()
			if err != nil {
				return nil, err
			}
			for _, v := range entries {
				if namesEqual(v.Name(), name) {
					return v, nil
				}
			}
		} else {
			tree, err = tree.SubTree(name)
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, ErrNotExist{"", relpath}
}

func stringsEqual(a, b string) bool {
	return a == b
}

// GetTreeEntryByPath get the tree entries according to the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
	return t.getTreeEntryByPathWithEqualName(relpath, stringsEqual)
}

// GetTreeEntryByFoldedPath get the tree entries according to the sub dir,
// regardless of the case of relpath. If there are multiple files with the same
// (case-insensitive) name, the first one found is returned.
func (t *Tree) GetTreeEntryByFoldedPath(relpath string) (*TreeEntry, error) {
	return t.getTreeEntryByPathWithEqualName(relpath, strings.EqualFold)
}

// GetBlobByPath get the blob object according the path
func (t *Tree) GetBlobByPath(relpath string) (*Blob, error) {
	entry, err := t.GetTreeEntryByPath(relpath)
	if err != nil {
		return nil, err
	}

	if !entry.IsDir() && !entry.IsSubmodule() {
		return entry.Blob(), nil
	}

	return nil, ErrNotExist{"", relpath}
}

// GetBlobByFoldedPath returns the blob object at relpath, regardless of the
// case of relpath. If there are multiple files with the same case-insensitive
// name, the first one found will be returned.
func (t *Tree) GetBlobByFoldedPath(relpath string) (*Blob, error) {
	entries, err := t.ListEntries()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), relpath) {
			return t.GetBlobByPath(entry.Name())
		}
	}

	return nil, ErrNotExist{"", relpath}
}
