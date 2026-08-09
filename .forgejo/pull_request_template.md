---

name: "Pull Request Template"
about: "Template for all Pull Requests"
labels:

- test/needed

---

## Checklist

The [contributor guide](https://forgejo.org/docs/next/contributor/) contains information that will be helpful to first time contributors. All work and communication must conform to Forgejo's [AI Agreement](https://codeberg.org/forgejo/governance/src/branch/main/AIAgreement.md). There also are a few [conditions for merging Pull Requests in Forgejo repositories](https://codeberg.org/forgejo/governance/src/branch/main/PullRequestsAgreement.md). You are also welcome to join the [Forgejo development chatroom](https://matrix.to/#/#forgejo-development:matrix.org).

### Compliance

- [ ] I confirm that I make this contribution in accordance with [Forgejo's AI Agreement](https://codeberg.org/forgejo/governance/src/commit/9084720dc6a7fbf1d3f358919d46bd6adfb55287/AIAgreement.md).

### Tests for Go changes

(can be removed for JavaScript changes)

- I added test coverage for Go changes...
  - [ ] in their respective `*_test.go` for unit tests.
  - [ ] in the `tests/integration` directory if it involves interactions with a live Forgejo server.
- I ran...
  - [ ] `make pr-go` before pushing

### Tests for JavaScript changes

(can be removed for Go changes)

- I added test coverage for JavaScript changes...
  - [ ] in `web_src/js/*.test.js` if it can be unit tested.
  - [ ] in `tests/e2e/*.test.e2e.js` if it requires interactions with a live Forgejo server (see also the [developer guide for JavaScript testing](https://codeberg.org/forgejo/forgejo/src/branch/forgejo/tests/e2e/README.md#end-to-end-tests)).

### Documentation

- [ ] I created a pull request [to the documentation](https://codeberg.org/forgejo/docs) to explain to Forgejo users how to use this change.
- [ ] I did not document these changes and I do not expect someone else to do it.

### Release notes

- [ ] This change will be noticed by a Forgejo user or admin (feature, bug fix, performance, etc.). I suggest to include a release note for this change.
- [ ] This change is not visible to a Forgejo user or admin (refactor, dependency upgrade, etc.). I think there is no need to add a release note for this change.

*The decision if the pull request will be shown in the release notes is up to the mergers / release team.*

The content of the `release-notes/<pull request number>.md` file will serve as the basis for the release notes. If the file does not exist, the title of the pull request will be used instead.
