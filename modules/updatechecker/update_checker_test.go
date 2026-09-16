// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"errors"
	"slices"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/system"
	"forgejo.org/modules/test"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDNSUpdate(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		defer test.MockVariableValue(&lookupTXT, func(domain string) ([]string, error) {
			assert.Equal(t, "release.forgejo.org", domain)
			return []string{}, nil
		})()
		_, err := getVersionDNS("release.forgejo.org")
		require.ErrorContains(t, err, "no TXT record")
	})

	t.Run("unrelated TXT", func(t *testing.T) {
		defer test.MockVariableValue(&lookupTXT, func(domain string) ([]string, error) {
			assert.Equal(t, "release.forgejo.org", domain)
			return []string{
				"v=spf1 -all",
			}, nil
		})()
		_, err := getVersionDNS("release.forgejo.org")
		require.ErrorContains(t, err, "no TXT record")
	})

	t.Run("single response, single value", func(t *testing.T) {
		defer test.MockVariableValue(&lookupTXT, func(domain string) ([]string, error) {
			assert.Equal(t, "release.forgejo.org", domain)
			return []string{"forgejo_versions=16.0.3"}, nil
		})()
		versions, err := getVersionDNS("release.forgejo.org")
		require.NoError(t, err)
		assert.Equal(t, []string{"16.0.3"}, versions)
	})

	t.Run("single response, multiple values", func(t *testing.T) {
		defer test.MockVariableValue(&lookupTXT, func(domain string) ([]string, error) {
			assert.Equal(t, "release.forgejo.org", domain)
			return []string{"forgejo_versions=16.0.3, 15.0.7"}, nil
		})()
		versions, err := getVersionDNS("release.forgejo.org")
		require.NoError(t, err)
		assert.Equal(t, []string{"16.0.3", "15.0.7"}, versions)
	})

	t.Run("multiple responses, multiple values", func(t *testing.T) {
		defer test.MockVariableValue(&lookupTXT, func(domain string) ([]string, error) {
			assert.Equal(t, "blah.forgejo.org", domain)
			return []string{
				"forgejo_versions=16.0.3",
				"forgejo_versions=15.0.7",
			}, nil
		})()
		versions, err := getVersionDNS("blah.forgejo.org")
		require.NoError(t, err)
		assert.Equal(t, []string{"16.0.3", "15.0.7"}, versions)
	})
}

func TestUpdateRemoteVersion(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Set", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			cs, ok := item.(*CheckerState)
			require.True(t, ok)
			return cs.Name() == "update-checker" &&
				slices.Equal(cs.SupportedVersions, []string{"16.0.3", "15.0.7"})
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()

		require.NoError(t, UpdateRemoteVersion(t.Context(), []string{"16.0.3", "15.0.7"}, nil))
	})

	t.Run("success storing error", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Set", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			cs, ok := item.(*CheckerState)
			require.True(t, ok)
			return cs.Name() == "update-checker" &&
				cs.Error == "some error"
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()

		require.NoError(t, UpdateRemoteVersion(t.Context(), nil, errors.New("some error")))
	})

	t.Run("error", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Set", mock.Anything, mock.Anything).Return(errors.New("oh no"))
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		require.ErrorContains(t, UpdateRemoteVersion(t.Context(), []string{"16.0.3", "15.0.7"}, nil), "oh no")
	})
}

func TestGetReleaseState(t *testing.T) {
	t.Run("AppState get failure", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			_, ok := item.(*CheckerState)
			return ok
		})).Return(errors.New("something went wrong"))
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		retval, err := GetReleaseState(t.Context())
		require.ErrorContains(t, err, "retrieve update checker output: something went wrong")
		assert.Nil(t, retval)
	})

	t.Run("AppState get had no data, but no error", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			_, ok := item.(*CheckerState)
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "13.0.0")()
		retval, err := GetReleaseState(t.Context())
		require.NoError(t, err)
		assert.Equal(t, MajorReleaseSupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseCurrent, retval.MinorReleaseState)
		assert.Empty(t, retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("unparseable current version value ignored", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			state, ok := item.(*CheckerState)
			if ok {
				state.SupportedVersions = []string{"14.0.3"}
			}
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "this is not a version")()
		retval, err := GetReleaseState(t.Context())
		require.ErrorContains(t, err, "Malformed version: this is not a version")
		assert.Nil(t, retval)
	})

	t.Run("unparseable available version ignored", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			state, ok := item.(*CheckerState)
			if ok {
				state.SupportedVersions = []string{"this is not a version"}
			}
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "13.0.0")()
		retval, err := GetReleaseState(t.Context())
		require.ErrorContains(t, err, "failure to parse remote version \"this is not a version\": Malformed version")
		assert.Nil(t, retval)
	})

	t.Run("no upgrade if local prerelease", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			state, ok := item.(*CheckerState)
			if ok {
				state.SupportedVersions = []string{"16.0.0"}
			}
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "17.0.0")()
		retval, err := GetReleaseState(t.Context())
		require.NoError(t, err)
		assert.Equal(t, MajorReleasePrerelease, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseCurrent, retval.MinorReleaseState)
		assert.Empty(t, retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("recommends upgrade to current major release", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			state, ok := item.(*CheckerState)
			if ok {
				state.SupportedVersions = []string{
					"13.0.5",
					"14.0.6",
				}
			}
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "13.0.0")()
		retval, err := GetReleaseState(t.Context())
		require.NoError(t, err)
		assert.Equal(t, MajorReleaseSupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "13.0.5", retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("recommends upgrade to highest LTS", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			state, ok := item.(*CheckerState)
			if ok {
				state.SupportedVersions = []string{
					"11.0.24",
					"15.0.2",
					"16.0.0",
				}
			}
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "13.0.0")()
		retval, err := GetReleaseState(t.Context())
		require.NoError(t, err)
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "15.0.2", retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("fallback to highest version", func(t *testing.T) {
		state := system.NewMockStateStore(t)
		state.On("Get", mock.Anything, mock.MatchedBy(func(item system.StateItem) bool {
			state, ok := item.(*CheckerState)
			if ok {
				state.SupportedVersions = []string{
					"16.0.1",
					"17.0.1",
				}
			}
			return ok
		})).Return(nil)
		defer test.MockVariableValue[system.StateStore](&system.AppState, state)()
		defer test.MockVariableValue(&setting.AppVer, "13.0.0")()
		retval, err := GetReleaseState(t.Context())
		require.NoError(t, err)
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "17.0.1", retval.RecommendedUpgrade.ValueOrZeroValue())
	})
}

