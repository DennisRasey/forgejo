// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
	base "forgejo.org/modules/migration"
	"forgejo.org/modules/structs"
	"forgejo.org/services/migrations/allowlist"
)

var (
	_ base.Downloader        = &BitbucketDataCenterDownloader{}
	_ base.DownloaderFactory = &BitbucketDataCenterDownloaderFactory{}

	errBitbucketNotFound = errors.New("bitbucket resource not found")
)

func init() {
	RegisterDownloaderFactory(&BitbucketDataCenterDownloaderFactory{})
}

// BitbucketDataCenterDownloaderFactory defines a downloader factory
type BitbucketDataCenterDownloaderFactory struct{}

// New returns a downloader related to this factory according MigrateOptions
func (f *BitbucketDataCenterDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL, project, repoSlug, err := parseBitbucketDataCenterURL(u)
	if err != nil {
		return nil, err
	}

	log.Trace("Create Bitbucket Data Center downloader. BaseURL: %s Project: %s Repo: %s", baseURL, project, repoSlug)
	return newBitbucketDataCenterDownloader(ctx, baseURL, project, repoSlug, opts.AuthToken), nil
}

// GitServiceType returns the type of git service.
func (f *BitbucketDataCenterDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.BitbucketDataCenterService
}

// parseBitbucketDataCenterURL extracts the REST base URL, project key and repo slug from a Bitbucket clone URL: [context]/scm/{project}/{slug}.git
func parseBitbucketDataCenterURL(u *url.URL) (baseURL *url.URL, project, repoSlug string, err error) {
	ctxPath, after, ok := strings.Cut("/"+strings.Trim(u.Path, "/"), "/scm/")
	fields := strings.SplitN(strings.Trim(after, "/"), "/", 2)
	if !ok || len(fields) != 2 {
		return nil, "", "", fmt.Errorf("invalid Bitbucket clone path, expected …/scm/{project}/{repository}: %s", u.Path)
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host, Path: ctxPath}, fields[0], strings.TrimSuffix(fields[1], ".git"), nil
}

// BitbucketDataCenterDownloader implements Downloader for Bitbucket Data Center / Server.
type BitbucketDataCenterDownloader struct {
	base.NullDownloader
	ctx      context.Context
	client   *http.Client
	baseURL  *url.URL
	project  string
	repoSlug string
	token    string
	// Bitbucket pages with an opaque cursor: GetPullRequests is called with sequential pages,
	// so the nextPageStart of the previous response is the start of the next one.
	prNextPageStart int
}

type bitbucketUser struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
}

type bitbucketCloneLinks struct {
	Clone []struct {
		Href string `json:"href"`
		Name string `json:"name"`
	} `json:"clone"`
}

// httpURL returns the http(s) clone link; empty when HTTP(S) SCM hosting is disabled.
func (l bitbucketCloneLinks) httpURL() string {
	for _, c := range l.Clone {
		if c.Name == "http" || c.Name == "https" {
			return c.Href
		}
	}
	return ""
}

type bitbucketRef struct {
	DisplayID    string `json:"displayId"`
	LatestCommit string `json:"latestCommit"`
	Repository   struct {
		Slug    string `json:"slug"`
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Links bitbucketCloneLinks `json:"links"`
	} `json:"repository"`
}

// newBitbucketDataCenterDownloader creates a Bitbucket Data Center downloader.
func newBitbucketDataCenterDownloader(ctx context.Context, baseURL *url.URL, project, repoSlug, token string) *BitbucketDataCenterDownloader {
	return &BitbucketDataCenterDownloader{
		ctx:      ctx,
		client:   allowlist.NewMigrationHTTPClient(),
		baseURL:  baseURL,
		project:  project,
		repoSlug: repoSlug,
		token:    token,
	}
}

// SetContext set context
func (d *BitbucketDataCenterDownloader) SetContext(ctx context.Context) {
	d.ctx = ctx
}

// String implements Stringer
func (d *BitbucketDataCenterDownloader) String() string {
	return fmt.Sprintf("migration from Bitbucket Data Center server %s %s/%s", d.baseURL, d.project, d.repoSlug)
}

