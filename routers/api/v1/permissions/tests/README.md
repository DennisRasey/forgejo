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

## Permission function signatures

The signature of every permission function has at least one argument which is a `routers/api/v1/permissions.Context` interface. It may also have additional arguments provided when building the routes. For instance `TokenRequiresScopes` may be given a list of scope categories. Such arguments do not vary depending on the context because they are preset when the route is built. In addition the function may have arguments that are extracted from the environment. For instance `ReqValidCommentID` may be given the content of the `id` field from the body of a JSON payload.

The string representation of the signature is:

- The function name if there are no arguments provided when building the routes. For instance `APIAuthorization`
- The function name followed by a whitespace list of arguments provided when building the routes. For instance `TokenRequiresScopes Repository User`

## Fixtures helpers

All fixtures are dynamically created (they are not using the global fixtures found in `models/fixtures`). The `fixture.go` file contains all the helpers to create those fixtures.

## Debugging

- Running the tests in verbose mode `GOTESTCOMPILEDRUNSUFFIX=-test.v TAGS='sqlite sqlite_unlock_notify' make 'test-sqlite#TestAPIv1Permissions'`
- Browsing the tests such as
```
...
=== RUN   TestAPIv1Permissions/APIAuthorization,TokenRequiresScopes_Admin_fixture_0
    functions.go:95: creating fixture data from doer:doerregular,level:read,scope:read:admin
    functions.go:98: created fixture data doer:doerregular,level:read,scope:read:admin
    functions.go:105: 	*auth.AccessToken(ID=10 Token=e26bfc1190efcf8c36ef640659af33e87073032c)
    functions.go:105: 	*user.User(Name=doerregular)
    functions.go:105: 	isSigned(true)
    functions.go:105: 	*tests_test.accessTokenAuthenticationResult(*user.User(Name=doerregular) auth.AccessTokenScope(read:admin) *authz.AllAccessAuthorizationReducer)
    fixture.go:637: calling permissions.APIAuthorization(ctx)
    functions.go:131: 	+ *authz.AllAccessAuthorizationReducer
    token_requires_scopes_test.go:67: calling TokenRequiresScopes(ctx, [1], 1)
    functions.go:131: 	+ []auth.AccessTokenScopeCategory([1])
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

`func(t *testing.T, ctx apiv1_permissions.Context, permissions *apiv1_permissions.Permissions, data *testData, signature []any)`

It is responsible for:

- Calling `t.Logf` to display the call about to be made
- Calling the function using `ctx` as a first argument

The `permissions` and `data` arguments are provided, as computed by the `interpret` function.

The `signature[0]` is the function itself and could be called with `signature[0].Call`.

The `signature[1:]` list are the mandatory arguments to the function call.

## Test coverage

### `routers/api/v1/permissions`

- At the root of the source tree
- `COVERAGE_TEST_PACKAGES="forgejo.org/routers/api/v1/permissions forgejo.org/routers/api/v1/permissions/tests" make coverage-run coverage-show-percentage | grep v1/permissions | grep -v v1/permissions/permissions.go | grep -v v1/permissions/tests | sed -e 's/\t\t*/ /g' -e 's|forgejo.org/routers/||'`
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
