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

	unit_model "forgejo.org/models/unit"
	"forgejo.org/modules/web/routing"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
	apiv1_permissions_testhelpers "forgejo.org/routers/api/v1/permissions/testhelpers"
)

func pointerToCopyOrNil[T any](s *T) *T {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

func getReferenceOrZero[T any](s *T) T {
	var zero T
	if s == nil {
		return zero
	}
	return *s
}

type doer struct {
	name *string

	admin                 *bool
	canCreateOrganization *bool
	authentication        *string
	scope                 *string

	actions                  *bool
	actionsRepoID            *int64
	actionsIsForkPullRequest *bool
}

func (o doer) Clone() doer {
	return doer{
		name: pointerToCopyOrNil(o.name),

		admin:                 pointerToCopyOrNil(o.admin),
		canCreateOrganization: pointerToCopyOrNil(o.canCreateOrganization),
		authentication:        pointerToCopyOrNil(o.authentication),
		scope:                 pointerToCopyOrNil(o.scope),

		actions:                  pointerToCopyOrNil(o.actions),
		actionsRepoID:            pointerToCopyOrNil(o.actionsRepoID),
		actionsIsForkPullRequest: pointerToCopyOrNil(o.actionsIsForkPullRequest),
	}
}

func (o doer) String() string {
	var str []string
	if o.name != nil {
		str = append(str, fmt.Sprintf("doer:%v", *o.name))
	}
	if o.admin != nil {
		str = append(str, fmt.Sprintf("doer.admin:%v", *o.admin))
	}
	if o.canCreateOrganization != nil {
		str = append(str, fmt.Sprintf("doer.canCreateOrganization:%v", *o.canCreateOrganization))
	}
	if o.authentication != nil {
		str = append(str, "doer.authentication:"+*o.authentication)
	}
	if o.scope != nil {
		str = append(str, "doer.scope:"+*o.scope)
	}
	if o.actions != nil {
		str = append(str, fmt.Sprintf("actions:%v", *o.actions))
	}
	if o.actionsRepoID != nil {
		str = append(str, fmt.Sprintf("actions.RepoID:%v", *o.actionsRepoID))
	}
	if o.actionsIsForkPullRequest != nil {
		str = append(str, fmt.Sprintf("actions.IsForkPullRequest:%v", *o.actionsIsForkPullRequest))
	}
	return strings.Join(str, " ")
}

type repository struct {
	name          *string
	private       *bool
	init          *bool
	archived      *bool
	disabledUnits *[]unit_model.Type
}

func (o repository) Clone() repository {
	var disabledUnits *[]unit_model.Type
	if o.disabledUnits != nil {
		clone := slices.Clone(*o.disabledUnits)
		disabledUnits = &clone
	}
	return repository{
		name:          pointerToCopyOrNil(o.name),
		private:       pointerToCopyOrNil(o.private),
		init:          pointerToCopyOrNil(o.init),
		archived:      pointerToCopyOrNil(o.archived),
		disabledUnits: disabledUnits,
	}
}

func (o repository) String() string {
	var str []string
	if o.name != nil {
		str = append(str, fmt.Sprintf("repository:%v", *o.name))
	}
	if o.private != nil {
		str = append(str, fmt.Sprintf("repository.private:%v", *o.private))
	}
	if o.init != nil {
		str = append(str, fmt.Sprintf("repository.init:%v", *o.init))
	}
	if o.archived != nil {
		str = append(str, fmt.Sprintf("repository.archived:%v", *o.archived))
	}
	if o.disabledUnits != nil {
		str = append(str, fmt.Sprintf("repository.disabledUnits:%v", *o.disabledUnits))
	}
	return strings.Join(str, " ")
}

type sharedData struct {
	doer       doer
	repository repository
	anonymous  *bool
	tokenLevel *string
}

func (o sharedData) Clone() sharedData {
	return sharedData{
		doer:       o.doer.Clone(),
		repository: o.repository.Clone(),
		anonymous:  pointerToCopyOrNil(o.anonymous),
		tokenLevel: pointerToCopyOrNil(o.tokenLevel),
	}
}

func (o sharedData) String() string {
	var str []string
	if !o.Anonymous() {
		str = append(str, fmt.Sprintf("%s", o.doer))
	} else {
		str = append(str, "anonymous")
	}
	if o.RepositoryName() != "" {
		str = append(str, fmt.Sprintf("%s", o.repository))
	}
	if o.TokenLevel() != "" {
		str = append(str, fmt.Sprintf("token.level:%s", *o.tokenLevel))
	}
	return strings.Join(str, " ")
}

func newSharedData() *sharedData {
	return &sharedData{}
}

func (o sharedData) DoerName() string {
	return getReferenceOrZero(o.doer.name)
}

func (o sharedData) HasDoerName() bool {
	return o.doer.name != nil
}

func (o *sharedData) SetDoerNameDefault(name string) *sharedData {
	if !o.HasDoerName() {
		o.SetDoerName(name)
	}
	return o
}

func (o *sharedData) SetDoerName(name string) *sharedData {
	o.doer.name = &name
	return o
}

func (o *sharedData) SetDoer() *sharedData {
	return o.SetDoerName(randomName())
}

func (o *sharedData) SetDoerDefault() *sharedData {
	return o.SetDoerNameDefault(randomName())
}

func (o sharedData) DoerAdmin() bool {
	return getReferenceOrZero(o.doer.admin)
}

func (o sharedData) HasDoerAdmin() bool {
	return o.doer.admin != nil
}

func (o *sharedData) SetDoerAdminDefault(admin bool) *sharedData {
	if !o.HasDoerAdmin() {
		o.SetDoerAdmin(admin)
	}
	return o
}

func (o *sharedData) SetDoerAdmin(admin bool) *sharedData {
	o.doer.admin = &admin
	return o
}

func (o sharedData) DoerCanCreateOrganization() bool {
	return getReferenceOrZero(o.doer.canCreateOrganization)
}

func (o sharedData) HasDoerCanCreateOrganization() bool {
	return o.doer.canCreateOrganization != nil
}

func (o *sharedData) SetDoerCanCreateOrganizationDefault(canCreateOrganization bool) *sharedData {
	if !o.HasDoerCanCreateOrganization() {
		o.SetDoerCanCreateOrganization(canCreateOrganization)
	}
	return o
}

func (o *sharedData) SetDoerCanCreateOrganization(canCreateOrganization bool) *sharedData {
	o.doer.canCreateOrganization = &canCreateOrganization
	return o
}

func (o sharedData) DoerAuthentication() string {
	return getReferenceOrZero(o.doer.authentication)
}

func (o sharedData) HasDoerAuthentication() bool {
	return o.doer.authentication != nil
}

func (o *sharedData) SetDoerAuthenticationDefault(authentication string) *sharedData {
	if !o.HasDoerAuthentication() {
		o.SetDoerAuthentication(authentication)
	}
	return o
}

func (o *sharedData) SetDoerAuthentication(authentication string) *sharedData {
	o.doer.authentication = &authentication
	return o
}

func (o sharedData) DoerScope() string {
	return getReferenceOrZero(o.doer.scope)
}

func (o sharedData) HasDoerScope() bool {
	return o.doer.scope != nil
}

func (o *sharedData) SetDoerScopeDefault(scope string) *sharedData {
	if !o.HasDoerScope() {
		o.SetDoerScope(scope)
	}
	return o
}

func (o *sharedData) SetDoerScope(scope string) *sharedData {
	o.doer.scope = &scope
	return o
}

func (o sharedData) DoerActions() bool {
	return getReferenceOrZero(o.doer.actions)
}

func (o sharedData) HasDoerActions() bool {
	return o.doer.actions != nil
}

func (o *sharedData) SetDoerActionsDefault(actions bool) *sharedData {
	if !o.HasDoerActions() {
		o.SetDoerActions(actions)
	}
	return o
}

func (o *sharedData) SetDoerActions(actions bool) *sharedData {
	o.doer.actions = &actions
	return o
}

func (o sharedData) DoerActionsRepoID() int64 {
	return getReferenceOrZero(o.doer.actionsRepoID)
}

func (o sharedData) HasDoerActionsRepoID() bool {
	return o.doer.actionsRepoID != nil
}

func (o *sharedData) SetDoerActionsRepoIDDefault(actionsRepoID int64) *sharedData {
	if !o.HasDoerActionsRepoID() {
		o.SetDoerActionsRepoID(actionsRepoID)
	}
	return o
}

func (o *sharedData) SetDoerActionsRepoID(actionsRepoID int64) *sharedData {
	o.doer.actionsRepoID = &actionsRepoID
	return o
}

func (o sharedData) DoerActionsIsForkPullRequest() bool {
	return getReferenceOrZero(o.doer.actionsIsForkPullRequest)
}

func (o sharedData) HasDoerActionsIsForkPullRequest() bool {
	return o.doer.actionsIsForkPullRequest != nil
}

func (o *sharedData) SetDoerActionsIsForkPullRequestDefault(actionsIsForkPullRequest bool) *sharedData {
	if !o.HasDoerActionsIsForkPullRequest() {
		o.SetDoerActionsIsForkPullRequest(actionsIsForkPullRequest)
	}
	return o
}

func (o *sharedData) SetDoerActionsIsForkPullRequest(actionsIsForkPullRequest bool) *sharedData {
	o.doer.actionsIsForkPullRequest = &actionsIsForkPullRequest
	return o
}

func (o sharedData) RepositoryName() string {
	return getReferenceOrZero(o.repository.name)
}

func (o sharedData) HasRepositoryName() bool {
	return o.repository.name != nil
}

func (o *sharedData) SetRepositoryNameDefault(name string) *sharedData {
	if !o.HasRepositoryName() {
		o.SetRepositoryName(name)
	}
	return o
}

func (o *sharedData) SetRepositoryName(name string) *sharedData {
	o.repository.name = &name
	return o
}

func (o *sharedData) SetRepositoryDefault() *sharedData {
	return o.SetRepositoryNameDefault(fmt.Sprintf("%s/%s", randomName(), randomName()))
}

func (o *sharedData) SetRepository() *sharedData {
	return o.SetRepositoryName(fmt.Sprintf("%s/%s", randomName(), randomName()))
}

func (o sharedData) RepositoryPrivate() bool {
	return getReferenceOrZero(o.repository.private)
}

func (o sharedData) HasRepositoryPrivate() bool {
	return o.repository.private != nil
}

func (o *sharedData) SetRepositoryPrivateDefault(private bool) *sharedData {
	if !o.HasRepositoryPrivate() {
		o.SetRepositoryPrivate(private)
	}
	return o
}

func (o *sharedData) SetRepositoryPrivate(private bool) *sharedData {
	o.repository.private = &private
	return o
}

func (o sharedData) RepositoryInit() bool {
	return getReferenceOrZero(o.repository.init)
}

func (o sharedData) HasRepositoryInit() bool {
	return o.repository.init != nil
}

func (o *sharedData) SetRepositoryInitDefault(init bool) *sharedData {
	if !o.HasRepositoryInit() {
		o.SetRepositoryInit(init)
	}
	return o
}

func (o *sharedData) SetRepositoryInit(init bool) *sharedData {
	o.repository.init = &init
	return o
}

func (o sharedData) RepositoryArchived() bool {
	return getReferenceOrZero(o.repository.archived)
}

func (o sharedData) HasRepositoryArchived() bool {
	return o.repository.archived != nil
}

func (o *sharedData) SetRepositoryArchivedDefault(archived bool) *sharedData {
	if !o.HasRepositoryArchived() {
		o.SetRepositoryArchived(archived)
	}
	return o
}

func (o *sharedData) SetRepositoryArchived(archived bool) *sharedData {
	o.repository.archived = &archived
	return o
}

func (o sharedData) RepositoryDisabledUnits() []unit_model.Type {
	return getReferenceOrZero(o.repository.disabledUnits)
}

func (o sharedData) HasRepositoryDisabledUnits() bool {
	return o.repository.disabledUnits != nil
}

func (o *sharedData) SetRepositoryDisabledUnitsDefault(disabledUnits []unit_model.Type) *sharedData {
	if !o.HasRepositoryDisabledUnits() {
		o.SetRepositoryDisabledUnits(disabledUnits)
	}
	return o
}

func (o *sharedData) SetRepositoryDisabledUnits(disabledUnits []unit_model.Type) *sharedData {
	o.repository.disabledUnits = &disabledUnits
	return o
}

func (o sharedData) Anonymous() bool {
	return getReferenceOrZero(o.anonymous)
}

func (o sharedData) HasAnonymous() bool {
	return o.anonymous != nil
}

func (o *sharedData) SetAnonymousDefault(anonymous bool) *sharedData {
	if !o.HasAnonymous() {
		o.SetAnonymous(anonymous)
	}
	return o
}

func (o *sharedData) SetAnonymous(anonymous bool) *sharedData {
	o.anonymous = &anonymous
	return o
}

func (o sharedData) TokenLevel() string {
	return getReferenceOrZero(o.tokenLevel)
}

func (o sharedData) HasTokenLevel() bool {
	return o.tokenLevel != nil
}

func (o *sharedData) SetTokenLevelDefault(tokenLevel string) *sharedData {
	if !o.HasTokenLevel() {
		o.SetTokenLevel(tokenLevel)
	}
	return o
}

func (o *sharedData) SetTokenLevel(tokenLevel string) *sharedData {
	o.tokenLevel = &tokenLevel
	return o
}

type testData struct {
	own    map[string]string
	shared *sharedData
}

func (o *testData) Set(key, value string) {
	o.own[key] = value
}

func (o *testData) SetDefault(key, value string) {
	if !o.Has(key) {
		o.Set(key, value)
	}
}

func (o *testData) Get(key string) string {
	return o.own[key]
}

func (o *testData) Has(key string) bool {
	_, has := o.own[key]
	return has
}

func (o *testData) String() string {
	var s []string
	s = append(s, o.shared.String())
	for k, e := range o.own {
		s = append(s, fmt.Sprintf("%s:%s", k, e))
	}
	slices.Sort(s)
	return strings.Join(s, ",")
}

func newTestData(own map[string]string, shared *sharedData) *testData {
	testData := &testData{
		own:    make(map[string]string, 10),
		shared: shared,
	}
	for key, value := range own {
		testData.Set(key, value)
	}
	return testData
}

func (o *testData) Clone() *testData {
	sharedClone := o.shared.Clone()
	return &testData{
		own:    maps.Clone(o.own),
		shared: &sharedClone,
	}
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
