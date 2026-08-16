// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2025 The Forgejo Authors.
// SPDX-License-Identifier: MIT

package markup

import (
	"bytes"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"forgejo.org/modules/base"
	"forgejo.org/modules/emoji"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup/common"
	"forgejo.org/modules/references"
	"forgejo.org/modules/regexplru"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/templates/vars"
	"forgejo.org/modules/translation"
	"forgejo.org/modules/util"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"mvdan.cc/xurls/v2"
)

// Issue name styles
const (
	IssueNameStyleNumeric      = "numeric"
	IssueNameStyleAlphanumeric = "alphanumeric"
	IssueNameStyleRegexp       = "regexp"
)

var (
	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the linked entity does not exist.
	// While fast, this is also incorrect and lead to false positives.
	// TODO: fix invalid linking issue

	// valid chars in encoded path and parameter: [-+~_%.a-zA-Z0-9/]

	// httpSchemePattern matches https:// or http://
	httpSchemePattern = regexp.MustCompile(`^https?://`)

	// hashCurrentPattern matches string that represents a commit SHA, e.g. d8a994ef243349f321568f9e36d5c3f444b99cae
	// Although SHA1 hashes are 40 chars long, SHA256 are 64, the regex matches the hash from 7 to 64 chars in length
	// so that abbreviated hash links can be used as well. This matches git and GitHub usability.
	hashCurrentPattern = regexp.MustCompile(`(?:^|\s)[^\w\d]*([0-9a-f]{7,64})[^\w\d]*(?:\s|$)`)

	// shortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	shortLinkPattern = regexp.MustCompile(`\[\[(.*?)\]\](\w*)`)

	// anyHashPattern splits url containing SHA into parts
	anyHashPattern = regexp.MustCompile(`https?://[^\s/]+/(\S+/(?:commit|tree|blob))/([0-9a-f]{7,64})(/[-+~_%.a-zA-Z0-9/]+)?(\?[-+~_%\.a-zA-Z0-9=&]+)?(#[-+~_%.a-zA-Z0-9]+)?`)

	// comparePattern matches "http://domain/org/repo/compare/COMMIT1...COMMIT2#hash"
	comparePattern = regexp.MustCompile(`https?://[^\s/]+/(?:\S+/)?([^\s/]+/[^\s/]+)/compare/([0-9a-f]{7,64})(\.\.\.?)([0-9a-f]{7,64})?(\?[-+~_%\.a-zA-Z0-9=&/]+)?(#[-+~_%.a-zA-Z0-9]+)?`)

	// pullReviewCommitPattern matches "https://domain.tld/<subpath...>/<owner>/<repo>/pulls/<id>/commits/<sha>"
	pullReviewCommitPattern = regexp.MustCompile(`https?://[^\s/]+/(?:\S+/)?([^\s/]+/[^\s/]+)/pulls/(\d+)/commits/([0-9a-f]{7,64})(#[-+~_%.a-zA-Z0-9]+)?`)

	validLinksPattern = regexp.MustCompile(`^[a-z][\w-]+://`)

	// While this email regex is definitely not perfect and I'm sure you can come up
	// with edge cases, it is still accepted by the CommonMark specification, as
	// well as the HTML5 spec:
	//   http://spec.commonmark.org/0.28/#email-address
	//   https://html.spec.whatwg.org/multipage/input.html#e-mail-state-(type%3Demail)
	emailRegex = regexp.MustCompile("(?:\\s|^|\\(|\\[)([a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9]{2,}(?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+)(?:\\s|$|\\)|\\]|;|,|\\?|!|\\.(\\s|$))")

	// Fediverse handle regex (same as emailRegex but with additional @ or !
	// at start)
	fediRegex = regexp.MustCompile("(?:\\s|^|\\(|\\[)([@!]([a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+)@([a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9]{2,}(?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+))(?:\\s|$|\\)|\\]|;|,|\\?|!|\\.(\\s|$))")

	// blackfriday extensions create IDs like fn:user-content-footnote
	blackfridayExtRegex = regexp.MustCompile(`[^:]*:user-content-`)

	// EmojiShortCodeRegex find emoji by alias like :smile:
	EmojiShortCodeRegex = regexp.MustCompile(`:[-+\w]+:`)

	InlineCodeBlockRegex = regexp.MustCompile("`[^`]+`")
)

// CSS class for action keywords (e.g. "closes: #1")
const keywordClass = "issue-keyword"

// IsLink reports whether link fits valid format.
func IsLink(link []byte) bool {
	return validLinksPattern.Match(link)
}

func IsLinkStr(link string) bool {
	return validLinksPattern.MatchString(link)
}

// regexp for full links to issues/pulls
var issueFullPattern *regexp.Regexp

// Once for to prevent races
var issueFullPatternOnce sync.Once

func getIssueFullPattern() *regexp.Regexp {
	issueFullPatternOnce.Do(func() {
		// example: https://domain/org/repo/pulls/27#hash
		issueFullPattern = regexp.MustCompile(regexp.QuoteMeta(setting.AppURL) +
			`(?P<user>[\w_.-]+)\/(?P<repo>[\w_.-]+)\/(?:issues|pulls)\/(?P<num>(?:\w{1,10}-)?[1-9][0-9]*)(?P<subpath>\/[\w_.-]+)?(?:(?P<comment>#(?:issue|issuecomment)-\d+)|(?:[\?#](?:\S+)?))?\b`)
	})
	return issueFullPattern
}

// CustomLinkURLSchemes allows for additional schemes to be detected when parsing links within text
func CustomLinkURLSchemes(schemes []string) {
	schemes = append(schemes, "http", "https")
	withAuth := make([]string, 0, len(schemes))
	validScheme := regexp.MustCompile(`^[a-z]+$`)
	for _, s := range schemes {
		if !validScheme.MatchString(s) {
			continue
		}
		without := slices.Contains(xurls.SchemesNoAuthority, s)
		if without {
			s += ":"
		} else {
			s += "://"
		}
		withAuth = append(withAuth, s)
	}
	common.LinkRegex, _ = xurls.StrictMatchingScheme(strings.Join(withAuth, "|"))
}

