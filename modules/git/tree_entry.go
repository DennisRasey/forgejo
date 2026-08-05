// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"forgejo.org/modules/log"
)

// TreeEntry the leaf in the git tree
type TreeEntry struct {
	ID ObjectID

	ptree *Tree

	entryMode EntryMode
	name      string

	size  int64
	sized bool
}

// Name returns the name of the entry
func (te *TreeEntry) Name() string {
	return te.name
}

// Mode returns the mode of the entry
func (te *TreeEntry) Mode() EntryMode {
	return te.entryMode
}

// Size returns the size of the entry
func (te *TreeEntry) Size() int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	if err := te.ptree.repo.WithCatFileBatchCheck(te.ptree.repo.Ctx, func(wr WriteCloserError, rd *bufio.Reader) error {
		_, err := wr.Write([]byte(te.ID.String() + "\n"))
		if err != nil {
			return err
		}
		_, _, te.size, err = ReadBatchLine(rd)
		return err
	}); err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), te.ptree.repo.Path, err)
		return 0
	}

	te.sized = true
	return te.size
}

// IsSubmodule if the entry is a submodule
func (te *TreeEntry) IsSubmodule() bool {
	return te.entryMode == EntryModeCommit
}

// IsDir if the entry is a sub dir
func (te *TreeEntry) IsDir() bool {
	return te.entryMode == EntryModeTree
}

// IsLink if the entry is a symlink
func (te *TreeEntry) IsLink() bool {
	return te.entryMode == EntryModeSymlink
}

// IsRegular if the entry is a regular file
func (te *TreeEntry) IsRegular() bool {
	return te.entryMode == EntryModeBlob
}

// IsExecutable if the entry is an executable file (not necessarily binary)
func (te *TreeEntry) IsExecutable() bool {
	return te.entryMode == EntryModeExec
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		ID:      te.ID,
		name:    te.Name(),
		size:    te.size,
		gotSize: te.sized,
		repo:    te.ptree.repo,
	}
}

// Type returns the type of the entry (commit, tree, blob)
func (te *TreeEntry) Type() string {
	switch te.Mode() {
	case EntryModeCommit:
		return "commit"
	case EntryModeTree:
		return "tree"
	default:
		return "blob"
	}
}

// LinkTarget returns the target of the symlink as string.
func (te *TreeEntry) LinkTarget() (string, error) {
	if !te.IsLink() {
		return "", ErrBadLink{te.Name(), "not a symlink"}
	}

	const symlinkLimit = 4096 // according to git config core.longpaths https://stackoverflow.com/a/22575737
	blob := te.Blob()
	if blob.Size() > symlinkLimit {
		return "", ErrBadLink{te.Name(), "symlink too large"}
	}

	rc, size, err := blob.NewTruncatedReader(symlinkLimit)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	buf := make([]byte, int(size))
	_, err = io.ReadFull(rc, buf)
	return string(buf), err
}

// FollowLink returns the entry pointed to by a symlink
func (te *TreeEntry) FollowLink() (*TreeEntry, error) {
	// read the link
	lnk, err := te.LinkTarget()
	if err != nil {
		return nil, err
	}

	t := te.ptree

	// traverse up directories
	for ; t != nil && strings.HasPrefix(lnk, "../"); lnk = lnk[3:] {
		t = t.ptree
	}

	if t == nil {
		return nil, ErrBadLink{te.Name(), "points outside of repo"}
	}

	target, err := t.GetTreeEntryByPath(lnk)
	if err != nil {
		if IsErrNotExist(err) {
			return nil, ErrBadLink{te.Name(), "broken link"}
		}
		return nil, err
	}
	return target, nil
}