func (d *BitbucketDataCenterDownloader) LogString() string {
	if d == nil {
		return "<BitbucketDataCenterDownloader nil>"
	}
	return fmt.Sprintf("<BitbucketDataCenterDownloader %s %s/%s>", d.baseURL, d.project, d.repoSlug)
}

// FormatCloneURL injects credentials into the clone URL; the token is the Basic-auth password,
// with the account username for user tokens or "x-token-auth" for project/repository tokens.
func (d *BitbucketDataCenterDownloader) FormatCloneURL(opts base.MigrateOptions, remoteAddr string) (string, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return "", err
	}
	if opts.AuthToken != "" {
		user := opts.AuthUsername
		if user == "" {
			user = "x-token-auth"
		}
		u.User = url.UserPassword(user, opts.AuthToken)
	} else if opts.AuthUsername != "" {
		u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
	}
	return u.String(), nil
}

// callAPI performs a GET against the Bitbucket Data Center REST API v1 and decodes the JSON result.
func (d *BitbucketDataCenterDownloader) callAPI(endpoint string, query url.Values, result any) error {
	u := d.baseURL.JoinPath("rest/api/1.0", endpoint)
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(d.ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errBitbucketNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: unexpected status %d", endpoint, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

// GetRepoInfo returns repository information.
// https://docs.atlassian.com/bitbucket-server/rest/latest/bitbucket-rest.html
func (d *BitbucketDataCenterDownloader) GetRepoInfo() (*base.Repository, error) {
	var repo struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Public      bool   `json:"public"`
		Links       struct {
			bitbucketCloneLinks
			Self []struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	}
	if err := d.callAPI(fmt.Sprintf("projects/%s/repos/%s", d.project, d.repoSlug), nil, &repo); err != nil {
		return nil, err
	}

	cloneURL := repo.Links.httpURL()
	if cloneURL == "" {
		return nil, fmt.Errorf("no HTTP clone link for %s/%s: HTTP(S) SCM hosting is probably disabled on the Bitbucket instance", d.project, d.repoSlug)
	}

	var originalURL string
	if len(repo.Links.Self) > 0 {
		originalURL = repo.Links.Self[0].Href
	}

	// Default branch is absent (404) for empty repositories; warn on any other failure.
	var defaultBranch struct {
		DisplayID string `json:"displayId"`
	}
	if err := d.callAPI(fmt.Sprintf("projects/%s/repos/%s/branches/default", d.project, d.repoSlug), nil, &defaultBranch); err != nil && !errors.Is(err, errBitbucketNotFound) {
		log.Warn("Unable to get the default branch of %s/%s: %v", d.project, d.repoSlug, err)
	}

	name := repo.Slug
	if name == "" {
		name = d.repoSlug
	}

	return &base.Repository{
		Name:          name,
		Owner:         d.project,
		Description:   repo.Description,
		IsPrivate:     !repo.Public,
		CloneURL:      cloneURL,
		OriginalURL:   originalURL,
		DefaultBranch: defaultBranch.DisplayID,
	}, nil
}

// GetPullRequests returns pull requests of the repository, all states included.
// https://docs.atlassian.com/bitbucket-server/rest/latest/bitbucket-rest.html#idp304
func (d *BitbucketDataCenterDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	var resp struct {
		IsLastPage    bool `json:"isLastPage"`
		NextPageStart int  `json:"nextPageStart"`
		Values        []struct {
			ID          int64        `json:"id"`
			Title       string       `json:"title"`
			Description string       `json:"description"`
			State       string       `json:"state"` // OPEN, MERGED, DECLINED
			Draft       bool         `json:"draft"`
			Locked      bool         `json:"locked"`
			CreatedDate int64        `json:"createdDate"`
			UpdatedDate int64        `json:"updatedDate"`
			ClosedDate  int64        `json:"closedDate"`
			FromRef     bitbucketRef `json:"fromRef"`
			ToRef       bitbucketRef `json:"toRef"`
			Author      struct {
				User bitbucketUser `json:"user"`
			} `json:"author"`
			Properties struct {
				MergeCommit struct {
					ID string `json:"id"`
				} `json:"mergeCommit"`
			} `json:"properties"`
		} `json:"values"`
	}

	start := 0
	if page > 1 {
		start = d.prNextPageStart
	}
	query := url.Values{
		"state": {"ALL"},
		"start": {strconv.Itoa(start)},
		"limit": {strconv.Itoa(perPage)},
	}
	if err := d.callAPI(fmt.Sprintf("projects/%s/repos/%s/pull-requests", d.project, d.repoSlug), query, &resp); err != nil {
		return nil, false, err
	}
	d.prNextPageStart = resp.NextPageStart

	prs := make([]*base.PullRequest, 0, len(resp.Values))
	for _, pr := range resp.Values {
		state := "open"
		merged := pr.State == "MERGED"
		var closed, mergedTime *time.Time
		if pr.State != "OPEN" {
			state = "closed"
			// fall back to the last update.
			closedAt := time.UnixMilli(pr.UpdatedDate)
			if pr.ClosedDate > 0 {
				closedAt = time.UnixMilli(pr.ClosedDate)
			}
			closed = &closedAt
			if merged {
				mergedTime = &closedAt
			}
		}

		head := base.PullRequestBranch{
			Ref:       pr.FromRef.DisplayID,
			SHA:       pr.FromRef.LatestCommit,
			RepoName:  pr.FromRef.Repository.Slug,
			OwnerName: pr.FromRef.Repository.Project.Key,
		}
		if head.RepoFullName() != fmt.Sprintf("%s/%s", pr.ToRef.Repository.Project.Key, pr.ToRef.Repository.Slug) {
			// Fork pull request: the head branch must be fetched from the source repository.
			head.CloneURL = pr.FromRef.Repository.Links.httpURL()
		}

		result := &base.PullRequest{
			Number:         pr.ID,
			Title:          pr.Title,
			Content:        pr.Description,
			State:          state,
			IsDraft:        pr.Draft,
			IsLocked:       pr.Locked,
			PosterID:       pr.Author.User.ID,
			PosterName:     pr.Author.User.Name,
			PosterEmail:    pr.Author.User.EmailAddress,
			Created:        time.UnixMilli(pr.CreatedDate),
			Updated:        time.UnixMilli(pr.UpdatedDate),
			Closed:         closed,
			Merged:         merged,
			MergedTime:     mergedTime,
			MergeCommitSHA: pr.Properties.MergeCommit.ID,
			Head:           head,
			Base: base.PullRequestBranch{
				Ref:       pr.ToRef.DisplayID,
				SHA:       pr.ToRef.LatestCommit,
				RepoName:  pr.ToRef.Repository.Slug,
				OwnerName: pr.ToRef.Repository.Project.Key,
			},
			ForeignIndex: pr.ID,
		}

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(result, d.baseURL.String(), d)

		prCtx, err := d.fetchPullRequestActivities(pr.ID)
		if err != nil {
			return nil, false, err
		}
		result.Context = prCtx

		prs = append(prs, result)
	}

	return prs, resp.IsLastPage, nil
}

// bitbucketPRContext carries the comments and reviews of a pull request, collected once
// from its activity stream and served by GetComments/GetReviews.
type bitbucketPRContext struct {
	comments []*base.Comment
	reviews  []*base.Review
}

type bitbucketComment struct {
	ID          int64              `json:"id"`
	Text        string             `json:"text"`
	Author      bitbucketUser      `json:"author"`
	CreatedDate int64              `json:"createdDate"`
	UpdatedDate int64              `json:"updatedDate"`
	Comments    []bitbucketComment `json:"comments"`
}

// addCommentThread flattens a comment and its replies into plain comments; replies carry
// their parent id in Meta["ReplyTo"] so the uploader links them to the parent.
func (p *bitbucketPRContext) addCommentThread(prID int64, c bitbucketComment, replyTo int64, seen map[int64]bool) {
	if !seen[c.ID] {
		seen[c.ID] = true
		comment := &base.Comment{
			IssueIndex:  prID,
			Index:       c.ID,
			PosterID:    c.Author.ID,
			PosterName:  c.Author.Name,
			PosterEmail: c.Author.EmailAddress,
			Created:     time.UnixMilli(c.CreatedDate),
			Updated:     time.UnixMilli(c.UpdatedDate),
			Content:     c.Text,
		}
		if replyTo != 0 {
			comment.Meta = map[string]any{"ReplyTo": replyTo}
		}
		p.comments = append(p.comments, comment)
	}
	for _, child := range c.Comments {
		p.addCommentThread(prID, child, c.ID, seen)
	}
}

// addInlineThread converts an inline (diff-anchored) comment thread into a single review;
// every comment carries its own author so replies keep their attribution.
func (p *bitbucketPRContext) addInlineThread(prID int64, c bitbucketComment, treePath string, line, extraLines int64, commitID string, seen map[int64]bool) {
	// The uploader drops code comments without a diff hunk and rebuilds the displayed patch
	// from git; Bitbucket anchors carry no hunk text, so provide a minimal valid header.
	hunkLine := max(line, -line, 1)
	diffHunk := fmt.Sprintf("@@ -%d +%d @@", hunkLine, hunkLine)

	review := &base.Review{
		ID:           c.ID,
		IssueIndex:   prID,
		ReviewerID:   c.Author.ID,
		ReviewerName: c.Author.Name,
		State:        base.ReviewStateCommented,
		CreatedAt:    time.UnixMilli(c.CreatedDate),
	}

	var collect func(bitbucketComment)
	collect = func(cc bitbucketComment) {
		if !seen[cc.ID] {
			seen[cc.ID] = true
			review.Comments = append(review.Comments, &base.ReviewComment{
				ID:              cc.ID,
				Content:         cc.Text,
				TreePath:        treePath,
				Line:            int(line),
				ExtraLinesCount: extraLines,
				CommitID:        commitID,
				DiffHunk:        diffHunk,
				PosterID:        cc.Author.ID,
				PosterName:      cc.Author.Name,
				CreatedAt:       time.UnixMilli(cc.CreatedDate),
				UpdatedAt:       time.UnixMilli(cc.UpdatedDate),
			})
		}
		for _, child := range cc.Comments {
			collect(child)
		}
	}
	collect(c)

	if len(review.Comments) > 0 {
		p.reviews = append(p.reviews, review)
	}
}

type bitbucketActivity struct {
	ID            int64             `json:"id"`
	Action        string            `json:"action"`
	CommentAction string            `json:"commentAction"`
	CreatedDate   int64             `json:"createdDate"`
	User          bitbucketUser     `json:"user"`
	Comment       *bitbucketComment `json:"comment"`
	CommentAnchor *struct {
		Path     string `json:"path"`
		Line     int64  `json:"line"`
		FileType string `json:"fileType"` // FROM (old file) or TO (new file)
		ToHash   string `json:"toHash"`
		// Present when the comment spans a line range (Bitbucket 9.2+); Line is then the last line.
		MultilineMarker *struct {
			StartLine int64 `json:"startLine"`
		} `json:"multilineMarker"`
	} `json:"commentAnchor"`
}

// fetchPullRequestActivities collects comments, inline comments and reviews from the
// activity stream of a pull request.
// https://docs.atlassian.com/bitbucket-server/rest/latest/bitbucket-rest.html
func (d *BitbucketDataCenterDownloader) fetchPullRequestActivities(prID int64) (*bitbucketPRContext, error) {
	var activities []bitbucketActivity
	start := 0
	for {
		var resp struct {
			IsLastPage    bool                `json:"isLastPage"`
			NextPageStart int                 `json:"nextPageStart"`
			Values        []bitbucketActivity `json:"values"`
		}
		query := url.Values{
			"start": {strconv.Itoa(start)},
			"limit": {"100"},
		}
		if err := d.callAPI(fmt.Sprintf("projects/%s/repos/%s/pull-requests/%d/activities", d.project, d.repoSlug, prID), query, &resp); err != nil {
			return nil, err
		}
		activities = append(activities, resp.Values...)
		if resp.IsLastPage {
			break
		}
		start = resp.NextPageStart
	}

	prCtx := &bitbucketPRContext{}
	// A comment can surface both as its own activity and nested in the snapshot of its
	// thread's activity: keep each comment once.
	seen := make(map[int64]bool)

	// Activities are returned newest first; process oldest first so a thread is grouped on
	// its opening activity and comments stay chronological.
	for _, act := range slices.Backward(activities) {
		switch act.Action {
		case "APPROVED":
			prCtx.reviews = append(prCtx.reviews, &base.Review{
				ID:           act.ID,
				IssueIndex:   prID,
				ReviewerID:   act.User.ID,
				ReviewerName: act.User.Name,
				State:        base.ReviewStateApproved,
				CreatedAt:    time.UnixMilli(act.CreatedDate),
			})
		case "REVIEWED": // the reviewer marked the pull request as "needs work"
			prCtx.reviews = append(prCtx.reviews, &base.Review{
				ID:           act.ID,
				IssueIndex:   prID,
				ReviewerID:   act.User.ID,
				ReviewerName: act.User.Name,
				State:        base.ReviewStateChangesRequested,
				CreatedAt:    time.UnixMilli(act.CreatedDate),
			})
		case "UNAPPROVED": // the reviewer withdrew an earlier approval or "needs work": drop it
			for i, r := range slices.Backward(prCtx.reviews) {
				if r.ReviewerID == act.User.ID && (r.State == base.ReviewStateApproved || r.State == base.ReviewStateChangesRequested) {
					prCtx.reviews = slices.Delete(prCtx.reviews, i, i+1)
					break
				}
			}
		case "COMMENTED":
			if act.Comment == nil || act.CommentAction != "ADDED" {
				continue
			}
			if act.CommentAnchor != nil {
				line := act.CommentAnchor.Line
				extraLines := int64(0)
				// Bitbucket anchors a range on its last line, Forgejo on its first.
				if m := act.CommentAnchor.MultilineMarker; m != nil && m.StartLine > 0 && m.StartLine <= line {
					extraLines = line - m.StartLine
					line = m.StartLine
				}
				// Negative line means the old (left) side of the diff.
				if act.CommentAnchor.FileType == "FROM" {
					line = -line
				}
				prCtx.addInlineThread(prID, *act.Comment, act.CommentAnchor.Path, line, extraLines, act.CommentAnchor.ToHash, seen)
			} else {
				prCtx.addCommentThread(prID, *act.Comment, 0, seen)
			}
		}
	}

	return prCtx, nil
}

// GetComments returns the comments of a pull request, collected with its activities.
func (d *BitbucketDataCenterDownloader) GetComments(commentable base.Commentable) ([]*base.Comment, bool, error) {
	prCtx, ok := commentable.GetContext().(*bitbucketPRContext)
	if !ok {
		return nil, false, fmt.Errorf("unexpected context: %+v", commentable.GetContext())
	}
	return prCtx.comments, true, nil
}

// SupportCommentReplyTo return true: replies carry their parent id in Meta["ReplyTo"].
func (d *BitbucketDataCenterDownloader) SupportCommentReplyTo() bool {
	return true
}

// GetReviews returns the reviews (approvals, change requests and inline comments) of a
// pull request, collected with its activities.
func (d *BitbucketDataCenterDownloader) GetReviews(reviewable base.Reviewable) ([]*base.Review, error) {
	commentable, ok := reviewable.(base.Commentable)
	if !ok {
		return nil, fmt.Errorf("unexpected reviewable: %+v", reviewable)
	}
	prCtx, ok := commentable.GetContext().(*bitbucketPRContext)
	if !ok {
		return nil, fmt.Errorf("unexpected context: %+v", commentable.GetContext())
	}
	return prCtx.reviews, nil
}
