// Copyright 2023, 2024, 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgefed_test

import (
	"reflect"
	"strings"
	"testing"

	"forgejo.org/modules/forgefed"
	"forgejo.org/modules/validation"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPersonIdFromModel(t *testing.T) {
	expected := forgefed.PersonID{}
	expected.ID = "1"
	expected.Source = "forgejo"
	expected.HostSchema = "https"
	expected.Path = "api/v1/activitypub/user-id"
	expected.Host = "an.other.host"
	expected.HostPort = 443
	expected.IsPortSupplemented = false
	expected.UnvalidatedInput = "https://an.other.host:443/api/v1/activitypub/user-id/1"

	sut, _ := forgefed.NewPersonIDFromModel("an.other.host", "https", 443, "forgejo", "1")
	assert.Equal(t, expected, sut)
}

func TestNewPersonId(t *testing.T) {
	var sut, expected forgefed.PersonID
	var err error

	expected = forgefed.PersonID{}
	expected.ID = "1"
	expected.Source = "forgejo"
	expected.HostSchema = "https"
	expected.Path = "api/v1/activitypub/user-id"
	expected.Host = "an.other.host"
	expected.HostPort = 443
	expected.IsPortSupplemented = true
	expected.UnvalidatedInput = "https://an.other.host/api/v1/activitypub/user-id/1"

	sut, err = forgefed.NewPersonID("https://an.other.host/api/v1/activitypub/user-id/1", "forgejo")
	require.NoError(t, err)
	assert.Equal(t, expected, sut)

	expected = forgefed.PersonID{}
	expected.ID = "1"
	expected.Source = "forgejo"
	expected.HostSchema = "https"
	expected.Path = "api/v1/activitypub/user-id"
	expected.Host = "an.other.host"
	expected.HostPort = 443
	expected.IsPortSupplemented = false
	expected.UnvalidatedInput = "https://an.other.host:443/api/v1/activitypub/user-id/1"

	sut, _ = forgefed.NewPersonID("https://an.other.host:443/api/v1/activitypub/user-id/1", "forgejo")
	assert.Equal(t, expected, sut)

	expected = forgefed.PersonID{}
	expected.ID = "1"
	expected.Source = "forgejo"
	expected.HostSchema = "http"
	expected.Path = "api/v1/activitypub/user-id"
	expected.Host = "an.other.host"
	expected.HostPort = 80
	expected.IsPortSupplemented = false
	expected.UnvalidatedInput = "http://an.other.host:80/api/v1/activitypub/user-id/1"

	sut, _ = forgefed.NewPersonID("http://an.other.host:80/api/v1/activitypub/user-id/1", "forgejo")
	assert.Equal(t, expected, sut)

	expected = forgefed.PersonID{}
	expected.ID = "1"
	expected.Source = "forgejo"
	expected.HostSchema = "https"
	expected.Path = "api/v1/activitypub/user-id"
	expected.Host = "an.other.host"
	expected.HostPort = 443
	expected.IsPortSupplemented = false
	expected.UnvalidatedInput = "https://an.other.host:443/api/v1/activitypub/user-id/1"

	sut, _ = forgefed.NewPersonID("HTTPS://an.other.host:443/api/v1/activitypub/user-id/1", "forgejo")
	assert.Equal(t, expected, sut)

	expected = forgefed.PersonID{}
	expected.ID = "@me"
	expected.Source = "gotosocial"
	expected.HostSchema = "https"
	expected.Path = ""
	expected.Host = "an.other.host"
	expected.HostPort = 443
	expected.IsPortSupplemented = true
	expected.UnvalidatedInput = "https://an.other.host/@me"

	sut, err = forgefed.NewPersonID("https://an.other.host/@me", "gotosocial")
	require.NoError(t, err)
	assert.Equal(t, expected, sut)
}

func TestPersonIdValidation(t *testing.T) {
	sut := forgefed.PersonID{}
	sut.ID = "1"
	sut.Source = "forgejo"
	sut.HostSchema = "https"
	sut.Path = ""
	sut.Host = "an.other.host"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://an.other.host/1"

	result, err := validation.IsValid(sut)
	assert.False(t, result)
	require.EqualError(t, err, "Validation Error: forgefed.PersonID: Value path should not be empty\npath: \"\" has to be a person specific api path")

	sut = forgefed.PersonID{}
	sut.ID = "1"
	sut.Source = "mastodon"
	sut.HostSchema = "https"
	sut.Path = ""
	sut.Host = "an.other.host"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://an.other.host/1"

	result, err = validation.IsValid(sut)
	assert.True(t, result)
	require.NoError(t, err)

	sut = forgefed.PersonID{}
	sut.ID = "1"
	sut.Source = "forgejo"
	sut.HostSchema = "https"
	sut.Path = "path"
	sut.Host = "an.other.host"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://an.other.host/path/1"

	result, err = validation.IsValid(sut)
	assert.False(t, result)
	require.EqualError(t, err, "Validation Error: forgefed.PersonID: path: \"path\" has to be a person specific api path")

	sut = forgefed.PersonID{}
	sut.ID = "1"
	sut.Source = "forgejox"
	sut.HostSchema = "https"
	sut.Path = "api/v1/activitypub/user-id"
	sut.Host = "an.other.host"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://an.other.host/api/v1/activitypub/user-id/1"

	result, err = validation.IsValid(sut)
	assert.False(t, result)
	require.EqualError(t, err, "Validation Error: forgefed.PersonID: Field Source contains the value forgejox, which is not in allowed subset [forgejo gitea mastodon gotosocial]")

	sut = forgefed.PersonID{}
	sut.ID = "actor"
	sut.Source = "forgejo"
	sut.HostSchema = "https"
	sut.Path = "api/v1/activitypub"
	sut.Host = "example.com"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://example.com/api/v1/activitypub/actor"

	result, err = validation.IsValid(sut)
	assert.True(t, result)
	require.NoError(t, err)

	sut = forgefed.PersonID{}
	sut.ID = "actor"
	sut.Source = "forgejo"
	sut.HostSchema = "https"
	sut.Path = "api/activitypub"
	sut.Host = "example.com"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://example.com/api/activitypub/actor"

	result, err = validation.IsValid(sut)
	assert.True(t, result)
	require.NoError(t, err)

	sut = forgefed.PersonID{}
	sut.ID = "1"
	sut.Source = "forgejo"
	sut.HostSchema = "https"
	sut.Path = "api/v1/activitypub"
	sut.Host = "example.com"
	sut.HostPort = 443
	sut.IsPortSupplemented = true
	sut.UnvalidatedInput = "https://example.com/api/v1/activitypub/1"

	result, err = validation.IsValid(sut)
	assert.False(t, result)
	require.EqualError(t, err, "Validation Error: forgefed.PersonID: path: \"api/v1/activitypub\" has to be a person specific api path")
}

func TestWebfingerId(t *testing.T) {
	sut, _ := forgefed.NewPersonID("https://codeberg.org/api/v1/activitypub/user-id/12345", "forgejo")
	assert.Equal(t, "@12345@codeberg.org", sut.AsWebfinger())
}

func TestShouldThrowErrorOnInvalidInput(t *testing.T) {
	tests := []struct {
		input     string
		username  string
		expectErr bool
	}{
		{"", "forgejo", true},
		{"http://localhost:3000/api/v1/something", "forgejo", true},
		{"./api/v1/something", "forgejo", true},
		{"http://1.2.3.4/api/v1/something", "forgejo", true},
		{"http:///[fe80::1ff:fe23:4567:890a%25eth0]/api/v1/something", "forgejo", true},
		{"https://codeberg.org/api/v1/activitypub/../activitypub/user-id/12345", "forgejo", true},
		{"https://myuser@an.other.host/api/v1/activitypub/user-id/1", "forgejo", true},
		{"https://an.other.host/api/v1/activitypub/user-id/1", "forgejo", false},
	}

	for _, tt := range tests {
		_, err := forgefed.NewPersonID(tt.input, tt.username)
		if tt.expectErr {
			assert.Error(t, err, "Expected an error for input: %s", tt.input)
		} else {
			assert.NoError(t, err, "Expected no error for input: %s, but got: %v", tt.input, err)
		}
	}
}

func Test_PersonMarshalJSON(t *testing.T) {
	sut := forgefed.ForgePerson{}
	sut.Type = ap.PersonType
	sut.PreferredUsername = ap.NaturalLanguageValuesNew()
	sut.PreferredUsername.Set(ap.English, ap.Content("MaxMuster"))
	result, _ := sut.MarshalJSON()
	assert.JSONEq(t, `{"type":"Person","preferredUsername":"MaxMuster"}`, string(result), "Expected string is not equal")
}

func Test_PersonUnmarshalJSON(t *testing.T) {
	expected := &forgefed.ForgePerson{
		Actor: ap.Actor{
			Type: ap.PersonType,
			PreferredUsername: ap.NaturalLanguageValues{
				ap.English: []byte("MaxMuster"),
			},
		},
	}
	sut := new(forgefed.ForgePerson)
	err := sut.UnmarshalJSON([]byte(`{"type":"Person","preferredUsername":"MaxMuster"}`))
	require.NoError(t, err, "UnmarshalJSON() unexpected error: %q", err)

	x, _ := expected.MarshalJSON()
	y, _ := sut.MarshalJSON()
	assert.True(t, reflect.DeepEqual(x, y), "UnmarshalJSON()\n got: %q,\n want: %q", x, y)

	expectedStr := strings.ReplaceAll(strings.ReplaceAll(`{
		"id":"https://federated-repo.prod.meissa.de/api/v1/activitypub/user-id/10",
		"type":"Person",
		"icon":{"type":"Image","mediaType":"image/png","url":"https://federated-repo.prod.meissa.de/avatar/fa7f9c4af2a64f41b1bef292bf872614"},
		"url":"https://federated-repo.prod.meissa.de/stargoose9",
		"inbox":"https://federated-repo.prod.meissa.de/api/v1/activitypub/user-id/10/inbox",
		"outbox":"https://federated-repo.prod.meissa.de/api/v1/activitypub/user-id/10/outbox",
		"preferredUsername":"stargoose9",
		"publicKey":{"id":"https://federated-repo.prod.meissa.de/api/v1/activitypub/user-id/10#main-key",
			"owner":"https://federated-repo.prod.meissa.de/api/v1/activitypub/user-id/10",
			"publicKeyPem":"-----BEGIN PUBLIC KEY-----\nMIIBoj...XAgMBAAE=\n-----END PUBLIC KEY-----\n"}}`,
		"\n", ""),
		"\t", "")
	err = sut.UnmarshalJSON([]byte(expectedStr))
	require.NoError(t, err, "UnmarshalJSON() unexpected error: %q", err)
	result, _ := sut.MarshalJSON()
	assert.JSONEq(t, expectedStr, string(result), "Expected string is not equal")
}

func TestForgePersonValidation(t *testing.T) {
	sut := new(forgefed.ForgePerson)
	sut.UnmarshalJSON([]byte(`{"type":"Person","preferredUsername":"MaxMuster"}`))
	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)
}

func TestAsloginName(t *testing.T) {
	sut, _ := forgefed.NewPersonID("https://codeberg.org/api/v1/activitypub/user-id/12345", "forgejo")
	assert.Equal(t, "12345@codeberg.org", sut.AsLoginName())

	sut, _ = forgefed.NewPersonID("https://codeberg.org:443/api/v1/activitypub/user-id/12345", "forgejo")
	assert.Equal(t, "12345@codeberg.org:443", sut.AsLoginName())
}

func TestHostSuffix(t *testing.T) {
	sut, _ := forgefed.NewPersonID("https://codeberg.org/api/v1/activitypub/user-id/12345", "forgejo")
	sut.Host = "forgejo.example.tld"
	sut.HostPort = 80

	// sut.IsPortSupplemented is true by default at time of writing.
	assert.Equal(t, "@forgejo.example.tld", sut.HostSuffix())
	sut.IsPortSupplemented = false
	assert.Equal(t, "@forgejo.example.tld:80", sut.HostSuffix())
}
