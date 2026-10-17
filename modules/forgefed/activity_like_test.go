// Copyright 2023, 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgefed_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"forgejo.org/modules/forgefed"
	"forgejo.org/modules/validation"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewForgeLike(t *testing.T) {
	want := []byte(`{"type":"Like","startTime":"2024-03-07T00:00:00Z","actor":"https://repo.prod.meissa.de/api/v1/activitypub/user-id/1","object":"https://codeberg.org/api/v1/activitypub/repository-id/1"}`)

	actorIRI := "https://repo.prod.meissa.de/api/v1/activitypub/user-id/1"
	objectIRI := "https://codeberg.org/api/v1/activitypub/repository-id/1"
	startTime, _ := time.Parse("2006-Jan-02", "2024-Mar-07")
	sut, err := forgefed.NewForgeLike(actorIRI, objectIRI, startTime)
	require.NoError(t, err, "unexpected error: %v", err)

	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)

	got, err := sut.MarshalJSON()
	require.NoError(t, err, "MarshalJSON() error = %q", err)
	assert.True(t, reflect.DeepEqual(got, want), "MarshalJSON()\n got: %q,\n want: %q", got, want)
}

func Test_LikeMarshalJSON(t *testing.T) {
	type testPair struct {
		item    forgefed.ForgeLike
		want    []byte
		wantErr error
	}

	tests := map[string]testPair{
		"empty": {
			item: forgefed.ForgeLike{},
			want: nil,
		},
		"with ID": {
			item: forgefed.ForgeLike{
				Activity: ap.Activity{
					Actor:  ap.IRI("https://repo.prod.meissa.de/api/v1/activitypub/user-id/1"),
					Type:   ap.LikeType,
					Object: ap.IRI("https://codeberg.org/api/v1/activitypub/repository-id/1"),
				},
			},
			want: []byte(`{"type":"Like","actor":"https://repo.prod.meissa.de/api/v1/activitypub/user-id/1","object":"https://codeberg.org/api/v1/activitypub/repository-id/1"}`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.item.MarshalJSON()
			assert.False(t, (err != nil || tt.wantErr != nil) && tt.wantErr.Error() != err.Error(), "MarshalJSON()\n got: %v,\n want: %v", err, tt.wantErr)
			assert.True(t, reflect.DeepEqual(got, tt.want), "MarshalJSON()\n got: %q\n want: %q", got, tt.want)
		})
	}
}

func Test_LikeUnmarshalJSON(t *testing.T) {
	type testPair struct {
		item    []byte
		want    *forgefed.ForgeLike
		wantErr error
	}

	tests := map[string]testPair{
		"with ID": {
			item: []byte(`{"type":"Like","actor":"https://repo.prod.meissa.de/api/activitypub/user-id/1","object":"https://codeberg.org/api/activitypub/repository-id/1"}`),
			want: &forgefed.ForgeLike{
				Activity: ap.Activity{
					Type:   ap.LikeType,
					Actor:  ap.IRI("https://repo.prod.meissa.de/api/activitypub/user-id/1"),
					Object: ap.IRI("https://codeberg.org/api/activitypub/repository-id/1"),
				},
			},
			wantErr: nil,
		},
		"invalid": {
			item:    []byte(`{"type":"Invalid","actor":"https://repo.prod.meissa.de/api/activitypub/user-id/1","object":"https://codeberg.org/api/activitypub/repository-id/1"`),
			want:    &forgefed.ForgeLike{},
			wantErr: errors.New("cannot parse JSON"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := new(forgefed.ForgeLike)
			err := got.UnmarshalJSON(test.item)
			assert.False(t, (err != nil || test.wantErr != nil) && !strings.Contains(err.Error(), test.wantErr.Error()), "UnmarshalJSON()\n error: %v\n wantErr: %v", err, test.wantErr)

			if !reflect.DeepEqual(got, test.want) {
				assert.Errorf(t, err, "UnmarshalJSON() got = %q, want %q, err %q", got, test.want, err.Error())
			}
		})
	}
}

func Test_ForgeLikeValidation(t *testing.T) {
	// Successful
	sut := new(forgefed.ForgeLike)
	sut.UnmarshalJSON([]byte(`{"type":"Like",
	"actor":"https://repo.prod.meissa.de/api/activitypub/user-id/1",
	"object":"https://codeberg.org/api/activitypub/repository-id/1",
	"startTime": "2014-12-31T23:00:00-08:00"}`))
	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)

	// Errors
	sut.UnmarshalJSON([]byte(`{"actor":"https://repo.prod.meissa.de/api/activitypub/user-id/1",
	"object":"https://codeberg.org/api/activitypub/repository-id/1",
	"startTime": "2014-12-31T23:00:00-08:00"}`))
	validate := sut.Validate()
	assert.Len(t, validate, 2)
	assert.Equal(t,
		"Field type contains the value <nil>, which is not in allowed subset [Like]",
		validate[1])

	sut.UnmarshalJSON([]byte(`{"type":"bad-type",
		"actor":"https://repo.prod.meissa.de/api/activitypub/user-id/1",
	"object":"https://codeberg.org/api/activitypub/repository-id/1",
	"startTime": "2014-12-31T23:00:00-08:00"}`))
	validate = sut.Validate()
	assert.Len(t, validate, 1)
	assert.Equal(t,
		"Field type contains the value bad-type, which is not in allowed subset [Like]",
		validate[0])

	sut.UnmarshalJSON([]byte(`{"type":"Like",
		"actor":"https://repo.prod.meissa.de/api/activitypub/user-id/1",
	  "object":"https://codeberg.org/api/activitypub/repository-id/1",
	  "startTime": "not a date"}`))
	validate = sut.Validate()
	assert.Len(t, validate, 1)
	assert.Equal(t,
		"StartTime was invalid.",
		validate[0])
}

func TestActivityValidation_Attack(t *testing.T) {
	sut := new(forgefed.ForgeLike)
	sut.UnmarshalJSON([]byte(`{rubbish}`))
	assert.Len(t, sut.Validate(), 5)
}
