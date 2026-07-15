Tests for `routers/api/v1/permissions`

Each permission function implemented in the `routers/api/v1/permissions` has a matching test in this package. For instance:

- the `ReqGitHook` function in `routers/api/v1/permissions/req_git_hook.go`
- is tested in `routers/api/v1/permissions/tests/req_git_hook_test.go`

To keep the tests maintainable despite the large number of fixtures and permission sequences, the tests for a function are described in a structure instead of being implemented by a `Test...` function.

```go
type functionTest struct {
	testCases      []*testCase
	fulfillNeeds   func(t *testing.T, data *testData)
	interpret      func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData)

	protect        func() func()
	call           func(t *testing.T, ctx apiv1_permissions.Context, permissions *apiv1_permissions.Permissions, data *testData, signature []any)
	sequenceFilter []string
}
```

- The test registers the struct and it will be used when the `TestAPIv1Permissions` test runs
- The `testCases` are described using a `map[string]string` using conventions that are to be interpreted by the fixture helpers. For instance `"doer": "regularuser"` will be interpreted by the `fixtureSetDoer` helper and create the user `regularuser`.
- The `fulfillNeeds` function will be called for each function that comes earlier in the sequence, to ensure it gets sensible defaults allowing it to run successfully. For instance if the sequence is `APIAuthorization,TokenRequiresScopes,ReqOrgOwnership`, the `fulfillNeeds` function of `TokenRequiresScopes` is expected to set a sensible default for the scope, such as `"scope": "read:repository"`
- After `fulfillNeeds` is called and the `testData` is assumed to have all the necessary defaults, it will be acted upon by each `interpret` function in the sequence. For instance, `APIAuthorization` will interpret the `"doer": "regularuser"` data by calling `fixtureSetDoer` to ensure it is created.
- If global variables need protection (for instance when changing a setting), they are to be protected by the `protect` function
- Once the `testData` has been interpreted, each permission function in the sequence is called in order. They are all expected to complete successfully. Except for the last one, which is the function under test, that may error out if the `testData` is designed for that purpose.
- The `call` function, if it exists, is expected to call the function for which the test is designed. Fos instance the `call` for `ReqValidCommentID` runs `apiv1_permissions.ReqValidCommentID(ctx, comment)`. The function must not have any side effect. Instead it must use what has been created by the fixtures (using the `interpret` function).
- The `sequenceFilter` only keeps some permissions function in the sequence leading to the function under test. For instance when testing `ReqOrgOwnership` the sequence `APIAuthorization,TokenRequiresScopes,ReqOrgOwnership` will be used. In some cases it is useful to simplify the tests in case the shortest sequence leading to a function contains functions that will interfere if a particular `testCase` is set.

## Test data interpretation

The `testData` of the `testCase` is split into data:

- owned by the `testCase` (`Has`, `Get`, `Set`, `SetDefault`) and not
  meant to be used anywhere else in the sequence. They should be used
  when creating a new `testCase` unless there is a good reason not to.
- shared by all the `testCase` in the sequence (`shared`)
  and care must be taken about how such data is handled
  by the `fulfillNeeds` function.

The `shared` data define:

- The `Doer` (which can also be anonymous)
- The `Repository`
- The `TokenLevel`

When running the tests for the sequence A,B,C the data that will be
used by the `interpret` function is built starting from the data of
C. Then the `fulfillNeeds` function of B is called and may modify it
depending on what it needs to succeed. The permission function under
test is C and the permission functions that come before it (B, A) in
the sequence are only expected to successfully pass so the sequence
can reach C without return a premature error.

Data shared between tests must be limited to a minimum because their
interactions are difficult to figure out. For instance
`APIAuthorization` must set a default `shared.DoerName()` and the
`shared.DoerAdmin()` value associated with it needs to be overriden by
the `ReqAdmin` tests function when it is not last in the sequence,
otherwise it will fail.

## Permission function signatures

The signature of every permission function has at least one argument which is a `routers/api/v1/permissions.Context` interface. It may also have additional arguments provided when building the routes. For instance `TokenRequiresScopes` may be given a list of scope categories. Such arguments do not vary depending on the context because they are preset when the route is built. In addition the function may have arguments that are extracted from the environment. For instance `ReqValidCommentID` may be given the content of the `id` field from the body of a JSON payload.

The string representation of the signature is:

- The function name if there are no arguments provided when building the routes. For instance `APIAuthorization`
- The function name followed by a whitespace list of arguments provided when building the routes. For instance `TokenRequiresScopes Repository User`

## Fixtures helpers

All fixtures are dynamically created (they are not using the global fixtures found in `models/fixtures`). The `fixture.go` file contains all the helpers to create those fixtures.

## Debugging

- Running the tests in verbose mode `COVERAGE_TEST_ARGS='-test.v -test.run=TestAPIv1Permissions' make coverage-reset coverage-run-integration-sqlite`
- Browsing the tests such as
```
...
=== RUN   TestAPIv1Permissions/APIAuthorization,TokenRequiresScopes_Admin_fixture_0
    functions.go:93: creating fixture data from doer:doername doer.scope:read:admin token.level:read
    functions.go:96: created fixture data doer:doername doer.scope:read:admin token.level:read
    functions.go:103: 	*auth.AccessToken(ID=5302 Token=ccd381ada3140a9ada3a72901456d7bb3553b3ce)
    functions.go:103: 	doer=*user.User(Name=doername, Private=false, Admin=false)
    functions.go:103: 	isSigned(true)
    functions.go:103: 	*tests.accessTokenAuthenticationResult(*user.User(Name=doername) auth.AccessTokenScope(read:admin) *authz.AllAccessAuthorizationReducer)
    testcase.go:550: calling permissions.APIAuthorization(ctx)
    functions.go:129: 	+ *authz.AllAccessAuthorizationReducer
    token_requires_scopes.go:94: calling TokenRequiresScopes(ctx, [1], 1)
    functions.go:129: 	+ []auth.AccessTokenScopeCategory([1])
...
```
- The name of the test is the sequence of middleware under test (`APIAuthorization`, `TokenRequiresScopes`)
- It is followed by the index of the `testCase` being used for running the test, as found in the test file of the last function in the sequence (`TokenRequiresScopes` in the example)
- The `creating fixture` line shows all the `testData` used in that `testCase`
- The `created fixture` line shows the `testData` added after calling the `fulfillNeeds` function for each function in the sequence
- The indented lines that follow shows the content of the `routers/api/v1/permissions.Permission` object before calling a permission function. To reduce the verbosity modifications are shown in a diff style fashion.
- The `calling` line shows the function and its arguments before it is called
- Running a single test (note the `/` is replaced with a `.` in the test name to comply with the Makefile rule) `make RACE_ENABLED=true GOTESTFLAGS=-v GO_TEST_PACKAGES=forgejo.org/routers/api/v1/permissions/tests/... 'test#TestAPIv1Permissions.APIAuthorization,TokenRequiresScopes_Repository,RepoAccess,CheckTokenPublicOnly,ReqToken,ReqRepoReader_TypeCode,CheckForkDestination'`

## The call function

`func(t *testing.T, ctx apiv1_permissions.Context, data *testData, staticArgs []any)`

It is responsible for:

- Calling `t.Logf` to display the call about to be made
- Calling the function using `ctx` as a first argument

The `permissions` and `data` arguments are provided, as computed by the `interpret` function.

The `args` are the mandatory arguments to the function call.

## Test coverage

### `routers/api/v1/permissions`

- At the root of the source tree
- `COVERAGE_TEST_ARGS='-test.v -test.run=TestAPIv1Permissions' make coverage-reset coverage-run-integration-sqlite coverage-show-percentage | grep v1/permissions | grep -v v1/permissions/permissions.go | grep -v v1/permissions/test | sed -e 's/\t\t*/ /g' -e 's|forgejo.org/routers/||'`
- `uncover coverage/textfmt.out ReqOrgOwnership`

### Forgejo development branch

- Run https://codeberg.org/forgejo/forgejo/actions?workflow=coverage.yml
- Download the `coverage.zip` artifact
- Extract it in `/tmp/coverage/merged`
- At the root of the source tree checked out at the same SHA that coverage used
- Convert with `go tool covdata textfmt -i=/tmp/coverage/merged -o=/tmp/coverage/textfmt.out`
- Show percentages per function `go tool cover -func=/tmp/coverage/textfmt.out`
- Show line covered and missed for a function `uncover /tmp/coverage/textfmt.out repoAssignment`

## References

Design discussion https://codeberg.org/forgejo/design/issues/63
