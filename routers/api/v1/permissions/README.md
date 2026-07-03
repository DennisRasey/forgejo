Permissions check functions for `routers/api/v1`

## What is it?

A collection of functions implementing permissions checks tasked to
decide if the `Doer` of a REST API endpoint is granted permissions or
not.

They are called from middleware and provided with a
[context](interface.go) that provides access to the `Doer` and other
information extracted from the request such as the organization or the
repository targeted by the endpoint.

Upon deciding permissions is not granted, the function is expected to
set an error in the same way a middleware would, using [context
methods](interface.go) such as `ctx.Error(http.StatusForbidden,
...)`. If permission is granted, the function just returns and the
REST API router will continue by calling the next middleware in the
sequence until it reaches the handler.

More information about this package is available in [the issue
describing its design](https://codeberg.org/forgejo/design/issues/63).

## How to add a new function?

The `ReqToken` function implemented in `req_token.go` can be used as an example.

It is called from the [`reqToken` middleware](https://codeberg.org/forgejo/forgejo/src/commit/958cface135f789600146d14e33491ead4029055/routers/api/v1/api.go#L279-L281) which is inserted in the REST API routes such as [`GET /orgs/{org}/actions/secrets`](https://codeberg.org/forgejo/forgejo/src/commit/958cface135f789600146d14e33491ead4029055/routers/api/v1/api.go#L490).

The tests verifying it works as intended are found in `tests/req_token_test.go` and use the a dedicated framework that [has its own documentation](tests/README.md).

To get started with a new function, it is recommended to:

- Look for a function that is similar to the one to be implemented.
- Copy/paste and rename the implementation and the tests.
- Run the tests for this new function as explained in the `tests/README.md` file.
- Continue from there.
