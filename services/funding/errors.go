// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package funding

import (
	"fmt"
	"regexp"
	"strings"

	repo_model "forgejo.org/models/repo"
)

// ErrFundingNotExist occurs when a repo has no funding config.
type ErrFundingNotExist struct {
	Repo *repo_model.Repository
}

func (err ErrFundingNotExist) Error() string {
	return fmt.Sprintf("No funding config found in repo %s/%s", err.Repo.OwnerName, err.Repo.Name)
}

func (ErrFundingNotExist) Is(err error) bool {
	_, ok := err.(ErrFundingNotExist)
	return ok
}

// ErrUnknownFundingProvider occurs when a funding config contains an unknown
// funding provider name.
type ErrUnknownFundingProvider struct {
	Name string
}

func (err ErrUnknownFundingProvider) Error() string {
	return fmt.Sprintf("Unknown funding provider: %s", err.Name)
}

func (ErrUnknownFundingProvider) Is(err error) bool {
	_, ok := err.(ErrUnknownFundingProvider)
	return ok
}

// ErrTooManyFundingProviders occurs when a funding config contains more
// funding entries than expected.
type ErrTooManyFundingProviders struct {
	TotalLimit int
}

func (err ErrTooManyFundingProviders) Error() string {
	return fmt.Sprintf("Expected up to %d funding providers", err.TotalLimit)
}

func (ErrTooManyFundingProviders) Is(err error) bool {
	_, ok := err.(ErrTooManyFundingProviders)
	return ok
}

// ErrDuplicateFundingEntry occurs when a funding config contains a provider
// with duplicate entries.
type ErrDuplicateFundingEntry struct {
	Name  string
	Value string
}

func (err ErrDuplicateFundingEntry) Error() string {
	return fmt.Sprintf("Duplicate entry for key \"%s\": %s", err.Name, err.Value)
}

func (ErrDuplicateFundingEntry) Is(err error) bool {
	_, ok := err.(ErrDuplicateFundingEntry)
	return ok
}

// ErrBadInput represents a failure to match the input string against the regex
// pattern.
type ErrBadInput struct {
	Name    string
	Pattern *regexp.Regexp
}

func (err ErrBadInput) Error() string {
	return fmt.Sprintf("Value for key \"%s\" does not match pattern /%s/", err.Name, err.Pattern.String())
}

func (ErrBadInput) Is(err error) bool {
	_, ok := err.(ErrBadInput)
	return ok
}

// ErrCannotParseURL represents a failure to parse a funding entry value as a URL.
type ErrCannotParseURL struct {
	Name string
	Err  error
}

func (err ErrCannotParseURL) Error() string {
	return fmt.Sprintf("Invalid URL value for key \"%s\": %v", err.Name, err.Err.Error())
}

func (ErrCannotParseURL) Is(err error) bool {
	_, ok := err.(ErrCannotParseURL)
	return ok
}

// ErrBadURLScheme occurs when a URL scheme is not in a list of valid schemes.
type ErrBadURLScheme struct {
	GivenScheme  string
	ValidSchemes []string
}

func (err ErrBadURLScheme) Error() string {
	valid := strings.Join(err.ValidSchemes, ", ")
	return fmt.Sprintf("invalid scheme \"%s\", expected one of: %s", err.GivenScheme, valid)
}

func (ErrBadURLScheme) Is(err error) bool {
	_, ok := err.(ErrBadURLScheme)
	return ok
}

// ErrInvalidYamlType occurs when a funding config is misshaped.
type ErrInvalidYamlType struct {
	Name string
}

func (err ErrInvalidYamlType) Error() string {
	return fmt.Sprintf("Invalid type for key \"%s\", expected a string or string array", err.Name)
}

func (ErrInvalidYamlType) Is(err error) bool {
	_, ok := err.(ErrInvalidYamlType)
	return ok
}