type postProcessError struct {
	context string
	err     error
}

func (p *postProcessError) Error() string {
	return "PostProcess: " + p.context + ", " + p.err.Error()
}

type processor func(ctx *RenderContext, node *html.Node)

var defaultProcessors = []processor{
	pullReviewCommitPatternProcessor,
	fullIssuePatternProcessor,
	comparePatternProcessor,
	filePreviewPatternProcessor,
	fullHashPatternProcessor,
	shortLinkProcessor,
	linkProcessor,
	mentionProcessor,
	issueIndexPatternProcessor,
	commitCrossReferencePatternProcessor,
	hashCurrentPatternProcessor,
	fediAddressProcessor,
	emailAddressProcessor,
	emojiProcessor,
	emojiShortCodeProcessor,
}

// PostProcess does the final required transformations to the passed raw HTML
// data, and ensures its validity. Transformations include: replacing links and
// emails with HTML links, parsing shortlinks in the format of [[Link]], like
// MediaWiki, linking issues in the format #ID, and mentions in the format
// @user, and others.
func PostProcess(
	ctx *RenderContext,
	input io.Reader,
	output io.Writer,
) error {
	return postProcess(ctx, defaultProcessors, input, output)
}

var commitMessageProcessors = []processor{
	pullReviewCommitPatternProcessor,
	fullIssuePatternProcessor,
	comparePatternProcessor,
	fullHashPatternProcessor,
	linkProcessor,
	mentionProcessor,
	issueIndexPatternProcessor,
	commitCrossReferencePatternProcessor,
	hashCurrentPatternProcessor,
	emailAddressProcessor,
	emojiProcessor,
	emojiShortCodeProcessor,
}

// RenderCommitMessage will use the same logic as PostProcess, but will disable
// the shortLinkProcessor and will add a defaultLinkProcessor if defaultLink is
// set, which changes every text node into a link to the passed default link.
func RenderCommitMessage(
	ctx *RenderContext,
	content string,
) (string, error) {
	procs := commitMessageProcessors
	if ctx.DefaultLink != "" {
		// we don't have to fear data races, because being
		// commitMessageProcessors of fixed len and cap, every time we append
		// something to it the slice is realloc+copied, so append always
		// generates the slice ex-novo.
		procs = append(procs, genDefaultLinkProcessor(ctx.DefaultLink))
	}
	return renderProcessString(ctx, procs, content)
}

var commitMessageSubjectProcessors = []processor{
	pullReviewCommitPatternProcessor,
	fullIssuePatternProcessor,
	comparePatternProcessor,
	fullHashPatternProcessor,
	linkProcessor,
	mentionProcessor,
	issueIndexPatternProcessor,
	commitCrossReferencePatternProcessor,
	hashCurrentPatternProcessor,
	emojiShortCodeProcessor,
	emojiProcessor,
}

var emojiProcessors = []processor{
	emojiShortCodeProcessor,
	emojiProcessor,
}

// RenderCommitMessageSubject will use the same logic as PostProcess and
// RenderCommitMessage, but will disable the shortLinkProcessor and
// emailAddressProcessor, will add a defaultLinkProcessor if defaultLink is set,
// which changes every text node into a link to the passed default link.
func RenderCommitMessageSubject(
	ctx *RenderContext,
	content string,
) (string, error) {
	procs := commitMessageSubjectProcessors
	if ctx.DefaultLink != "" {
		// we don't have to fear data races, because being
		// commitMessageSubjectProcessors of fixed len and cap, every time we
		// append something to it the slice is realloc+copied, so append always
		// generates the slice ex-novo.
		procs = append(procs, genDefaultLinkProcessor(ctx.DefaultLink))
	}
	return renderProcessString(ctx, procs, content)
}

