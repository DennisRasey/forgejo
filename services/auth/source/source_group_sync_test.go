package source

import (
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/container"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDynGroupMapsCaseInsensitive tests NewDynGroupMaps, case insensitive.
func TestNewDynGroupMapsCaseInsensitive(t *testing.T) {
	want := NewDynGroupMaps([]string{
		"dyn-{org}-{team}",
		"other:{org}/{team}",
	})
	got := NewDynGroupMaps([]string{
		"dyn-{org}-{team}",
		"DYN-{ORG}-{TEAM}",
		"DyN-{OrG}-{TeAm}",
		"dYn-{oRg}-{tEaM}",
		"other:{org}/{team}",
		"OTHER:{ORG}/{TEAM}",
		"OtHeR:{OrG}/{TeAm}",
		"oThEr:{oRg}/{tEaM}",
	})
	assert.Equal(t, want, got)
}

// TestNewDynGroupMapsInvalidPlaceholders tests NewDynGroupMaps, invalid placeholders.
func TestNewDynGroupMapsInvalidPlaceholders(t *testing.T) {
	got := NewDynGroupMaps([]string{
		"dyn-{org}",
		"dyn-{team}",
		"dyn-{org}-{org}-{team}",
		"dyn-{org}-{team}-{team}",
		"dyn-{org}-{team}-{org}",
		"dyn-{team}-{org}-{team}",
	})
	assert.True(t, got.Empty())
}

// TestGetDynGroupMaps tests GetDynGroupMaps.
func TestGetDynGroupMaps(t *testing.T) {
	defer test.MockProtect(&sourceDynGroupMaps.d)()

	// same source
	want := GetDynGroupMaps(0, []string{
		"dyn-{org}-{team}",
		"other:{org}/{team}",
	})
	got := GetDynGroupMaps(0, []string{
		"dyn-{org}-{team}",
		"other:{org}/{team}",
	})
	assert.Same(t, want, got)

	// different sources
	got = GetDynGroupMaps(1, []string{
		"dyn-{org}-{team}",
		"other:{org}/{team}",
	})
	assert.NotSame(t, want, got)
}

// TestRemoveDynGroupMaps tests RemoveDynGroupMaps.
func TestRemoveDynGroupMaps(t *testing.T) {
	defer test.MockProtect(&sourceDynGroupMaps.d)()

	// empty
	assert.Nil(t, sourceDynGroupMaps.d[0])
	RemoveDynGroupMaps(0)
	assert.Nil(t, sourceDynGroupMaps.d[0])

	// with entry
	GetDynGroupMaps(0, []string{
		"dyn-{org}-{team}",
		"other:{org}/{team}",
	})
	assert.NotNil(t, sourceDynGroupMaps.d[0])
	RemoveDynGroupMaps(0)
	assert.Nil(t, sourceDynGroupMaps.d[0])
}

// TestResolveMappedMemberships tests resolveMappedMemberships.
func TestResolveMappedMemberships(t *testing.T) {
	type test struct {
		name        string
		srcGroups   container.Set[string]
		mappings    map[string]map[string][]string
		dynMappings *DynGroupMaps
		dynRemoval  bool
		wantAdd     map[string][]string
		wantRemove  map[string][]string
	}

	// get from test db:
	// test user with id 2 with memberships:
	// - "org3":  {"owners", "team1", "teamcreaterepo"},
	// - "org17": {"test_team"},
	require.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	ctx := db.DefaultContext
	for _, test := range []test{
		// static, no match
		{
			name:      "static, no match",
			srcGroups: container.SetOf("does-not-matter"),
			mappings: map[string]map[string][]string{
				"test-static": {"static-org": {"static-team"}},
			},
			dynMappings: nil,
			dynRemoval:  false,
			wantAdd:     map[string][]string{},
			wantRemove: map[string][]string{
				"static-org": {"static-team"},
			},
		},
		// static, match
		{
			name:      "static, match",
			srcGroups: container.SetOf("test-static"),
			mappings: map[string]map[string][]string{
				"test-static": {"static-org": {"static-team"}},
			},
			dynMappings: nil,
			dynRemoval:  false,
			wantAdd: map[string][]string{
				"static-org": {"static-team"},
			},
			wantRemove: map[string][]string{},
		},
		// static, multiple matches
		{
			name: "static, multiple matches",
			srcGroups: container.SetOf(
				"test-static",
				"static2",
				"other3",
			),
			mappings: map[string]map[string][]string{
				"test-static": {"static1-org": {"static1-team"}},
				"static2":     {"static2-org": {"static2-team"}},
				"other3":      {"static3-org": {"static3-team"}},
			},
			dynMappings: nil,
			dynRemoval:  false,
			wantAdd: map[string][]string{
				"static1-org": {"static1-team"},
				"static2-org": {"static2-team"},
				"static3-org": {"static3-team"},
			},
			wantRemove: map[string][]string{},
		},
		// static, some matches
		{
			name: "static, some matches",
			srcGroups: container.SetOf(
				"does-not-matter",
				"test-static",
				"other3",
				"does-not-exists",
			),
			mappings: map[string]map[string][]string{
				"test-static": {"static1-org": {"static1-team"}},
				"static2":     {"static2-org": {"static2-team"}},
				"other3":      {"static3-org": {"static3-team"}},
			},
			dynMappings: nil,
			dynRemoval:  false,
			wantAdd: map[string][]string{
				"static1-org": {"static1-team"},
				"static3-org": {"static3-team"},
			},
			wantRemove: map[string][]string{
				"static2-org": {"static2-team"},
			},
		},
		// dynamic, no match
		{
			name:        "dynamic, no match",
			srcGroups:   container.SetOf("test-notmatching"),
			mappings:    map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{"dyn-{org}-{team}"}),
			dynRemoval:  false,
			wantAdd:     map[string][]string{},
			wantRemove:  map[string][]string{},
		},
		// dynamic, match
		{
			name:        "dynamic, match",
			srcGroups:   container.SetOf("dyn-dynorg-dynteam"),
			mappings:    map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{"dyn-{org}-{team}"}),
			dynRemoval:  false,
			wantAdd: map[string][]string{
				"dynorg": {"dynteam"},
			},
			wantRemove: map[string][]string{},
		},
		// dynamic, multiple matches
		{
			name: "dynamic, multiple matches",
			srcGroups: container.SetOf(
				"dyn-dynorg1-dynteam1",
				"dyn-dynorg2-dynteam2",
				"other:dynorg3/dynteam3",
				"other:dynorg4/dynteam4",
			),
			mappings: map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"other:{org}/{team}",
			}),
			dynRemoval: false,
			wantAdd: map[string][]string{
				"dynorg1": {"dynteam1"},
				"dynorg2": {"dynteam2"},
				"dynorg3": {"dynteam3"},
				"dynorg4": {"dynteam4"},
			},
			wantRemove: map[string][]string{},
		},
		// dynamic, case insensitive matches
		{
			name: "dynamic, case insensitive matches",
			srcGroups: container.SetOf(
				"dyn-dynorg1-dynteam1",
				"DYN-DYNORG1-DYNTEAM1",
				"DyN-DyNoRg1-DyNtEaM1",
				"dYn-dYnOrG1-dYnTeAm1",
				"other:dynorg2/dynteam2",
				"OTHER:DYNORG2/DYNTEAM2",
				"OtHeR:DyNoRg2/DyNtEaM2",
				"oThEr:dYnOrG2/dYnTeAm2",
			),
			mappings: map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"OTHER:{ORG}/{TEAM}",
			}),
			dynRemoval: false,
			wantAdd: map[string][]string{
				"dynorg1": {"dynteam1"},
				"dynorg2": {"dynteam2"},
			},
			wantRemove: map[string][]string{},
		},
		// dynamic, other char matches
		{
			name: "dynamic, other chars matches",
			srcGroups: container.SetOf(
				"dyn-dyn_org1-dyn_team1",
				"dyn-dyn.org1-dyn.team1",
				"dyn-dyn!org1-dyn!team1", // invalid char
				"other:dyn_org2/dyn_team2",
				"other:dyn-org2/dyn-team2",
				"other:dyn.org2/dyn.team2",
				"other:dyn!org2/dyn!team2", // invalid char
			),
			mappings: map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"OTHER:{ORG}/{TEAM}",
			}),
			dynRemoval: false,
			wantAdd: map[string][]string{
				"dyn_org1": {"dyn_team1"},
				"dyn.org1": {"dyn.team1"},
				"dyn_org2": {"dyn_team2"},
				"dyn-org2": {"dyn-team2"},
				"dyn.org2": {"dyn.team2"},
			},
			wantRemove: map[string][]string{},
		},
		// dynamic, regex char matches
		{
			name: "dynamic, regex chars matches",
			srcGroups: container.SetOf(
				"\\.+*?()|[]{}^$not_matching",
				"\\.+*?()|[]{}^$invalid*org-invalid*team",
				"\\.+*?()|[]{}^$dyn_org1-dyn_team1",
				"\\\\n-dyn_org2-dyn_team2",
			),
			mappings: map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{
				"\\.+*?()|[]{}^${org}-{team}",
				"\\\\n-{org}-{team}",
			}),
			dynRemoval: false,
			wantAdd: map[string][]string{
				"dyn_org1": {"dyn_team1"},
				"dyn_org2": {"dyn_team2"},
			},
			wantRemove: map[string][]string{},
		},

		// dynamic, some matches
		{
			name: "dynamic, some matches",
			srcGroups: container.SetOf(
				"test-notmatching",
				"dyn-dynorg1-dynteam1",
				"dyn-dynorg2-dynteam2",
				"does-not-matter",
			),
			mappings: map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"other:{org}/{team}",
			}),
			dynRemoval: false,
			wantAdd: map[string][]string{
				"dynorg1": {"dynteam1"},
				"dynorg2": {"dynteam2"},
			},
			wantRemove: map[string][]string{},
		},
		// mixed, no match
		{
			name:      "mixed, no match",
			srcGroups: container.SetOf("does-not-matter"),
			mappings: map[string]map[string][]string{
				"test-static": {"static-org": {"static-team"}},
			},
			dynMappings: NewDynGroupMaps([]string{"dyn-{org}-{team}"}),
			dynRemoval:  false,
			wantAdd:     map[string][]string{},
			wantRemove: map[string][]string{
				"static-org": {"static-team"},
			},
		},
		// mixed, some matches
		{
			name: "mixed, some matches",
			srcGroups: container.SetOf(
				"does-not-matter",
				"test-static",
				"dyn-dynorg1-dynteam1",
				"other3",
				"does-not-exists",
				"dyn-dynorg2-dynteam2",
			),
			mappings: map[string]map[string][]string{
				"test-static": {"static1-org": {"static1-team"}},
				"static2":     {"static2-org": {"static2-team"}},
				"other3":      {"static3-org": {"static3-team"}},
			},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"other:{org}/{team}",
			}),
			dynRemoval: false,
			wantAdd: map[string][]string{
				"dynorg1":     {"dynteam1"},
				"dynorg2":     {"dynteam2"},
				"static1-org": {"static1-team"},
				"static3-org": {"static3-team"},
			},
			wantRemove: map[string][]string{
				"static2-org": {"static2-team"},
			},
		},
		// dynamic, some matches, dynamic remove
		{
			name: "dynamic, some matches, dynamic remove",
			srcGroups: container.SetOf(
				"test-notmatching",
				"dyn-dynorg1-dynteam1",
				"dyn-dynorg2-dynteam2",
				"does-not-matter",
			),
			mappings: map[string]map[string][]string{},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"other:{org}/{team}",
			}),
			dynRemoval: true,
			wantAdd: map[string][]string{
				"dynorg1": {"dynteam1"},
				"dynorg2": {"dynteam2"},
			},
			wantRemove: map[string][]string{
				"org17": {"test_team"},
				"org3":  {"team1", "teamcreaterepo"}, // owners cannot be removed because user is last user
			},
		},
		// mixed, some matches, dynamic remove
		{
			name: "mixed, some matches, dynamic remove",
			srcGroups: container.SetOf(
				"does-not-matter",
				"test-static",
				"dyn-dynorg1-dynteam1",
				"other3",
				"does-not-exists",
				"dyn-dynorg2-dynteam2",
			),
			mappings: map[string]map[string][]string{
				"test-static": {"static1-org": {"static1-team"}},
				"static2":     {"static2-org": {"static2-team"}},
				"other3":      {"static3-org": {"static3-team"}},
			},
			dynMappings: NewDynGroupMaps([]string{
				"dyn-{org}-{team}",
				"other:{org}/{team}",
			}),
			dynRemoval: true,
			wantAdd: map[string][]string{
				"dynorg1":     {"dynteam1"},
				"dynorg2":     {"dynteam2"},
				"static1-org": {"static1-team"},
				"static3-org": {"static3-team"},
			},
			wantRemove: map[string][]string{
				// "static2-org": {"static2-team"} only if user added to it previously
				"org17": {"test_team"},
				"org3":  {"team1", "teamcreaterepo"}, // owners cannot be removed because user is last user
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotAdd, gotRemove := resolveMappedMemberships(ctx, user,
				test.srcGroups, test.mappings,
				test.dynMappings, test.dynRemoval)

			assert.Equal(t, test.wantAdd, gotAdd)
			assert.Equal(t, test.wantRemove, gotRemove)
		})
	}
}
