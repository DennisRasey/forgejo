// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
	"unicode/utf8"
)

// in UTF8 "…" is 3 bytes so doesn't really gain us anything...
const (
	utf8Ellipsis  = "…"
	asciiEllipsis = "..."
)

// SplitStringAtByteN splits a string at byte n accounting for rune boundaries. (Combining characters are not accounted for.)
func SplitStringAtByteN(input string, n int) (left, right string) {
	if len(input) <= n {
		return input, ""
	}

	if !utf8.ValidString(input) {
		if n-3 < 0 {
			return input, ""
		}
		return input[:n-3] + asciiEllipsis, asciiEllipsis + input[n-3:]
	}

	end := 0
	for end <= n-3 {
		_, size := utf8.DecodeRuneInString(input[end:])
		if end+size > n-3 {
			break
		}
		end += size
	}

	return input[:end] + utf8Ellipsis, utf8Ellipsis + input[end:]
}

// SplitTrimSpace splits the string at given separator and trims leading and trailing space
func SplitTrimSpace(input, sep string) []string {
	// Trim initial leading & trailing space
	input = strings.TrimSpace(input)
	// replace CRLF with LF
	input = strings.ReplaceAll(input, "\r\n", "\n")

	var stringList []string
	for s := range strings.SplitSeq(input, sep) {
		// trim leading and trailing space
		stringList = append(stringList, strings.TrimSpace(s))
	}

	return stringList
}

// TruncateRunes returns a truncated string with given rune limit,
// it returns input string if its rune length doesn't exceed the limit.
func TruncateRunes(str string, limit int) string {
	if utf8.RuneCountInString(str) < limit {
		return str
	}
	return string([]rune(str)[:limit])
}

// TruncateStringAtWordBoundary truncates input to at most n bytes - aware of utf8 runes of course -
// If the truncation point is inside a word and a space is close by the truncation happens at that word boundary.
func TruncateStringAtWordBoundary(input string, n int) string {
	truncated, right := SplitStringAtByteN(input, n)
	if right == "" {
		return truncated
	}
	// in case the content is in a Latin family language, we remove the last broken word.
	if lastSpaceIdx := strings.LastIndex(truncated, " "); lastSpaceIdx != -1 && len(truncated)-lastSpaceIdx < 15 {
		truncated = truncated[:lastSpaceIdx] + utf8Ellipsis
	}
	return truncated
}
