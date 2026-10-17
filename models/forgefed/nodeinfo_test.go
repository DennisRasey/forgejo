// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgefed

import (
	"errors"
	"reflect"
	"testing"

	"forgejo.org/modules/validation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NodeInfoWellKnownUnmarshalJSON(t *testing.T) {
	type testPair struct {
		item    []byte
		want    NodeInfoWellKnown
		wantErr error
	}

	tests := map[string]testPair{
		"with href": {
			item: []byte(`{"links":[{"href":"https://federated-repo.prod.meissa.de/api/v1/nodeinfo","rel":"http://nodeinfo.diaspora.software/ns/schema/2.1"}]}`),
			want: NodeInfoWellKnown{
				Href: "https://federated-repo.prod.meissa.de/api/v1/nodeinfo",
			},
		},
		"empty": {
			item:    []byte(``),
			wantErr: errors.New("cannot parse JSON: cannot parse empty string; unparsed tail: \"\""),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := NodeInfoWellKnownUnmarshalJSON(tt.item)
			if (err != nil || tt.wantErr != nil) && tt.wantErr.Error() != err.Error() {
				t.Errorf("UnmarshalJSON() error = \"%v\", wantErr \"%v\"", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalJSON() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_NodeInfoWellKnownValidate(t *testing.T) {
	sut := NodeInfoWellKnown{Href: "https://federated-repo.prod.meissa.de/api/v1/nodeinfo"}
	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)

	sut = NodeInfoWellKnown{Href: "./federated-repo.prod.meissa.de/api/v1/nodeinfo"}
	_, err = validation.IsValid(sut)
	require.ErrorContains(t, err, "Href has to be absolute\nField parsedURL.Scheme contains the value , which is not in allowed subset [http https]")
	assert.True(t, validation.IsErrNotValid(err), "validation error expected but was: %v", err)

	sut = NodeInfoWellKnown{Href: "https://federated-repo.prod.meissa.de/api/v1/nodeinfo"}
	_, err = validation.IsValid(sut)
	require.NoError(t, err)
}

func Test_NewNodeInfoWellKnown(t *testing.T) {
	sut, _ := NewNodeInfoWellKnown([]byte(`{"links":[{"href":"https://federated-repo.prod.meissa.de/api/v1/nodeinfo","rel":"http://nodeinfo.diaspora.software/ns/schema/2.1"}]}`))
	expected := NodeInfoWellKnown{Href: "https://federated-repo.prod.meissa.de/api/v1/nodeinfo"}
	assert.Equal(t, expected, sut)

	_, err := NewNodeInfoWellKnown([]byte(`invalid`))
	require.Error(t, err)
}

func Test_NewNodeInfo(t *testing.T) {
	sut, _ := NewNodeInfo([]byte(`{"version":"2.1","software":{"name":"gitea","version":"1.20.0+dev-2539-g5840cc6d3","repository":"https://github.com/go-gitea/gitea.git","homepage":"https://gitea.io/"},"protocols":["activitypub"],"services":{"inbound":[],"outbound":["rss2.0"]},"openRegistrations":true,"usage":{"users":{"total":13,"activeHalfyear":1,"activeMonth":1}},"metadata":{}}`))
	expected := NodeInfo{SoftwareName: "gitea"}
	assert.Equal(t, expected, sut)

	_, err := NewNodeInfo([]byte(`invalid`))
	require.Error(t, err)
}
