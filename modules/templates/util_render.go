// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	issues_model "forgejo.org/models/issues"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/emoji"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
)

// RenderCommitMessage renders commit message with XSS-safe and special links.
func RenderCommitMessage(ctx context.Context, msg string, metas map[string]string) template.HTML {
	cleanMsg := html.EscapeString(msg)
	// we can safely assume that it will not return any error, since there
	// shouldn't be any special HTML.
	fullMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		Ctx:   ctx,
		Metas: metas,
	}, cleanMsg)
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	msgLines := strings.Split(strings.TrimSpace(fullMessage), "\n")
	if len(msgLines) == 0 {
		return template.HTML("")
	}
	return RenderCodeBlock(template.HTML(msgLines[0]))
}

// RenderCommitMessageLinkSubject renders commit message as a XSS-safe link to
// the provided default url, handling for special links without email to links.
func RenderCommitMessageLinkSubject(ctx context.Context, msg, urlDefault string, metas map[string]string) template.HTML {
	msgLine := strings.TrimLeftFunc(msg, unicode.IsSpace)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[:lineEnd]
	}
	msgLine = strings.TrimRightFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return template.HTML("")
	}

	// we can safely assume that it will not return any error, since there
	// shouldn't be any special HTML.
	renderedMessage, err := markup.RenderCommitMessageSubject(&markup.RenderContext{
		Ctx:         ctx,
		DefaultLink: urlDefault,
		Metas:       metas,
	}, html.EscapeString(msgLine))
	if err != nil {
		log.Error("RenderCommitMessageSubject: %v", err)
		return template.HTML("")
	}
	return RenderCodeBlock(template.HTML(renderedMessage))
}

// RenderCommitBody extracts the body of a commit message without its title.
func RenderCommitBody(ctx context.Context, msg string, metas map[string]string) template.HTML {
	msgLine := strings.TrimSpace(msg)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[lineEnd+1:]
	} else {
		return ""
	}
	msgLine = strings.TrimLeftFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return ""
	}

	renderedMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		Ctx:   ctx,
		Metas: metas,
	}, html.EscapeString(msgLine))
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	return template.HTML(renderedMessage)
}

// Match text that is between back ticks.
var codeMatcher = regexp.MustCompile("`([^`]+)`")

// RenderCodeBlock renders "`…`" as highlighted "<code>" block, intended for issue and PR titles
func RenderCodeBlock(htmlEscapedTextToRender template.HTML) template.HTML {
	htmlWithCodeTags := codeMatcher.ReplaceAllString(string(htmlEscapedTextToRender), `<code class="inline-code-block">$1</code>`) // replace with HTML <code> tags
	return template.HTML(htmlWithCodeTags)
}

const (
	activeLabelOpacity   = uint8(255)
	archivedLabelOpacity = uint8(127)
)

func GetLabelOpacityByte(isArchived bool) uint8 {
	if isArchived {
		return archivedLabelOpacity
	}
	return activeLabelOpacity
}

