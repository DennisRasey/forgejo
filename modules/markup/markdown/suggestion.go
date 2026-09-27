// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package markdown

import (
	"bytes"
	"strings"

	giteautil "forgejo.org/modules/util"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// suggestionLang is the info-string language that marks a fenced code block as a
// review "suggested change".
const suggestionLang = "suggestion"

// suggestionParser only locates fenced code blocks; it avoids the full
// SpecializedMarkdown pipeline whose AST transformers need a render context and
// would panic here. Block parsing is goldmark core, so it matches the renderer.
var suggestionParser = goldmark.New()

// isSuggestionLanguage reports (case-insensitively) whether a fence info-string marks a suggestion.
// Split on " ," to match the rendered language (goldmark splits on space, commas are stripped); a tab
// is excluded because the renderer keeps it and the sanitizer then drops the class (block stays plain).
func isSuggestionLanguage(info string) bool {
	info = strings.TrimSpace(info)
	lang := info
	if i := strings.IndexAny(info, " ,"); i >= 0 {
		lang = info[:i]
	}
	return strings.EqualFold(lang, suggestionLang)
}

// ExtractSuggestions returns the raw text of every ```suggestion fenced code block in content, in document order
func ExtractSuggestions(content string) []string {
	src := giteautil.NormalizeEOL([]byte(content))
	doc := suggestionParser.Parser().Parse(text.NewReader(src))

	var suggestions []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fcb, ok := n.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		if fcb.Info != nil && isSuggestionLanguage(string(fcb.Info.Segment.Value(src))) {
			suggestions = append(suggestions, fencedCodeBlockText(fcb, src))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return suggestions
}

// fencedCodeBlockText concatenates the raw text lines of a fenced code block.
func fencedCodeBlockText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(src))
	}
	return buf.String()
}