// RenderIssueTitle to process title on individual issue/pull page
func RenderIssueTitle(
	ctx *RenderContext,
	title string,
) (string, error) {
	return renderProcessString(ctx, []processor{
		inlineCodeBlockProcessor,
		issueIndexPatternProcessor,
		commitCrossReferencePatternProcessor,
		hashCurrentPatternProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, title)
}

// RenderRefIssueTitle to process title on places where an issue is referenced
func RenderRefIssueTitle(
	ctx *RenderContext,
	title string,
) (string, error) {
	return renderProcessString(ctx, []processor{
		inlineCodeBlockProcessor,
		issueIndexPatternProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, title)
}

func renderProcessString(ctx *RenderContext, procs []processor, content string) (string, error) {
	var buf strings.Builder
	if err := postProcess(ctx, procs, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderDescriptionHTML will use similar logic as PostProcess, but will
// use a single special linkProcessor.
func RenderDescriptionHTML(
	ctx *RenderContext,
	content string,
) (string, error) {
	return renderProcessString(ctx, []processor{
		descriptionLinkProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, escapeInlineCodeBlocks(content))
}

// RenderEmoji for when we want to just process emoji and shortcodes
// in various places it isn't already run through the normal markdown processor
func RenderEmoji(
	ctx *RenderContext,
	content string,
) (string, error) {
	return renderProcessString(ctx, emojiProcessors, content)
}

var (
	tagCleaner = regexp.MustCompile(`<((?:/?\w+/\w+)|(?:/[\w ]+/)|(/?[hH][tT][mM][lL]\b)|(/?[hH][eE][aA][dD]\b))`)
	nulCleaner = strings.NewReplacer("\000", "")
)

func postProcess(ctx *RenderContext, procs []processor, input io.Reader, output io.Writer) error {
	defer ctx.Cancel()
	// FIXME: don't read all content to memory
	rawHTML, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	// parse the HTML
	node, err := html.Parse(io.MultiReader(
		// prepend "<html><body>"
		strings.NewReader("<html><body>"),
		// Strip out nuls - they're always invalid
		bytes.NewReader(tagCleaner.ReplaceAll([]byte(nulCleaner.Replace(string(rawHTML))), []byte("&lt;$1"))),
		// close the tags
		strings.NewReader("</body></html>"),
	))
	if err != nil {
		return &postProcessError{"invalid HTML", err}
	}

	if node.Type == html.DocumentNode {
		node = node.FirstChild
	}

	visitNode(ctx, procs, node)

	newNodes := make([]*html.Node, 0, 5)

	if node.Data == "html" {
		node = node.FirstChild
		for node != nil && node.Data != "body" {
			node = node.NextSibling
		}
	}
	if node != nil {
		if node.Data == "body" {
			child := node.FirstChild
			for child != nil {
				newNodes = append(newNodes, child)
				child = child.NextSibling
			}
		} else {
			newNodes = append(newNodes, node)
		}
	}

	// Render everything to buf.
	for _, node := range newNodes {
		if err := html.Render(output, node); err != nil {
			return &postProcessError{"error rendering processed HTML", err}
		}
	}
	return nil
}

func visitNode(ctx *RenderContext, procs []processor, node *html.Node) {
	// Add user-content- to IDs and "#" links if they don't already have them
	for idx, attr := range node.Attr {
		val := strings.TrimPrefix(attr.Val, "#")
		notHasPrefix := !strings.HasPrefix(val, "user-content-") && !blackfridayExtRegex.MatchString(val)

		if attr.Key == "id" && notHasPrefix {
			node.Attr[idx].Val = "user-content-" + attr.Val
		}

		if attr.Key == "href" && strings.HasPrefix(attr.Val, "#") && notHasPrefix {
			node.Attr[idx].Val = "#user-content-" + val
		}

		if attr.Key == "class" && attr.Val == "emoji" {
			procs = nil
		}
	}

	// We ignore code and pre.
	switch node.Type {
	case html.TextNode:
		processTextNodes(ctx, procs, node)
	case html.ElementNode:
		if node.Data == "img" {
			for i, attr := range node.Attr {
				if attr.Key != "src" {
					continue
				}
				if len(attr.Val) > 0 && !IsLinkStr(attr.Val) && !strings.HasPrefix(attr.Val, "data:image/") {
					// Skip URLs already resolved from markdown, they start with `/user/repo/media/branch/...`
					// Only raw HTML <img> tags need processing here.
					mediaBase := ctx.Links.ResolveMediaLink(ctx.IsWiki, true)
					if !strings.HasPrefix(attr.Val, mediaBase+"/") {
						isRootRelative := strings.HasPrefix(attr.Val, "/")
						if isRootRelative {
							attr.Val = strings.TrimLeft(attr.Val, "/")
						}
						attr.Val = util.URLJoin(ctx.Links.ResolveMediaLink(ctx.IsWiki, isRootRelative), attr.Val)
					}
				}
				attr.Val = camoHandleLink(attr.Val)
				node.Attr[i] = attr
			}
		} else if node.Data == "a" {
			// Restrict text in links to emojis
			procs = emojiProcessors
		} else if node.Data == "code" || node.Data == "pre" {
			return
		} else if node.Data == "i" {
			for _, attr := range node.Attr {
				if attr.Key != "class" {
					continue
				}
				classes := strings.Split(attr.Val, " ")
				for i, class := range classes {
					if class == "icon" {
						classes[0], classes[i] = classes[i], classes[0]
						attr.Val = strings.Join(classes, " ")

						// Remove all children of icons
						child := node.FirstChild
						for child != nil {
							node.RemoveChild(child)
							child = node.FirstChild
						}
						break
					}
				}
			}
		}
		for n := node.FirstChild; n != nil; n = n.NextSibling {
			visitNode(ctx, procs, n)
		}
	default:
	}
	// ignore everything else
}

// processTextNodes runs the passed node through various processors, in order to handle
// all kinds of special links handled by the post-processing.
func processTextNodes(ctx *RenderContext, procs []processor, node *html.Node) {
	for _, p := range procs {
		p(ctx, node)
	}
}

// createKeyword() renders a highlighted version of an action keyword
func createKeyword(content string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, html.Attribute{Key: "class", Val: keywordClass})

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	span.AppendChild(text)

	return span
}

func createInlineCode(content string) *html.Node {
	code := &html.Node{
		Type: html.ElementNode,
		Data: atom.Code.String(),
		Attr: []html.Attribute{},
	}

	code.Attr = append(code.Attr, html.Attribute{Key: "class", Val: "inline-code-block"})

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	code.AppendChild(text)
	return code
}

func createEmoji(content, class, name, alias string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	if class != "" {
		span.Attr = append(span.Attr, html.Attribute{Key: "class", Val: class})
	}
	if name != "" {
		span.Attr = append(span.Attr, html.Attribute{Key: "aria-label", Val: name})
	}
	if alias != "" {
		span.Attr = append(span.Attr, html.Attribute{Key: "data-alias", Val: alias})
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	span.AppendChild(text)
	return span
}

func createCustomEmoji(alias string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, html.Attribute{Key: "class", Val: "emoji"})
	span.Attr = append(span.Attr, html.Attribute{Key: "aria-label", Val: alias})
	span.Attr = append(span.Attr, html.Attribute{Key: "data-alias", Val: alias})

	img := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Img,
		Data:     "img",
		Attr:     []html.Attribute{},
	}
	img.Attr = append(img.Attr, html.Attribute{Key: "alt", Val: ":" + alias + ":"})
	img.Attr = append(img.Attr, html.Attribute{Key: "src", Val: setting.StaticURLPrefix + "/assets/img/emoji/" + alias + ".png"})

	span.AppendChild(img)
	return span
}

func createLink(href, content, class string) *html.Node {
	a := &html.Node{
		Type: html.ElementNode,
		Data: atom.A.String(),
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}

	if class != "" {
		a.Attr = append(a.Attr, html.Attribute{Key: "class", Val: class})
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	a.AppendChild(text)
	return a
}

func createCodeLink(href, content, class string) *html.Node {
	a := &html.Node{
		Type: html.ElementNode,
		Data: atom.A.String(),
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}

	if class != "" {
		a.Attr = append(a.Attr, html.Attribute{Key: "class", Val: class})
	}

	unescaped, err := url.QueryUnescape(content)
	if err != nil {
		unescaped = content
	}
	text := &html.Node{
		Type: html.TextNode,
		Data: unescaped,
	}

	code := &html.Node{
		Type: html.ElementNode,
		Data: atom.Code.String(),
		Attr: []html.Attribute{{Key: "class", Val: "nohighlight"}},
	}

	code.AppendChild(text)
	a.AppendChild(code)
	return a
}

// replaceContent takes text node, and in its content it replaces a section of
// it with the specified newNode.
func replaceContent(node *html.Node, i, j int, newNode *html.Node) {
	replaceContentList(node, i, j, []*html.Node{newNode})
}

// replaceContentList takes text node, and in its content it replaces a section of
// it with the specified newNodes. An example to visualize how this can work can
// be found here: https://play.golang.org/p/5zP8NnHZ03s
func replaceContentList(node *html.Node, i, j int, newNodes []*html.Node) {
	// get the data before and after the match
	before := node.Data[:i]
	after := node.Data[j:]

	// Replace in the current node the text, so that it is only what it is
	// supposed to have.
	node.Data = before

	// Get the current next sibling, before which we place the replaced data,
	// and after that we place the new text node.
	nextSibling := node.NextSibling
	for _, n := range newNodes {
		node.Parent.InsertBefore(n, nextSibling)
	}
	if after != "" {
		node.Parent.InsertBefore(&html.Node{
			Type: html.TextNode,
			Data: after,
		}, nextSibling)
	}
}

func mentionProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		// We replace only the first mention; other mentions will be addressed later
		found, loc := references.FindFirstMentionBytes([]byte(node.Data[start:]))
		if !found {
			return
		}
		loc.Start += start
		loc.End += start
		mention := node.Data[loc.Start:loc.End]
		var teams string
		teams, ok := ctx.Metas["teams"]
		// FIXME: util.URLJoin may not be necessary here:
		// - setting.AppURL is defined to have a terminal '/' so unless mention[1:]
		// is an AppSubURL link we can probably fallback to concatenation.
		// team mention should follow @orgName/teamName style
		if ok && strings.Contains(mention, "/") {
			mentionOrgAndTeam := strings.Split(mention, "/")
			if mentionOrgAndTeam[0][1:] == ctx.Metas["org"] && strings.Contains(teams, ","+strings.ToLower(mentionOrgAndTeam[1])+",") {
				replaceContent(node, loc.Start, loc.End, createLink(util.URLJoin(ctx.Links.Prefix(), "org", ctx.Metas["org"], "teams", mentionOrgAndTeam[1]), mention, "mention"))
				node = node.NextSibling.NextSibling
				start = 0
				continue
			}
			start = loc.End
			continue
		}
		mentionedUsername := mention[1:]

		if DefaultProcessorHelper.IsUsernameMentionable != nil && DefaultProcessorHelper.IsUsernameMentionable(ctx.Ctx, mentionedUsername) {
			replaceContent(node, loc.Start, loc.End, createLink(util.URLJoin(ctx.Links.Prefix(), mentionedUsername), mention, "mention"))
			node = node.NextSibling.NextSibling
			start = 0
		} else {
			start = loc.End
		}
	}
}

func shortLinkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := shortLinkPattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		content := node.Data[m[2]:m[3]]
		tail := node.Data[m[4]:m[5]]
		props := make(map[string]string)

		// MediaWiki uses [[link|text]], while GitHub uses [[text|link]]
		// It makes page handling terrible, but we prefer GitHub syntax
		// And fall back to MediaWiki only when it is obvious from the look
		// Of text and link contents
		sl := strings.SplitSeq(content, "|")
		for v := range sl {
			if found := strings.Contains(v, "="); !found {
				// There is no equal in this argument; this is a mandatory arg
				if props["name"] == "" {
					if IsLinkStr(v) {
						// If we clearly see it is a link, we save it so

						// But first we need to ensure, that if both mandatory args provided
						// look like links, we stick to GitHub syntax
						if props["link"] != "" {
							props["name"] = props["link"]
						}

						props["link"] = strings.TrimSpace(v)
					} else {
						props["name"] = v
					}
				} else {
					props["link"] = strings.TrimSpace(v)
				}
			} else {
				// There is an equal; optional argument.

				before, after, _ := strings.Cut(v, "=")
				key, val := before, html.UnescapeString(after)

				// When parsing HTML, x/net/html will change all quotes which are
				// not used for syntax into UTF-8 quotes. So checking val[0] won't
				// be enough, since that only checks a single byte.
				if len(val) > 1 {
					if (strings.HasPrefix(val, "“") && strings.HasSuffix(val, "”")) ||
						(strings.HasPrefix(val, "‘") && strings.HasSuffix(val, "’")) {
						const lenQuote = len("‘")
						val = val[lenQuote : len(val)-lenQuote]
					} else if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
						(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
						val = val[1 : len(val)-1]
					} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "’") {
						const lenQuote = len("‘")
						val = val[1 : len(val)-lenQuote]
					}
				}
				props[key] = val
			}
		}

		var name, link string
		if props["link"] != "" {
			link = props["link"]
		} else if props["name"] != "" {
			link = props["name"]
		}
		if props["title"] != "" {
			name = props["title"]
		} else if props["name"] != "" {
			name = props["name"]
		} else {
			name = link
		}

		name += tail
		image := false
		switch ext := filepath.Ext(link); ext {
		// fast path: empty string, ignore
		case "":
			// leave image as false
			break
		case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".gif", ".bmp", ".ico", ".svg":
			image = true
		}

		childNode := &html.Node{}
		linkNode := &html.Node{
			FirstChild: childNode,
			LastChild:  childNode,
			Type:       html.ElementNode,
			Data:       "a",
			DataAtom:   atom.A,
		}
		childNode.Parent = linkNode
		absoluteLink := IsLinkStr(link)
		if !absoluteLink {
			if image {
				link = strings.ReplaceAll(link, " ", "+")
			} else {
				link = strings.ReplaceAll(link, " ", "-")
			}
			if !strings.Contains(link, "/") {
				link = url.PathEscape(link)
			}
		}
		if image {
			if !absoluteLink {
				isRootRelative := strings.HasPrefix(link, "/")
				if isRootRelative {
					link = strings.TrimLeft(link, "/")
				}
				link = util.URLJoin(ctx.Links.ResolveMediaLink(ctx.IsWiki, isRootRelative), link)
			}
			title := props["title"]
			if title == "" {
				title = props["alt"]
			}
			if title == "" {
				title = path.Base(name)
			}
			alt := props["alt"]

			// make the childNode an image - if we can, we also place the alt
			childNode.Type = html.ElementNode
			childNode.Data = "img"
			childNode.DataAtom = atom.Img
			childNode.Attr = []html.Attribute{
				{Key: "src", Val: link},
				{Key: "title", Val: title},
				{Key: "alt", Val: alt},
			}
		} else {
			if !absoluteLink {
				isRootRelative := strings.HasPrefix(link, "/")
				if isRootRelative {
					link = strings.TrimLeft(link, "/")
				}
				if ctx.IsWiki {
					link = util.URLJoin(ctx.Links.WikiLink(), link)
				} else if isRootRelative && ctx.Links.HasBranchInfo() {
					link = util.URLJoin(ctx.Links.SrcLinkBase(), link)
				} else {
					link = util.URLJoin(ctx.Links.SrcLink(), link)
				}
			}
			childNode.Type = html.TextNode
			childNode.Data = name
		}
		linkNode.Attr = []html.Attribute{{Key: "href", Val: link}}
		replaceContent(node, m[0], m[1], linkNode)
		node = node.NextSibling.NextSibling
	}
}

