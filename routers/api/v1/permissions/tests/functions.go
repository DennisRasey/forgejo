// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

// See README.md for a documentation of the test logic

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/test"
	"forgejo.org/modules/web/routing"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
	apiv1_permissions_testhelpers "forgejo.org/routers/api/v1/permissions/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createFixture(t *testing.T, signatures [][]any, permissions *apiv1_permissions.Permissions, data *testData) {
	for _, signature := range signatures[:len(signatures)-1] {
		signatureString := apiv1_permissions_testhelpers.SignatureToString(signature)
		functionTest, has := signatureStringToFunctionTest[signatureString]
		require.True(t, has)
		if functionTest.fulfillNeeds != nil {
			functionTest.fulfillNeeds(t, data)
		}
	}
	for _, signature := range signatures {
		signatureString := apiv1_permissions_testhelpers.SignatureToString(signature)
		functionTest, has := signatureStringToFunctionTest[signatureString]
		require.True(t, has)
		if functionTest.interpret != nil {
			functionTest.interpret(t, permissions, data)
		}
	}
}

func getFunctionTest(t *testing.T, signatures [][]any) functionTest {
	lastSignature := signatures[len(signatures)-1]
	lastSignatureString := apiv1_permissions_testhelpers.SignatureToString(lastSignature)
	test, has := signatureStringToFunctionTest[lastSignatureString]
	require.True(t, has, lastSignatureString)
	return test
}

func getFixtures(t *testing.T, signatures [][]any) []*testCase {
	return getFunctionTest(t, signatures).testCases
}

func protectVariables(t *testing.T, signatures [][]any) {
	for _, signature := range signatures {
		signatureString := apiv1_permissions_testhelpers.SignatureToString(signature)
		functionTest, has := signatureStringToFunctionTest[signatureString]
		require.True(t, has)
		for _, b := range functionTest.protectSettingsBool {
			t.Cleanup(test.MockProtect(b))
		}
	}
}

func testSequence(t *testing.T, signatures [][]any, onlyForSuccess bool) int {
	t.Helper()
	signaturesString := apiv1_permissions_testhelpers.SignaturesToString(signatures)
	var fixtures []*testCase
	if onlyForSuccess {
		for _, fixture := range getFixtures(t, signatures) {
			if fixture.error == "" {
				fixtures = []*testCase{fixture}
				break
			}
		}
		require.NotEmpty(t, fixtures, "%s must have at least one fixture with no error", signaturesString)
	} else {
		fixtures = getFixtures(t, signatures)
	}
	for i, fixture := range fixtures {
		runName := signaturesString
		if len(fixtures) > 1 {
			runName = fmt.Sprintf("%s fixture %d", signaturesString, i)
		}
		t.Run(runName, func(t *testing.T) {
			protectVariables(t, signatures)

			_ = unittest.LoadFixtures() // reset the database to clear any side effect of running the test
			permissions := &apiv1_permissions.Permissions{}
			permissions.SetContext(t.Context())
			t.Logf("creating fixture data from %s", fixture.data)
			modifiedFixture := fixture.Clone()
			createFixture(t, signatures, permissions, modifiedFixture.data)
			t.Logf("created fixture data %s", modifiedFixture.data)

			var previousPerms []string
			showPermissionsDiff := func() {
				newPerms := permissions.Strings()
				if previousPerms == nil {
					for _, s := range newPerms {
						t.Logf("\t%s", s)
					}
				} else {
					// easier to compute the additions first (since we can destroy previousPerms)
					// nicer to show the additions after the deletion
					var additions []string
					for _, s := range newPerms {
						if i := slices.Index(previousPerms, s); i >= 0 {
							if i == 0 {
								// most frequent case
								previousPerms = previousPerms[1:]
								continue
							}
							last := len(previousPerms) - 1
							if i != last {
								previousPerms[i] = previousPerms[last]
							}
							previousPerms = previousPerms[:last]
							continue
						}
						additions = append(additions, s)
					}
					for _, s := range previousPerms {
						t.Logf("\t- %s", s)
					}
					for _, s := range additions {
						t.Logf("\t+ %s", s)
					}
				}
				previousPerms = newPerms
			}
			showPermissionsDiff()
			var permissionsContext apiv1_permissions.Context = permissions
			for i, signature := range signatures {
				signatureString := apiv1_permissions_testhelpers.SignatureToString(signature)
				functionTest, has := signatureStringToFunctionTest[signatureString]
				require.True(t, has)

				args := signature[1:]
				if len(args) != functionTest.staticArgs {
					t.Fatalf("%s expects %d static arguments, got %d: %#v", routing.GetFuncShortName(signature[0]), functionTest.staticArgs, len(args), args)
				}
				functionTest.call(t, permissionsContext, modifiedFixture.data, args)
				showPermissionsDiff()
				if i == len(signatures)-1 {
					fixture.used = true
					if fixture.error != "" {
						assert.NotZero(t, permissions.GetStatus())
						assert.Contains(t, permissions.GetMessage(), fixture.error)
					} else {
						assert.Zero(t, permissions.GetStatus(), permissions.GetMessage())
					}
				} else {
					assert.Zero(t, permissions.GetStatus(), permissions.GetMessage())
				}
			}
		})
	}
	return len(fixtures)
}

