// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

// See README.md for a documentation of the test logic

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"forgejo.org/modules/web/routing"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
	apiv1_permissions_testhelpers "forgejo.org/routers/api/v1/permissions/testhelpers"

	"github.com/stretchr/testify/require"
)

func dataToString(t *testing.T, testData *testData, key string) string {
	t.Helper()
	require.True(t, testData.Has(key))
	return testData.Get(key)
}

type testData struct {
	entries map[string]string
}

func (o *testData) Set(key, value string) {
	o.entries[key] = value
}

func (o *testData) SetDefault(key, value string) {
	if !o.Has(key) {
		o.Set(key, value)
	}
}

func (o *testData) Get(key string) string {
	return o.entries[key]
}

func (o *testData) Has(key string) bool {
	_, has := o.entries[key]
	return has
}

func (o *testData) String() string {
	var s []string
	for k, e := range o.entries {
		s = append(s, fmt.Sprintf("%s:%s", k, e))
	}
	slices.Sort(s)
	return strings.Join(s, ",")
}

func newTestData(data map[string]string) *testData {
	testData := &testData{
		entries: make(map[string]string, 10),
	}
	for key, value := range data {
		testData.Set(key, value)
	}
	return testData
}

func (o *testData) Clone() *testData {
	return &testData{entries: maps.Clone(o.entries)}
}

type testCase struct {
	data  *testData
	error string

	used bool
}

func (o *testCase) Clone() *testCase {
	f := *o
	f.data = o.data.Clone()
	return &f
}

// See README.md for a documentation of the test logic that uses
// this test description.
type functionTest struct {
	// The testCase will be constructed, when this function is the last
	// one of the chain.  It will go through the fulfillNeeds and
	// interpret of the previous functions in the chain, as well as its
	// own interpret.
	testCases []*testCase

	// List the settings which might be updated while interpreting the testData
	// so that they are restored upon test completion.
	protectSettingsBool []*bool

	// number of static arguments to pass to call's last argument
	staticArgs int
	// call the middleware (set automatically by [registerFunctionTest])
	call func(t *testing.T, ctx apiv1_permissions.Context, data *testData, staticArgs []any)

	sequenceFilter []string
	fulfillNeeds   func(t *testing.T, data *testData)
	interpret      func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData)
}

func buildSignatureStringToFunctionTest(t *testing.T) {
	for signatureString, signature := range apiv1_permissions_testhelpers.GetSignatureStringToSignature() {
		for prefix, builder := range prefixToFunctionTestBuilder {
			if strings.HasPrefix(signatureString, prefix) {
				builder(t, signatureString, signature)
			}
		}
	}
}

func registerFunctionTest(fun func(apiv1_permissions.Context), test functionTest) bool {
	shortName := routing.GetFuncShortName(fun)
	test.call = func(t *testing.T, ctx apiv1_permissions.Context, _ *testData, _ []any) {
		t.Logf("calling %s(ctx)", shortName)
		fun(ctx)
	}
	return registerFunctionTestWithCall(fun, test)
}

func registerFunctionTestWithCall(fun any, test functionTest) bool {
	signatureString := apiv1_permissions_testhelpers.SignatureToString([]any{fun})
	if _, has := signatureStringToFunctionTest[signatureString]; has {
		panic(fmt.Errorf("attempt to register %s twice", signatureString))
	}
	if test.call == nil {
		panic("'call' field is required")
	}
	signatureStringToFunctionTest[signatureString] = test
	return true
}

var signatureStringToFunctionTest = map[string]functionTest{}

type functionTestBuilder func(t *testing.T, signatureString string, signature []any)

func registerFunctionTestBuilder(prefixes []string, builder functionTestBuilder) bool {
	for _, prefix := range prefixes {
		if _, has := prefixToFunctionTestBuilder[prefix]; has {
			panic(fmt.Errorf("attempt to register %s twice", prefix))
		}
		prefixToFunctionTestBuilder[prefix] = builder
	}
	return true
}

var prefixToFunctionTestBuilder = map[string]functionTestBuilder{}