// pullReviewCommitPatternProcessor creates links to pull review commits.
func pullReviewCommitPatternProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := pullReviewCommitPattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		urlFull := node.Data[m[0]:m[1]]
		repoSlug := node.Data[m[2]:m[3]]
		id := node.Data[m[4]:m[5]]
		sha := base.ShortSha(node.Data[m[6]:m[7]])

		// Create an `<a>` node with a text of
		// `!123 (commit <code>abcdef1234</code>)`
		aNode := &html.Node{
			Type: html.ElementNode,
			Data: atom.A.String(),
			Attr: []html.Attribute{{Key: "href", Val: urlFull}, {Key: "class", Val: "commit"}},
		}

		text := "!" + id + " (commit "

		optionalRepoSlugAndInstancePath(ctx, &text, urlFull, repoSlug)

		aNode.AppendChild(&html.Node{
			Type: html.TextNode,
			Data: text,
		})

		textNode := &html.Node{
			Type: html.TextNode,
			Data: sha,
		}

		codeNode := &html.Node{
			Type: html.ElementNode,
			Data: atom.Code.String(),
			Attr: []html.Attribute{{Key: "class", Val: "nohighlight"}},
		}

		codeNode.AppendChild(textNode)
		aNode.AppendChild(codeNode)

		aNode.AppendChild(&html.Node{
			Type: html.TextNode,
			Data: ")",
		})

		replaceContent(node, m[0], m[1], aNode)
		node = node.NextSibling.NextSibling
	}
}

func fullIssuePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}
	next := node.NextSibling
	for node != nil && node != next {
		re := getIssueFullPattern()
		linkIndex, m := re.FindStringIndex(node.Data), re.FindStringSubmatch(node.Data)
		if linkIndex == nil || m == nil {
			return
		}

		link := node.Data[linkIndex[0]:linkIndex[1]]
		text := "#" + m[re.SubexpIndex("num")] + m[re.SubexpIndex("subpath")]

		if len(m[re.SubexpIndex("comment")]) > 0 {
			if locale, ok := ctx.Ctx.Value(translation.ContextKey).(translation.Locale); ok {
				text += " " + locale.TrString("repo.from_comment")
			} else {
				text += " (comment)"
			}
		}

		matchUser := m[re.SubexpIndex("user")]
		matchRepo := m[re.SubexpIndex("repo")]

		if matchUser == ctx.Metas["user"] && matchRepo == ctx.Metas["repo"] {
			replaceContent(node, linkIndex[0], linkIndex[1], createLink(link, text, "ref-issue"))
		} else {
			text = matchUser + "/" + matchRepo + text
			replaceContent(node, linkIndex[0], linkIndex[1], createLink(link, text, "ref-issue"))
		}
		node = node.NextSibling.NextSibling
	}
}

func issueIndexPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}

	// FIXME: the use of "mode" is quite dirty and hacky, for example: what is a "document"? how should it be rendered?
	// The "mode" approach should be refactored to some other more clear&reliable way.
	crossLinkOnly := (ctx.Metas["mode"] == "document" && !ctx.IsWiki)

	var (
		found bool
		ref   *references.RenderizableReference
	)

	next := node.NextSibling

	for node != nil && node != next {
		_, hasExtTrackFormat := ctx.Metas["format"]

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		isNumericStyle := ctx.Metas["style"] == "" || ctx.Metas["style"] == IssueNameStyleNumeric
		foundNumeric, refNumeric := references.FindRenderizableReferenceNumeric(node.Data, hasExtTrackFormat && !isNumericStyle, crossLinkOnly)

		switch ctx.Metas["style"] {
		case "", IssueNameStyleNumeric:
			found, ref = foundNumeric, refNumeric
		case IssueNameStyleAlphanumeric:
			found, ref = references.FindRenderizableReferenceAlphanumeric(node.Data)
		case IssueNameStyleRegexp:
			pattern, err := regexplru.GetCompiled(ctx.Metas["regexp"])
			if err != nil {
				return
			}
			found, ref = references.FindRenderizableReferenceRegexp(node.Data, pattern)
		}

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		if hasExtTrackFormat && !isNumericStyle && refNumeric != nil {
			// If numeric (PR) was found, and it was BEFORE the non-numeric pattern, use that
			// Allow a free-pass when non-numeric pattern wasn't found.
			if found && (ref == nil || refNumeric.RefLocation.Start < ref.RefLocation.Start) {
				found = foundNumeric
				ref = refNumeric
			}
		}
		if !found {
			return
		}

		var link *html.Node
		reftext := node.Data[ref.RefLocation.Start:ref.RefLocation.End]
		if hasExtTrackFormat && !ref.IsPull && ref.Owner == "" {
			ctx.Metas["index"] = ref.Issue

			res, err := vars.Expand(ctx.Metas["format"], ctx.Metas)
			if err != nil {
				// here we could just log the error and continue the rendering
				log.Error("unable to expand template vars for ref %s, err: %v", ref.Issue, err)
			}

			link = createLink(res, reftext, "ref-issue ref-external-issue")
		} else {
			// Path determines the type of link that will be rendered. It's unknown at this point whether
			// the linked item is actually a PR or an issue. Luckily it's of no real consequence because
			// Forgejo will redirect on click as appropriate.
			path := "issues"
			if ref.IsPull {
				path = "pulls"
			}
			if ref.Owner == "" {
				link = createLink(util.URLJoin(ctx.Links.Prefix(), ctx.Metas["user"], ctx.Metas["repo"], path, ref.Issue), reftext, "ref-issue")
			} else {
				link = createLink(util.URLJoin(ctx.Links.Prefix(), ref.Owner, ref.Name, path, ref.Issue), reftext, "ref-issue")
			}
		}

		if ref.Action == references.XRefActionNone {
			replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
			node = node.NextSibling.NextSibling
			continue
		}

		// Decorate action keywords if actionable
		var keyword *html.Node
		if references.IsXrefActionable(ref, hasExtTrackFormat) {
			keyword = createKeyword(node.Data[ref.ActionLocation.Start:ref.ActionLocation.End])
		} else {
			keyword = &html.Node{
				Type: html.TextNode,
				Data: node.Data[ref.ActionLocation.Start:ref.ActionLocation.End],
			}
		}
		spaces := &html.Node{
			Type: html.TextNode,
			Data: node.Data[ref.ActionLocation.End:ref.RefLocation.Start],
		}
		replaceContentList(node, ref.ActionLocation.Start, ref.RefLocation.End, []*html.Node{keyword, spaces, link})
		node = node.NextSibling.NextSibling.NextSibling.NextSibling
	}
}

func commitCrossReferencePatternProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling

	for node != nil && node != next {
		found, ref := references.FindRenderizableCommitCrossReference(node.Data)
		if !found {
			return
		}

		reftext := ref.Owner + "/" + ref.Name + "@" + base.ShortSha(ref.CommitSha)
		link := createLink(util.URLJoin(ctx.Links.Prefix(), ref.Owner, ref.Name, "commit", ref.CommitSha), reftext, "commit")

		replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
		node = node.NextSibling.NextSibling
	}
}

// fullHashPatternProcessor renders URLs that contain a SHA
func fullHashPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}

	next := node.NextSibling
	for node != nil && node != next {
		m := anyHashPattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		urlFull := node.Data[m[0]:m[1]]

		// In most cases, the URL will look like this:
		// `https://domain.tld/<owner>/<repo>/<path>/<sha>`.
		// The amount of components in `<path>` is variable, but that alone is doable with regexp.
		//
		// However, Forgejo also allows being hosted on a sub path, i.e.
		// `https://domain.tld/<sub>/<owner>/<repo>/<path>/<sha>`.
		// And this sub path can also have any amount of components. But fishing out a section
		// between two variable length matches is not something regular grammars are capable of.
		//
		// Instead, the regexp extracts the entire path section before the SHA
		// (i.e. `<sub>/<owner>/<repo>/<path>`), and we find the components we need by counting.
		// `<sub>` is unknown, but the possible values for `<path>` are defined by us
		// (see `router/web/web.go`). So we count from the back.
		subPath := node.Data[m[2]:m[3]]

		components := strings.Split(subPath, "/")
		componentCount := len(components)

		// In most cases, the `<owner>` component is right at the start of the path.
		ownerIndex := 0

		// But if there are more than three components, this could be `<sub>` or an app route
		// with two components. Or both.
		if componentCount > 3 {
			// As mentioned, we count from the back. We decrement for the `<repo>` component, and the one
			// component from the app route that's guaranteed to be there.
			// We also adjust this to be an array index, so we subtract one more.
			ownerIndex = componentCount - 3

			// We then check for known app routes that use two components.
			// Currently, this checks for:
			// - `src/commit`
			// - `commits/commit`
			//
			// This does have one scenario where we cannot figure things out reliably:
			// If there is a sub path, and the repository is named like one of the known app routes
			// (e.g. `src`), we cannot distinguish between the repo and the app route.
			// We assume that naming a repository like that is uncommon, and prioritize the case where its
			// part of the app route.
			if components[componentCount-1] == "commit" &&
				(components[componentCount-2] == "src" || components[componentCount-2] == "commits") {
				ownerIndex--
			}
		}

		repoSlug := components[ownerIndex] + "/" + components[ownerIndex+1]

		text := base.ShortSha(node.Data[m[4]:m[5]])

		// We need to figure out the base of the provided URL, which is up to and including the
		// `<owner>/<repo>` slug.
		// With that we can determine if it matches the current repo, or if the slug should be shown.
		optionalRepoSlugAndInstancePath(ctx, &text, urlFull, repoSlug)

		// 3rd capture group matches an optional file path after the SHA
		filePath := ""
		if m[7] > 0 {
			filePath = node.Data[m[6]:m[7]]
		}

		// 5th capture group matches a optional url hash
		hash := ""
		if m[11] > 0 {
			hash = node.Data[m[10]:m[11]][1:]

			// Truncate long diff IDs
			if len(hash) > 15 && strings.HasPrefix(hash, "diff-") {
				hash = hash[:15]
			}
		}

		start := m[0]
		end := m[1]

		// If the URL ends in '.', it's very likely that it is not part of the
		// actual URL but used to finish a sentence.
		if strings.HasSuffix(urlFull, ".") {
			end--
			urlFull = urlFull[:len(urlFull)-1]
			if hash != "" {
				hash = hash[:len(hash)-1]
			} else if filePath != "" {
				filePath = filePath[:len(filePath)-1]
			}
		}

		if filePath != "" {
			decoded, err := url.QueryUnescape(filePath)
			if err != nil {
				text += decoded
			} else {
				text += filePath
			}
		}

		if hash != "" {
			text += " (" + hash + ")"
		}
		replaceContent(node, start, end, createCodeLink(urlFull, text, ""))
		node = node.NextSibling.NextSibling
	}
}

func comparePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}

	next := node.NextSibling
	for node != nil && node != next {
		m := comparePattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		// Ensure that every group (m[0]...m[9]) has a match
		for i := range 10 {
			if m[i] == -1 {
				return
			}
		}

		urlFull := node.Data[m[0]:m[1]]
		repoSlug := node.Data[m[2]:m[3]]
		text1 := base.ShortSha(node.Data[m[4]:m[5]])
		textDots := base.ShortSha(node.Data[m[6]:m[7]])
		text2 := base.ShortSha(node.Data[m[8]:m[9]])

		query := ""
		if m[11] > 0 {
			query = node.Data[m[10]:m[11]][1:]
		}

		hash := ""
		if m[13] > 0 {
			hash = node.Data[m[12]:m[13]][1:]
		}

		start := m[0]
		end := m[1]

		// If the URL ends in '.', it's very likely that it is not part of the
		// actual URL but used to finish a sentence.
		if strings.HasSuffix(urlFull, ".") {
			end--
			urlFull = urlFull[:len(urlFull)-1]
			if hash != "" {
				hash = hash[:len(hash)-1]
			} else if query != "" {
				query = query[:len(query)-1]
			} else if text2 != "" {
				text2 = text2[:len(text2)-1]
			}
		}

		text := text1 + textDots + text2

		optionalRepoSlugAndInstancePath(ctx, &text, urlFull, repoSlug)

		extra := ""
		if query != "" {
			query, err := url.ParseQuery(query)
			if err == nil && query.Has("files") {
				extra = query.Get("files")
			}
		}

		if hash != "" {
			if extra != "" {
				extra += "#"
			}

			extra += hash
		}

		if extra != "" {
			text += " (" + extra + ")"
		}
		replaceContent(node, start, end, createCodeLink(urlFull, text, "compare"))
		node = node.NextSibling.NextSibling
	}
}

func filePreviewPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil || ctx.Metas["user"] == "" || ctx.Metas["repo"] == "" {
		return
	}
	if DefaultProcessorHelper.GetRepoFileBlob == nil {
		return
	}

	locale := translation.NewLocale("en-US")
	if ctx.Ctx != nil {
		ctxLocale, ok := ctx.Ctx.Value(translation.ContextKey).(translation.Locale)
		if ok {
			locale = ctxLocale
		}
	}

	next := node.NextSibling
	for node != nil && node != next {
		if node.Parent == nil || node.Parent.Type != html.ElementNode {
			node = node.NextSibling
			continue
		}
		previews := NewFilePreviews(ctx, node, locale)
		if previews == nil {
			node = node.NextSibling
			continue
		}
		offset := 0
		for _, preview := range previews {
			previewNode := preview.CreateHTML(locale)

			// Specialized version of replaceContent, so the parent paragraph element is not destroyed from our div
			before := node.Data[:(preview.start - offset)]
			after := node.Data[(preview.end - offset):]
			afterTextNode := &html.Node{
				Type: html.TextNode,
				Data: after,
			}
			matched := true
			switch node.Parent.Data {
			case "div", "li", "td", "th", "details":
				nextSibling := node.NextSibling
				node.Parent.InsertBefore(previewNode, nextSibling)
				node.Parent.InsertBefore(afterTextNode, nextSibling)
			case "p", "span", "em", "strong":
				nextParentSibling := node.Parent.NextSibling
				node.Parent.Parent.InsertBefore(previewNode, nextParentSibling)
				afterNode := &html.Node{
					Type: html.ElementNode,
					Data: node.Parent.Data,
					Attr: node.Parent.Attr,
				}
				afterNode.AppendChild(afterTextNode)
				node.Parent.Parent.InsertBefore(afterNode, nextParentSibling)
				for sibling := node.NextSibling; sibling != nil; sibling = node.NextSibling {
					sibling.Parent.RemoveChild(sibling)
					afterNode.AppendChild(sibling)
				}
			default:
				matched = false
			}
			if matched {
				offset = preview.end
				node.Data = before
				node = afterTextNode
			}
		}
		node = node.NextSibling
	}
}

func inlineCodeBlockProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := InlineCodeBlockRegex.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}

		code := node.Data[m[0]+1 : m[1]-1]
		replaceContent(node, m[0], m[1], createInlineCode(code))
		node = node.NextSibling.NextSibling
	}
}

// emojiShortCodeProcessor for rendering text like :smile: into emoji
func emojiShortCodeProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := EmojiShortCodeRegex.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[0] += start
		m[1] += start

		start = m[1]

		alias := node.Data[m[0]:m[1]]
		alias = strings.ReplaceAll(alias, ":", "")
		converted := emoji.FromAlias(alias)
		if converted == nil {
			// check if this is a custom reaction
			if setting.UI.CustomEmojisLookup.Contains(alias) {
				replaceContent(node, m[0], m[1], createCustomEmoji(alias))
				node = node.NextSibling.NextSibling
				start = 0
				continue
			}
			continue
		}

		replaceContent(node, m[0], m[1], createEmoji(converted.Emoji, "emoji", converted.Description, alias))
		node = node.NextSibling.NextSibling
		start = 0
	}
}

// emoji processor to match emoji and add emoji class
func emojiProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := emoji.FindEmojiSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[0] += start
		m[1] += start

		codepoint := node.Data[m[0]:m[1]]
		start = m[1]
		val := emoji.FromCode(codepoint)
		if val != nil {
			replaceContent(node, m[0], m[1], createEmoji(codepoint, "emoji", val.Description, val.Aliases[0]))
			node = node.NextSibling.NextSibling
			start = 0
		}
	}
}