func TestCheckPreRelease(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		retval := tryCheckPrerelease(
			version.Must(version.NewVersion("14.0.3")),
			[]*version.Version{},
		)
		assert.Equal(t, MajorReleasePrerelease, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseCurrent, retval.MinorReleaseState)
		assert.Empty(t, retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("not prerelease", func(t *testing.T) {
		retval := tryCheckPrerelease(
			version.Must(version.NewVersion("14.0.3")),
			[]*version.Version{
				version.Must(version.NewVersion("14.0.3")),
			},
		)
		assert.Nil(t, retval)
	})

	t.Run("prerelease", func(t *testing.T) {
		retval := tryCheckPrerelease(
			version.Must(version.NewVersion("17.0.3")),
			[]*version.Version{
				version.Must(version.NewVersion("15.0.3")),
			},
		)
		assert.Equal(t, MajorReleasePrerelease, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseCurrent, retval.MinorReleaseState)
		assert.Empty(t, retval.RecommendedUpgrade.ValueOrZeroValue())
	})
}

func TestTryGetMatchingMajorRelease(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		retval := tryGetMatchingMajorRelease(version.Must(version.NewVersion("14.0.3")), []*version.Version{})
		assert.Nil(t, retval)
	})

	t.Run("single major same release", func(t *testing.T) {
		retval := tryGetMatchingMajorRelease(
			version.Must(version.NewVersion("14.0.3")),
			[]*version.Version{
				version.Must(version.NewVersion("14.0.3")),
				version.Must(version.NewVersion("15.0.3")),
			},
		)
		assert.Equal(t, MajorReleaseSupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseCurrent, retval.MinorReleaseState)
		assert.Empty(t, retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("single major upgrade", func(t *testing.T) {
		retval := tryGetMatchingMajorRelease(
			version.Must(version.NewVersion("14.0.3")),
			[]*version.Version{
				version.Must(version.NewVersion("14.0.4")),
				version.Must(version.NewVersion("15.0.3")),
			},
		)
		assert.Equal(t, MajorReleaseSupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "14.0.4", retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("multiple matching major upgrade", func(t *testing.T) {
		retval := tryGetMatchingMajorRelease(
			version.Must(version.NewVersion("14.0.3")),
			[]*version.Version{
				// Shouldn't happen, but if DNS TXT records are intermittently wrong or error occurs updating it, prefer
				// the highest release
				version.Must(version.NewVersion("14.0.4")),
				version.Must(version.NewVersion("14.0.5")),
			},
		)
		assert.Equal(t, MajorReleaseSupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "14.0.5", retval.RecommendedUpgrade.ValueOrZeroValue())
	})
}

func TestTryGetHighestLTSRelease(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		retval := tryGetHighestLTSRelease([]*version.Version{})
		assert.Nil(t, retval)
	})

	t.Run("no LTS", func(t *testing.T) {
		retval := tryGetHighestLTSRelease([]*version.Version{
			version.Must(version.NewVersion("14.0.3")),
		})
		assert.Nil(t, retval)
	})

	t.Run("single LTS", func(t *testing.T) {
		retval := tryGetHighestLTSRelease([]*version.Version{
			version.Must(version.NewVersion("15.0.3")),
		})
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "15.0.3", retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("multiple LTS", func(t *testing.T) {
		retval := tryGetHighestLTSRelease([]*version.Version{
			version.Must(version.NewVersion("15.0.13")),
			version.Must(version.NewVersion("19.0.0")),
		})
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "19.0.0", retval.RecommendedUpgrade.ValueOrZeroValue())
	})
}

func TestGetHighestRelease(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		retval := getHighestRelease([]*version.Version{
			version.Must(version.NewVersion("14.0.3")),
		})
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "14.0.3", retval.RecommendedUpgrade.ValueOrZeroValue())
	})

	t.Run("multiple", func(t *testing.T) {
		retval := getHighestRelease([]*version.Version{
			version.Must(version.NewVersion("14.0.3")),
			version.Must(version.NewVersion("15.0.3")),
		})
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "15.0.3", retval.RecommendedUpgrade.ValueOrZeroValue())

		retval = getHighestRelease([]*version.Version{
			version.Must(version.NewVersion("15.0.3")),
			version.Must(version.NewVersion("14.0.3")),
		})
		assert.Equal(t, MajorReleaseUnsupported, retval.MajorReleaseState)
		assert.Equal(t, MinorReleaseOutOfDate, retval.MinorReleaseState)
		assert.Equal(t, "15.0.3", retval.RecommendedUpgrade.ValueOrZeroValue())
	})
}
