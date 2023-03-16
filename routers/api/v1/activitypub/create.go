// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"

	ap "github.com/go-ap/activitypub"
)

// Create a new federated user from a Person object
func createPerson(ctx context.Context, person *ap.Person) error {
	_, err := user_model.GetUserByIRI(ctx, person.GetLink().String())
	if !user_model.IsErrUserNotExist(err) {
		// User already exists
		return err
	}

	personIRISplit := strings.Split(person.GetLink().String(), "/")
	if len(personIRISplit) < 4 {
		return errors.New("not a Person actor IRI")
	}

	// Get instance by taking the domain of the IRI
	instance := personIRISplit[2]
	if instance == setting.Domain {
		// Local user
		return nil
	}

	// Send a WebFinger request to get the username
	resp, err := activitypub.Fetch("https://" + instance + "/.well-known/webfinger?resource=" + person.GetLink().String())
	if err != nil {
		return err
	}
	var data activitypub.WebfingerJRD
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return err
	}
	subjectSplit := strings.Split(data.Subject, ":")
	if subjectSplit[0] != "acct" {
		return errors.New("subject is not an acct URI")
	}
	name := subjectSplit[1]

	var email string
	if person.Location != nil {
		email = person.Location.GetLink().String()
	} else {
		// This might not even work
		email = strings.ReplaceAll(name, "@", "+") + "@" + setting.Service.NoReplyAddress
	}

	if person.PublicKey.PublicKeyPem == "" {
		return errors.New("person public key not found")
	}

	user := &user_model.User{
		Name:                         name,
		Email:                        email,
		LoginType:                    auth.Federated,
		LoginName:                    person.GetLink().String(),
		EmailNotificationsPreference: user_model.EmailNotificationsDisabled,
	}
	if person.Name != nil {
		user.FullName = person.Name.String()
	}
	err = user_model.CreateUser(ctx, user)
	if err != nil {
		return err
	}

	if person.Icon != nil {
		// Fetch and save user icon
		icon, err := ap.ToObject(person.Icon)
		if err != nil {
			return err
		}
		body, err := activitypub.Fetch(icon.URL.GetLink().String())
		if err != nil {
			return err
		}
		err = user_service.UploadAvatar(ctx, user, body)
		if err != nil {
			return err
		}
	}

	err = user_model.SetUserSetting(ctx, user.ID, user_model.UserActivityPubPrivPem, "")
	if err != nil {
		return err
	}
	// Set public key
	return user_model.SetUserSetting(ctx, user.ID, user_model.UserActivityPubPubPem, person.PublicKey.PublicKeyPem)
}

// Create a new federated user from a Person IRI
func createPersonFromIRI(ctx context.Context, personIRI ap.IRI) error {
	object, err := activitypub.FetchObject(personIRI.String())
	if err != nil {
		return err
	}
	return ap.OnActor(object, func(p *ap.Person) error {
		return createPerson(ctx, p)
	})
}