// hashCurrentPatternProcessor renders SHA1/SHA256 strings to corresponding links that
// are assumed to be in the same repository.
func hashCurrentPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil || ctx.Metas["user"] == "" || ctx.Metas["repo"] == "" || ctx.Metas["repoPath"] == "" {
		return
	}

	start := 0
	next := node.NextSibling
	if ctx.ShaExistCache == nil {
		ctx.ShaExistCache = make(map[string]bool)
	}
	for node != nil && node != next && start < len(node.Data) {
		m := hashCurrentPattern.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[2] += start
		m[3] += start

		hash := node.Data[m[2]:m[3]]
		// The regex does not lie, it matches the hash pattern.
		// However, a regex cannot know if a hash actually exists or not.
		// We could assume that a SHA1 hash should probably contain alphas AND numerics
		// but that is not always the case.
		// Although unlikely, deadbeef and 1234567 are valid short forms of SHA1 hash
		// as used by git and github for linking and thus we have to do similar.
		// Because of this, we check to make sure that a matched hash is actually
		// a commit in the repository before making it a link.

		// check cache first
		exist, inCache := ctx.ShaExistCache[hash]
		if !inCache {
			if ctx.GitRepo == nil {
				var err error
				ctx.GitRepo, err = git.OpenRepository(ctx.Ctx, ctx.Metas["repoPath"])
				if err != nil {
					log.Error("unable to open repository: %s Error: %v", ctx.Metas["repoPath"], err)
					return
				}
				ctx.AddCancel(func() {
					ctx.GitRepo.Close()
					ctx.GitRepo = nil
				})
			}

			exist = ctx.GitRepo.IsReferenceExist(hash)
			ctx.ShaExistCache[hash] = exist
		}

		if !exist {
			start = m[3]
			continue
		}

		link := util.URLJoin(ctx.Links.Prefix(), ctx.Metas["user"], ctx.Metas["repo"], "commit", hash)
		replaceContent(node, m[2], m[3], createCodeLink(link, base.ShortSha(hash), ""))
		start = 0
		node = node.NextSibling.NextSibling
	}
}

// fediAddressProcessor replaces raw fediverse handles with toolforge links
func fediAddressProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := fediRegex.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		fedihandle := node.Data[m[2]:m[3]]
		replaceContent(node, m[2], m[3], createLink("https://fedirect.toolforge.org/?id="+url.QueryEscape(fedihandle), fedihandle, "fedihandle"))
		node = node.NextSibling.NextSibling
	}
}

// emailAddressProcessor replaces raw email addresses with a mailto: link.
func emailAddressProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := emailRegex.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		mail := node.Data[m[2]:m[3]]
		replaceContent(node, m[2], m[3], createLink("mailto:"+mail, mail, "mailto"))
		node = node.NextSibling.NextSibling
	}
}

// linkProcessor creates links for any HTTP or HTTPS URL not captured by
// markdown.
func linkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := common.LinkRegex.FindStringIndex(node.Data)
		if m == nil {
			return
		}

		uri := node.Data[m[0]:m[1]]
		replaceContent(node, m[0], m[1], createLink(uri, uri, "link"))
		node = node.NextSibling.NextSibling
	}
}

func genDefaultLinkProcessor(defaultLink string) processor {
	return func(ctx *RenderContext, node *html.Node) {
		ch := &html.Node{
			Parent: node,
			Type:   html.TextNode,
			Data:   node.Data,
		}

		node.Type = html.ElementNode
		node.Data = "a"
		node.DataAtom = atom.A
		node.Attr = []html.Attribute{
			{Key: "href", Val: defaultLink},
			{Key: "class", Val: "default-link muted"},
		}
		node.FirstChild, node.LastChild = ch, ch
	}
}

// descriptionLinkProcessor creates links for DescriptionHTML
func descriptionLinkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := common.LinkRegex.FindStringIndex(node.Data)
		if m == nil {
			return
		}

		uri := node.Data[m[0]:m[1]]
		replaceContent(node, m[0], m[1], createDescriptionLink(uri, uri))
		node = node.NextSibling.NextSibling
	}
}

func createDescriptionLink(href, content string) *html.Node {
	textNode := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	linkNode := &html.Node{
		FirstChild: textNode,
		LastChild:  textNode,
		Type:       html.ElementNode,
		Data:       "a",
		DataAtom:   atom.A,
		Attr: []html.Attribute{
			{Key: "href", Val: href},
			{Key: "target", Val: "_blank"},
			{Key: "rel", Val: "noopener noreferrer"},
		},
	}
	textNode.Parent = linkNode
	return linkNode
}

// Adds an optional repo slug and optionally the instance domain and URL
//
// The repo slug is added if the link points to a different repo
// The instance domain and sub-path is added if the link points to a different instance
func optionalRepoSlugAndInstancePath(ctx *RenderContext, text *string, fullURL, slug string) {
	if len(ctx.Links.Base) > 0 {
		// The fullURL is the url to e.g. the commit. The slug is e.g. `forgejo/forgejo`.
		// To retrieve the instance domain and sub-path we need to remove the repo slug
		slugStart := strings.LastIndex(fullURL, slug)
		targetInstance := fullURL[:slugStart]

		// Check if the URL points to a different instance
		if setting.AppURL != targetInstance {
			// Remove the http scheme for displaying
			targetInstance = httpSchemePattern.ReplaceAllString(targetInstance, "")

			*text = targetInstance + slug + "@" + *text
		} else if !strings.HasSuffix(strings.TrimSuffix(ctx.Links.Base, "/"), slug) {
			// If it is a link to a different repo, but on the same instance only add the repo slug
			*text = slug + "@" + *text
		}
	}
}

// escapeInlineCodeBlocks escapes HTML symbols in contents of Markdown inline code blocks
// to prevent clashing with HTML parsing
func escapeInlineCodeBlocks(input string) string {
	return InlineCodeBlockRegex.ReplaceAllStringFunc(input, html.EscapeString)
}