// FollowLinks returns the entry ultimately pointed to by a symlink
func (te *TreeEntry) FollowLinks() (*TreeEntry, error) {
	if !te.IsLink() {
		return nil, ErrBadLink{te.Name(), "not a symlink"}
	}
	entry := te
	for range 999 {
		if entry.IsLink() {
			next, err := entry.FollowLink()
			if err != nil {
				return nil, err
			}
			if next.ID == entry.ID {
				return nil, ErrBadLink{
					entry.Name(),
					"recursive link",
				}
			}
			entry = next
		} else {
			break
		}
	}
	if entry.IsLink() {
		return nil, ErrBadLink{
			te.Name(),
			"too many levels of symbolic links",
		}
	}
	return entry, nil
}

// returns the Tree pointed to by this TreeEntry, or nil if this is not a tree
func (te *TreeEntry) Tree() *Tree {
	t, err := te.ptree.repo.getTree(te.ID)
	if err != nil {
		return nil
	}
	t.ptree = te.ptree
	return t
}

// returns the calulcated path within the tree of this TreeEntry, or an error if it can be determined
func (te *TreeEntry) Path() (string, error) {
	targetPath := te.Name()
	parentTree := te.ptree
	if parentTree == nil {
		return "", fmt.Errorf("couldn't find the parent tree of the entry")
	}

	prevID := parentTree.ID
	parentTree = parentTree.ptree
	for parentTree != nil {
		entries, err := parentTree.ListEntries()
		if err != nil {
			return "", fmt.Errorf("couldn't list entries: %v", err)
		}

		var matchingEntry *TreeEntry
		for _, entry := range entries {
			if entry.ID == prevID {
				matchingEntry = entry
				break
			}
		}

		if matchingEntry == nil {
			return "", fmt.Errorf("this shouldn't happen: couldn't find entry (ID: %s) in tree (ID: %s)", prevID, parentTree.ID)
		}

		targetPath = matchingEntry.name + "/" + targetPath
		prevID = parentTree.ID
		parentTree = parentTree.ptree
	}

	return targetPath, nil
}

// GetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
func (te *TreeEntry) GetSubJumpablePathName() string {
	if te.IsSubmodule() || !te.IsDir() {
		return ""
	}
	tree, err := te.ptree.SubTree(te.Name())
	if err != nil {
		return te.Name()
	}
	entries, _ := tree.ListEntries()
	if len(entries) == 1 && entries[0].IsDir() {
		name := entries[0].GetSubJumpablePathName()
		if name != "" {
			return te.Name() + "/" + name
		}
	}
	return te.Name()
}

// Entries a list of entry
type Entries []*TreeEntry

type customSortableEntries struct {
	Comparer func(s1, s2 string) bool
	Entries
}

var sorter = []func(t1, t2 *TreeEntry, cmp func(s1, s2 string) bool) bool{
	func(t1, t2 *TreeEntry, cmp func(s1, s2 string) bool) bool {
		return (t1.IsDir() || t1.IsSubmodule()) && !t2.IsDir() && !t2.IsSubmodule()
	},
	func(t1, t2 *TreeEntry, cmp func(s1, s2 string) bool) bool {
		return cmp(t1.Name(), t2.Name())
	},
}

func (ctes customSortableEntries) Len() int { return len(ctes.Entries) }

func (ctes customSortableEntries) Swap(i, j int) {
	ctes.Entries[i], ctes.Entries[j] = ctes.Entries[j], ctes.Entries[i]
}

func (ctes customSortableEntries) Less(i, j int) bool {
	t1, t2 := ctes.Entries[i], ctes.Entries[j]
	var k int
	for k = 0; k < len(sorter)-1; k++ {
		s := sorter[k]
		switch {
		case s(t1, t2, ctes.Comparer):
			return true
		case s(t2, t1, ctes.Comparer):
			return false
		}
	}
	return sorter[k](t1, t2, ctes.Comparer)
}

// Sort sort the list of entry
func (tes Entries) Sort() {
	sort.Sort(customSortableEntries{func(s1, s2 string) bool {
		return s1 < s2
	}, tes})
}

// CustomSort customizable string comparing sort entry list
func (tes Entries) CustomSort(cmp func(s1, s2 string) bool) {
	sort.Sort(customSortableEntries{cmp, tes})
}
