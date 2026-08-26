// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package funding

import (
	"reflect"
	"slices"

	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"

	"go.yaml.in/yaml/v3"
)

// Parses the given file data for funding entries.
func getFundingFromBlob(content []byte) (entries []*api.RepoFundingEntry, errs []error) {
	config := make(rawRepoFundingConfig, 0)
	if err := yaml.Unmarshal(content, &config); err != nil {
		errs = append(errs, err)
		return entries, errs
	}

	entryList, errs := processFundingConfig(config)
	return entryList, errs
}

// Sorts the raw config data into distinct funding entries, listing both valid
// entries and data errors.
func processFundingConfig(config rawRepoFundingConfig) ([]*api.RepoFundingEntry, []error) {
	entryList := make([]*api.RepoFundingEntry, 0)
	var errs []error

	// no need to sort these, they'll come in the order they were given
	limitReached := false
	for _, entry := range config {
		if limitReached {
			break // we've reached our limit, as reported by a downstream list processor (error already listed)
		}
		if len(entryList) >= setting.MaxFundingEntriesPerConfig {
			errs = append(errs, TooManyProvidersError{TotalLimit: setting.MaxFundingEntriesPerConfig})
			break // we've reached our limit; no point checking further, even for parse errors (there may be many!)
		}

		providerName := entry.Key
		provider := setting.GetFundingProviderByName(providerName)
		if provider == nil {
			errs = append(errs, UnknownProviderError{Name: providerName})
			continue
		}

		entryData := entry.Value
		if entryData == nil {
			errs = append(errs, InvalidYamlTypeError{Name: providerName})
			continue
		}

		dataType := reflect.TypeOf(entryData)
		switch dataType.Kind() {
		case reflect.String:
			value := entryData.(string)
			entryList, errs = processRowAsString(provider, value, entryList, errs)

		case reflect.Slice:
			// no need to sort these either, they'll come in the order they were given
			slice := entryData.([]any)
			entryList, errs, limitReached = processRowAsSlice(provider, slice, entryList, errs)

		default:
			errs = append(errs, InvalidYamlTypeError{Name: providerName})
			continue
		}
	}

	return entryList, errs
}

func processRowAsString(provider *setting.FundingProviderConfig, value string, entryList []*api.RepoFundingEntry, errs []error) ([]*api.RepoFundingEntry, []error) {
	newEntry, err := getFundingEntry(provider, value)
	if err != nil {
		errs = append(errs, err)
	} else {
		if slices.ContainsFunc(entryList, func(e *api.RepoFundingEntry) bool {
			return e.Value == newEntry.Value
		}) {
			errs = append(errs, DuplicateEntryError{Name: provider.Name, Value: newEntry.Value})
			return entryList, errs
		}
		entryList = append(entryList, newEntry)
	}
	return entryList, errs
}

func processRowAsSlice(provider *setting.FundingProviderConfig, slice []any, entryList []*api.RepoFundingEntry, errs []error) ([]*api.RepoFundingEntry, []error, bool) {
	for _, value := range slice {
		if len(entryList) >= setting.MaxFundingEntriesPerConfig {
			errs = append(errs, TooManyProvidersError{TotalLimit: setting.MaxFundingEntriesPerConfig})
			return entryList, errs, true // we've reached our limit; no point checking further, even for parse errors (there may be many!)
		}
		str, ok := value.(string)
		if !ok {
			errs = append(errs, InvalidYamlTypeError{Name: provider.Name})
		} else {
			entryList, errs = processRowAsString(provider, str, entryList, errs)
		}
	}
	return entryList, errs, false
}
