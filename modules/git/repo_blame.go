// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrBlameFileDoesNotExist   = errors.New("the blamed file does not exist")
	ErrBlameFileNotEnoughLines = errors.New("the blamed file has not enough lines")

	notEnoughLinesRe = regexp.MustCompile(`^fatal: file .+ has only \d+ lines?\n$`)
)

// LineBlame returns the latest commit at the given line
func (repo *Repository) LineBlame(revision, file string, line uint64) (*Commit, uint64, error) {
	res, _, gitErr := NewCommand(repo.Ctx, "blame").
		AddOptionFormat("-L %d,%d", line, line).
		AddOptionValues("-p", revision).
		AddDashesAndList(file).RunStdString(&RunOpts{Dir: repo.Path})
	if gitErr != nil {
		stdErr := gitErr.Stderr()

		if stdErr == fmt.Sprintf("fatal: no such path %s in %s\n", file, revision) {
			return nil, 0, ErrBlameFileDoesNotExist
		}
		if notEnoughLinesRe.MatchString(stdErr) {
			return nil, 0, ErrBlameFileNotEnoughLines
		}

		return nil, 0, gitErr
	}

	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, 0, err
	}

	objectIDLen := objectFormat.FullLength()

	if len(res) < objectIDLen+1 {
		return nil, 0, fmt.Errorf("output of blame is invalid, cannot contain commit ID: %s", res)
	}

	commit, err := repo.GetCommit(res[:objectIDLen])
	if err != nil {
		return nil, 0, fmt.Errorf("GetCommit: %w", err)
	}

	endIdxOriginalLineNo := strings.IndexRune(res[objectIDLen+1:], ' ')
	if endIdxOriginalLineNo == -1 {
		return nil, 0, fmt.Errorf("output of blame is invalid, cannot contain original line number: %s", res)
	}

	originalLineNo, err := strconv.ParseUint(res[objectIDLen+1:objectIDLen+1+endIdxOriginalLineNo], 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("strconv.ParseUint: %w", err)
	}

	return commit, originalLineNo, nil
}

type ReverseLineBlame struct {
	CommitID   string
	LineNumber uint64
	FilePath   string
}

// Reverses the effect of [LineBlame]. If a file was modified at originalLine number in originalRevision,
// ReverseLineBlame will identify the last commit up-to-and-including currentRevision where that line exists, including
// its new path and line number. If the returned commit is not the same as currentRevision, then it indicates that
// content can no longer be located in currentRevision, and the returned commit is the last commit that had it.
func (repo *Repository) ReverseLineBlame(originalRevision, file string, originalLine uint64, currentRevision string) (*ReverseLineBlame, error) {
	if originalRevision == currentRevision {
		// Would cause an error to run the reverse, "fatal: More than one commit to dig up from, (N) and (N)"
		return &ReverseLineBlame{
			CommitID:   originalRevision,
			LineNumber: originalLine,
			FilePath:   file,
		}, nil
	}

	res, _, gitErr := NewCommand(repo.Ctx, "blame").
		AddOptionValues("--reverse").
		AddDynamicArguments(fmt.Sprintf("%s..%s", originalRevision, currentRevision)).
		AddOptionFormat("-L %d,%d", originalLine, originalLine).
		AddOptionValues("-p").
		AddDashesAndList(file).RunStdString(&RunOpts{Dir: repo.Path})
	if gitErr != nil {
		return nil, gitErr
	}

	// Example output:
	//
	// 74be0e8aa338d1374ab7ca0a25a4f594955a69c2 16 9 1
	// author FirstName LastName
	// author-mail <author@example.org>
	// author-time 1775492007
	// author-tz -0600
	// committer FirstName LastName
	// committer-mail <author@example.org>
	// committer-time 1775492007
	// committer-tz -0600
	// summary restore file-in-base to orig, now not present in diff
	// filename README.md
	//
	// Header (https://git-scm.com/docs/git-blame#_the_porcelain_format):
	// - 40-byte SHA-1 of the commit the line is attributed to;
	// - the line number of the line in the original file; [note: opposite in reverse]
	// - the line number of the line in the final file; [note: opposite in reverse]
	// - on a line that starts a group of lines from a different commit than the previous one, the number of lines in
	//   this group. On subsequent lines this field is absent.

	lines := strings.Split(res, "\n")

	header := lines[0]
	headerValues := strings.Split(header, " ")
	if len(headerValues) < 2 {
		return nil, fmt.Errorf("failed to parse blame --reverse header: %q", header)
	}

	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	objectIDLen := objectFormat.FullLength()
	objectID := headerValues[0]
	if len(objectID) != objectIDLen {
		return nil, fmt.Errorf("output of blame is invalid, cannot contain commit ID: %s", objectID)
	}
	commit, err := repo.GetCommit(objectID)
	if err != nil {
		return nil, fmt.Errorf("GetCommit: %w", err)
	}

	currentLineStr := headerValues[1]
	currentLineNo, err := strconv.ParseUint(currentLineStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseUint: %w", err)
	}

	var filename string
	for _, otherLine := range lines {
		if strings.HasPrefix(otherLine, "filename ") {
			filename = otherLine[len("filename "):]
			break
		}
	}

	return &ReverseLineBlame{
		CommitID:   commit.ID.String(),
		LineNumber: currentLineNo,
		FilePath:   filename,
	}, nil
}