// Create a new federated repo from a Repository object
func createRepository(ctx context.Context, repository *forgefed.Repository) error {
	user, err := user_model.GetUserByIRI(ctx, repository.AttributedTo.GetLink().String())
	if user_model.IsErrUserNotExist(err) {
		// TODO: This should probably return the created user too
		err := createPersonFromIRI(ctx, repository.AttributedTo.GetLink())
		if err != nil {
			return err
		}
		user, err = user_model.GetUserByIRI(ctx, repository.AttributedTo.GetLink().String())
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Check if repo exists
	_, err = repo_model.GetRepositoryByIRI(ctx, repository.GetLink().String())
	if !repo_model.IsErrRepoNotExist(err) {
		return err
	}

	repo, err := repo_service.CreateRepository(ctx, user, user, repo_service.CreateRepoOptions{
		Name:        repository.Name.String(),
		OriginalURL: repository.GetLink().String(),
	})
	if err != nil {
		return err
	}

	if repository.ForkedFrom != nil {
		repo.IsFork = true
		forkedFrom, err := repo_model.GetRepositoryByIRI(ctx, repository.ForkedFrom.GetLink().String())
		if err != nil {
			return err
		}
		repo.ForkID = forkedFrom.ID
	}
	return nil
}

// Create a new federated repo from a Repository IRI
func createRepositoryFromIRI(ctx context.Context, repoIRI ap.IRI) error {
	object, err := activitypub.FetchObject(repoIRI.String())
	if err != nil {
		return err
	}
	return forgefed.OnRepository(object, func(r *forgefed.Repository) error {
		return createRepository(ctx, r)
	})
}

// Create a ticket
func createTicket(ctx context.Context, ticket *forgefed.Ticket) error {
	if ticket.Origin != nil && ticket.Target != nil {
		return createPullRequest(ctx, ticket)
	}
	return createIssue(ctx, ticket)
}

// Create an issue
func createIssue(ctx context.Context, ticket *forgefed.Ticket) error {
	// TODO: don't call this function here
	err := createRepositoryFromIRI(ctx, ticket.Context.GetLink())
	if err != nil {
		return err
	}

	// Construct issue
	user, err := user_model.GetUserByIRI(ctx, ticket.AttributedTo.GetLink().String())
	if err != nil {
		return err
	}
	repo, err := repo_model.GetRepositoryByIRI(ctx, ticket.Context.GetLink().String())
	if err != nil {
		return err
	}
	idx, err := strconv.ParseInt(ticket.Name.String()[1:], 10, 64)
	if err != nil {
		return err
	}
	issue := &issues_model.Issue{
		Index:          idx, // TODO: This doesn't seem to work?
		RepoID:         repo.ID,
		Repo:           repo,
		Title:          ticket.Summary.String(),
		PosterID:       user.ID,
		Poster:         user,
		Content:        ticket.Content.String(),
		OriginalAuthor: ticket.GetLink().String(), // Create new database field to store IRI?
		IsClosed:       ticket.IsResolved,
	}
	return issue_service.NewIssue(ctx, repo, issue, nil, nil, nil)
}

// Create a pull request
func createPullRequest(ctx context.Context, ticket *forgefed.Ticket) error {
	// TODO: don't call this function here
	err := createRepositoryFromIRI(ctx, ticket.Context.GetLink())
	if err != nil {
		return err
	}
	user, err := user_model.GetUserByIRI(ctx, ticket.AttributedTo.GetLink().String())
	if err != nil {
		return err
	}

	// Extract origin and target repos
	originUsername, originReponame, originBranch, err := activitypub.BranchIRIToName(ticket.Origin.GetLink())
	if err != nil {
		return err
	}
	originRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, originUsername, originReponame)
	if err != nil {
		return err
	}
	targetUsername, targetReponame, targetBranch, err := activitypub.BranchIRIToName(ticket.Target.GetLink())
	if err != nil {
		return err
	}
	targetRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, targetUsername, targetReponame)
	if err != nil {
		return err
	}

	idx, err := strconv.ParseInt(ticket.Name.String()[1:], 10, 64)
	if err != nil {
		return err
	}
	prIssue := &issues_model.Issue{
		Index:    idx,
		RepoID:   targetRepo.ID,
		Title:    ticket.Summary.String(),
		PosterID: user.ID,
		Poster:   user,
		IsPull:   true,
		Content:  ticket.Content.String(),
		IsClosed: ticket.IsResolved,
	}
	pr := &issues_model.PullRequest{
		HeadRepoID: originRepo.ID,
		BaseRepoID: targetRepo.ID,
		HeadBranch: originBranch,
		BaseBranch: targetBranch,
		HeadRepo:   originRepo,
		BaseRepo:   targetRepo,
		MergeBase:  "",
		Type:       issues_model.PullRequestGitea,
	}
	return pull_service.NewPullRequest(ctx, targetRepo, prIssue, []int64{}, []string{}, pr, []int64{})
}

// Create a comment
func createComment(ctx context.Context, note *ap.Note) error {
	// Make sure repo exists
	user, err := user_model.GetUserByIRI(ctx, note.AttributedTo.GetLink().String())
	if err != nil {
		return err
	}

	username, reponame, idx, err := activitypub.TicketIRIToName(note.Context.GetLink())
	if err != nil {
		return err
	}
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, username, reponame)
	if err != nil {
		return err
	}
	issue, err := issues_model.GetIssueByIndex(ctx, repo.ID, idx)
	if err != nil {
		return err
	}
	_, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Doer:     user,
		Repo:     repo,
		Issue:    issue,
		OldTitle: note.GetLink().String(),
		Content:  note.Content.String(),
	})
	return err
}
