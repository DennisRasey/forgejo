// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package funding

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	repo_model "forgejo.org/models/repo"
)

// NotExistError occurs when a repo has no funding config.
type NotExistError struct {
	Repo *repo_model.Repository
}

func (err NotExistError) Error() string {
	return fmt.Sprintf("No funding config found in repo %s/%s", err.Repo.OwnerName, err.Repo.Name)
}

func IsNotExistError(err error) bool {
	_, ok := errors.AsType[NotExistError](err)
	return ok
}

// UnknownProviderError occurs when a funding config contains an unknown
// funding provider name.
type UnknownProviderError struct {
	Name string
}

func (err UnknownProviderError) Error() string {
	return fmt.Sprintf("Unknown funding provider: %s", err.Name)
}

// TooManyProvidersError occurs when a funding config contains more
// funding entries than expected.
type TooManyProvidersError struct {
	TotalLimit int
}

func (err TooManyProvidersError) Error() string {
	return fmt.Sprintf("Expected up to %d funding providers", err.TotalLimit)
}

// DuplicateEntryError occurs when a funding config contains a provider
// with duplicate entries.
type DuplicateEntryError struct {
	Name  string
	Value string
}

func (err DuplicateEntryError) Error() string {
	return fmt.Sprintf("Duplicate entry for key \"%s\": %s", err.Name, err.Value)
}

// BadInputError represents a failure to match the input string against the regex
// pattern.
type BadInputError struct {
	Name    string
	Pattern *regexp.Regexp
}

func (err BadInputError) Error() string {
	return fmt.Sprintf("Value for key \"%s\" does not match pattern /%s/", err.Name, err.Pattern.String())
}

// CannotParseURLError represents a failure to parse a funding entry value as a URL.
type CannotParseURLError struct {
	Name string
	Err  error
}

func (err CannotParseURLError) Error() string {
	return fmt.Sprintf("Invalid URL value for key \"%s\": %v", err.Name, err.Err.Error())
}

// BadURLSchemeError occurs when a URL scheme is not in a list of valid schemes.
type BadURLSchemeError struct {
	GivenScheme  string
	ValidSchemes []string
}

func (err BadURLSchemeError) Error() string {
	valid := strings.Join(err.ValidSchemes, ", ")
	return fmt.Sprintf("invalid scheme \"%s\", expected one of: %s", err.GivenScheme, valid)
}

// InvalidYamlTypeError occurs when a funding config is misshaped.
type InvalidYamlTypeError struct {
	Name string
}

func (err InvalidYamlTypeError) Error() string {
	return fmt.Sprintf("Invalid type for key \"%s\", expected a string or string array", err.Name)
}

// Occurs when unmarshaling a YAML file with duplicate map keys.
type DuplicateYAMLKeyError struct {
	Key string
}

func (err DuplicateYAMLKeyError) Error() string {
	return fmt.Sprintf("Duplicate YAML key: %s", err.Key)
}

// Occurs when unmarshaling a YAML file and finding an unexpected non-map node.
type NotYAMLMappingError struct {
	ShortTag string
}

func (err NotYAMLMappingError) Error() string {
	return fmt.Sprintf("Expected YAML mapping, got %s", err.ShortTag)
}