func getPermissionSequenceForFunction(t *testing.T, sequence [][]any) [][]any {
	sequenceString := apiv1_permissions_testhelpers.SignaturesToString(sequence)
	sequenceFilter := getFunctionTest(t, sequence).sequenceFilter
	if sequenceFilter == nil {
		return sequence
	}
	var filteredSequence [][]any
	if len(sequenceFilter) > len(sequence) {
		panic(fmt.Errorf("%s is longer than %v", sequenceString, sequenceFilter))
	}
	for _, signature := range sequence {
		if len(sequenceFilter) == 0 {
			break
		}
		signatureString := apiv1_permissions_testhelpers.SignatureToString(signature)
		if signatureString == sequenceFilter[0] || strings.HasPrefix(signatureString, sequenceFilter[0]+" ") {
			filteredSequence = append(filteredSequence, signature)
			sequenceFilter = sequenceFilter[1:]
		}
	}
	if len(sequenceFilter) > 0 {
		panic(fmt.Errorf("%s filtered by %v does not consume all filters and %v is left", sequenceString, getFunctionTest(t, sequence).sequenceFilter, sequenceFilter))
	}
	if len(filteredSequence) == 0 {
		panic(fmt.Errorf("%s filtered by %v is an empty sequence", sequenceString, getFunctionTest(t, sequence).sequenceFilter))
	}
	getPrefix := func(signature []any) string {
		signatureString := apiv1_permissions_testhelpers.SignatureToString(signature)
		prefix, _, _ := strings.Cut(signatureString, " ")
		return prefix
	}
	lastFilteredFunctionString := getPrefix(filteredSequence[len(filteredSequence)-1])
	lastFunctionString := getPrefix(sequence[len(sequence)-1])
	if lastFilteredFunctionString != lastFunctionString {
		panic(fmt.Errorf("%s filtered by %v ends with the function %s instead of %s", sequenceString, getFunctionTest(t, sequence).sequenceFilter, lastFilteredFunctionString, lastFunctionString))
	}
	return filteredSequence
}

func getPermissionSequencesForFunctions(t *testing.T) [][][]any {
	var sequences [][][]any
	for _, sequence := range apiv1_permissions_testhelpers.GetShortestPermissionSequenceForEachSignature() {
		sequences = append(sequences, getPermissionSequenceForFunction(t, sequence))
	}
	return sequences
}

func APIv1Permissions(t *testing.T) {
	buildSignatureStringToFunctionTest(t)

	runs := 0
	t.Logf("running all fixtures for each permission function")
	for _, sequence := range getPermissionSequencesForFunctions(t) {
		runs += testSequence(t, sequence, false)
	}
	t.Logf("verify all unique permission sequences can run successfully")
	uniqueSequences := apiv1_permissions_testhelpers.GetUniquePermissionsSequences()
	for _, sequence := range uniqueSequences {
		runs += testSequence(t, sequence, true)
	}

	hasRunArg := func() bool {
		for _, arg := range os.Args {
			if arg != "-test.run=Test" && strings.HasPrefix(arg, "-test.run") {
				return true
			}
			if arg != "-run=Test" && strings.HasPrefix(arg, "-run") {
				return true
			}
		}
		return false
	}

	// this sanity check will fail if only a selection of tests is run
	// with -run=TestMyTest
	if !hasRunArg() {
		unusedFixtures := false
		for signatureString, test := range signatureStringToFunctionTest {
			for _, fixture := range test.testCases {
				if !fixture.used {
					t.Logf("%s fixture %v not used", signatureString, fixture.data)
					unusedFixtures = true
				}
			}
		}
		assert.False(t, unusedFixtures)
	}
	t.Logf("%d unique sequence of permission functions", len(uniqueSequences))
	t.Logf("%d permissions functions", len(signatureStringToFunctionTest))
	t.Logf("used a total of %d fixtures", runs)
}