// RenderIssueTitle renders issue/pull title with defined post processors
func RenderIssueTitle(ctx context.Context, text string, metas map[string]string) template.HTML {
	renderedText, err := markup.RenderIssueTitle(&markup.RenderContext{
		Ctx:   ctx,
		Metas: metas,
	}, html.EscapeString(text))
	if err != nil {
		log.Error("RenderIssueTitle: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedText)
}

// RenderRefIssueTitle renders referenced issue/pull title with defined post processors
func RenderRefIssueTitle(ctx context.Context, text string) template.HTML {
	renderedText, err := markup.RenderRefIssueTitle(&markup.RenderContext{Ctx: ctx}, html.EscapeString(text))
	if err != nil {
		log.Error("RenderRefIssueTitle: %v", err)
		return ""
	}

	return template.HTML(renderedText)
}

// RenderLabel renders a label
// locale is needed due to an import cycle with our context providing the `Tr` function
func RenderLabel(ctx *Context, label *issues_model.Label) template.HTML {
	var (
		archivedCSSClass string
		textColor        = util.ContrastColor(label.Color)
		labelScope       = label.ExclusiveScope()
	)

	description := emoji.ReplaceAliases(html.EscapeString(label.Description))

	if label.IsArchived() {
		archivedCSSClass = "archived-label"
		description = ctx.Locale.TrString("repo.issues.archived_label_description", description)
	}

	if labelScope == "" {
		// Regular label

		labelColor := label.Color + hex.EncodeToString([]byte{GetLabelOpacityByte(label.IsArchived())})
		s := fmt.Sprintf("<div class='ui label %s' style='color: %s !important; background-color: %s !important;' data-tooltip-content='%s' aria-description='%s'>%s</div>",
			archivedCSSClass, textColor, labelColor, description, description, RenderEmoji(ctx, label.Name))
		return template.HTML(s)
	}

	// Scoped label
	scopeText := RenderEmoji(ctx, labelScope)
	itemText := RenderEmoji(ctx, label.Name[len(labelScope)+1:])

	// Make scope and item background colors slightly darker and lighter respectively.
	// More contrast needed with higher luminance, empirically tweaked.
	luminance := util.GetRelativeLuminance(label.Color)
	contrast := 0.01 + luminance*0.03
	// Ensure we add the same amount of contrast also near 0 and 1.
	darken := contrast + math.Max(luminance+contrast-1.0, 0.0)
	lighten := contrast + math.Max(contrast-luminance, 0.0)
	// Compute factor to keep RGB values proportional.
	darkenFactor := math.Max(luminance-darken, 0.0) / math.Max(luminance, 1.0/255.0)
	lightenFactor := math.Min(luminance+lighten, 1.0) / math.Max(luminance, 1.0/255.0)

	opacity := GetLabelOpacityByte(label.IsArchived())
	r, g, b := util.HexToRBGColor(label.Color)
	scopeBytes := []byte{
		uint8(math.Min(math.Round(r*darkenFactor), 255)),
		uint8(math.Min(math.Round(g*darkenFactor), 255)),
		uint8(math.Min(math.Round(b*darkenFactor), 255)),
		opacity,
	}
	itemBytes := []byte{
		uint8(math.Min(math.Round(r*lightenFactor), 255)),
		uint8(math.Min(math.Round(g*lightenFactor), 255)),
		uint8(math.Min(math.Round(b*lightenFactor), 255)),
		opacity,
	}

	scopeColor := "#" + hex.EncodeToString(scopeBytes)
	itemColor := "#" + hex.EncodeToString(itemBytes)

	s := fmt.Sprintf("<span class='ui label %s scope-parent' data-tooltip-content='%s' aria-description='%s'>"+
		"<div class='ui label scope-left' style='color: %s !important; background-color: %s !important'>%s</div>"+
		"<div class='ui label scope-right' style='color: %s !important; background-color: %s !important'>%s</div>"+
		"</span>",
		archivedCSSClass, description, description,
		textColor, scopeColor, scopeText,
		textColor, itemColor, itemText)
	return template.HTML(s)
}

// RenderEmoji renders html text with emoji post processors
func RenderEmoji(ctx context.Context, text string) template.HTML {
	renderedText, err := markup.RenderEmoji(&markup.RenderContext{Ctx: ctx},
		html.EscapeString(text))
	if err != nil {
		log.Error("RenderEmoji: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedText)
}

// ReactionToEmoji renders emoji for use in reactions
func ReactionToEmoji(reaction string) template.HTML {
	val := emoji.FromCode(reaction)
	if val != nil {
		return template.HTML(val.Emoji)
	}
	val = emoji.FromAlias(reaction)
	if val != nil {
		return template.HTML(val.Emoji)
	}
	return template.HTML(fmt.Sprintf(`<img alt=":%s:" src="%s/assets/img/emoji/%s.png"></img>`, reaction, setting.StaticURLPrefix, url.PathEscape(reaction)))
}

func RenderMarkdownToHtml(ctx context.Context, input string) template.HTML { //nolint:revive
	output, err := markdown.RenderString(&markup.RenderContext{
		Ctx:   ctx,
		Metas: map[string]string{"mode": "document"},
	}, input)
	if err != nil {
		log.Error("RenderString: %v", err)
	}
	return output
}

func RenderLabels(ctx *Context, labels []*issues_model.Label, repoLink string, isPull bool) template.HTML {
	var htmlCode strings.Builder
	htmlCode.WriteString(`<span class="labels-list">`)
	for _, label := range labels {
		// Protect against nil value in labels - shouldn't happen but would cause a panic if so
		if label == nil {
			continue
		}

		issuesOrPull := "issues"
		if isPull {
			issuesOrPull = "pulls"
		}
		fmt.Fprintf(&htmlCode, "<a href='%s/%s?labels=%d' rel='nofollow'>%s</a> ",
			repoLink, issuesOrPull, label.ID, RenderLabel(ctx, label))
	}
	htmlCode.WriteString("</span>")
	return template.HTML(htmlCode.String())
}

func RenderUser(ctx context.Context, user user_model.User) template.HTML {
	if user.ID > 0 {
		return template.HTML(fmt.Sprintf(
			"<a href='%s' rel='nofollow'><strong>%s</strong></a>",
			user.HomeLink(), html.EscapeString(user.GetDisplayName()),
		))
	}
	return template.HTML(fmt.Sprintf("<strong>%s</strong>",
		html.EscapeString(user.GetDisplayName())))
}

func RenderReviewRequest(ctx context.Context, users []issues_model.RequestReviewTarget) template.HTML {
	usernames := make([]string, 0, len(users))
	for _, user := range users {
		if user.ID() > 0 {
			usernames = append(usernames, fmt.Sprintf(
				"<a href='%s' rel='nofollow'><strong>%s</strong></a>",
				user.Link(ctx), html.EscapeString(user.Name()),
			))
		} else {
			usernames = append(usernames, fmt.Sprintf(
				"<strong>%s</strong>",
				html.EscapeString(user.Name()),
			))
		}
	}

	htmlCode := `<span class="review-request-list">`
	htmlCode += strings.Join(usernames, ", ")
	htmlCode += "</span>"
	return template.HTML(htmlCode)
}
