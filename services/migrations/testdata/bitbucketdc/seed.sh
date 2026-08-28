#!/usr/bin/env bash
# Seeds the Bitbucket Data Center repository used to record the fixtures of
# TestBitbucketDataCenterDownloadRepo (services/migrations/bitbucketdc_test.go).
#
# Bitbucket Data Center has no public SaaS instance, so the fixtures are recorded against
# any instance you control.
#
# Requirements:
#   - Bitbucket Data Center >= 9.2 (multi-line comments)
#   - two accounts: an author (creates the repository and pull requests) and a reviewer
#     (replies, marks "needs work", approves then withdraws)
#   - a clean slate: delete both $PROJECT/test-repo and the author's personal fork
#     (~author/test-repo) before re-running
#
# Environment:
#   BITBUCKET_DC_URL             base URL, e.g. https://bitbucket.example.com
#   BITBUCKET_DC_AUTHOR          username of the author (used for the git pushes)
#   BITBUCKET_DC_TOKEN           HTTP access token of the author
#   BITBUCKET_DC_REVIEWER        username (slug) of the reviewer
#   BITBUCKET_DC_REVIEWER_TOKEN  HTTP access token of the reviewer
#   BITBUCKET_DC_PROJECT         project key (default: MIGR)
#
# Recording the fixtures once this script has run:
#   BITBUCKET_DC_URL=... BITBUCKET_DC_TOKEN=... \
#     go test -tags 'sqlite sqlite_unlock_notify' ./services/migrations/ -run TestBitbucketDataCenterDownloadRepo
# then run ./sanitize.sh to strip the instance host and the seed accounts from the fixtures.
set -euo pipefail

: "${BITBUCKET_DC_URL:?}" "${BITBUCKET_DC_AUTHOR:?}" "${BITBUCKET_DC_TOKEN:?}"
: "${BITBUCKET_DC_REVIEWER:?}" "${BITBUCKET_DC_REVIEWER_TOKEN:?}"
PROJECT="${BITBUCKET_DC_PROJECT:-MIGR}"
REPO=test-repo
API="$BITBUCKET_DC_URL/rest/api/1.0/projects/$PROJECT/repos/$REPO"

as_author()   { curl -sfS -H "Authorization: Bearer $BITBUCKET_DC_TOKEN" -H 'Content-Type: application/json' "$@"; }
as_reviewer() { curl -sfS -H "Authorization: Bearer $BITBUCKET_DC_REVIEWER_TOKEN" -H 'Content-Type: application/json' "$@"; }
# Extracts a field from the JSON on stdin.
field() { python3 -c "import json,sys; print(json.load(sys.stdin)['$1'])"; }
pr_version() { as_author "$API/pull-requests/$1" | field version; }
step() { echo ">>> $*" >&2; }

# --- project and repository -----------------------------------------------------------------
step "Creating project $PROJECT (ignored if it already exists)"
as_author -X POST "$BITBUCKET_DC_URL/rest/api/1.0/projects" \
	-d "{\"key\": \"$PROJECT\", \"name\": \"Migration fixtures\"}" > /dev/null || true
step "Creating repository $PROJECT/$REPO"
as_author -X POST "$BITBUCKET_DC_URL/rest/api/1.0/projects/$PROJECT/repos" \
	-d "{\"name\": \"$REPO\", \"defaultBranch\": \"main\"}" > /dev/null
# Best effort: on a freshly created private project the reviewer needs access; on a
# pre-existing project (BITBUCKET_DC_PROJECT) they may already have it.
step "Granting $BITBUCKET_DC_REVIEWER access to the project"
as_author -X PUT "$BITBUCKET_DC_URL/rest/api/1.0/projects/$PROJECT/permissions/users?name=$BITBUCKET_DC_REVIEWER&permission=PROJECT_WRITE" || true

# --- git content: main and one branch per pull request ---------------------------------------
step "Pushing the git content (main and three feature branches)"
workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
git -C "$workdir" init -q -b main
for i in $(seq 1 10); do echo "line $i"; done > "$workdir/README.md"
git -C "$workdir" add README.md
git -C "$workdir" commit -qm 'Initial content'

# tokens may contain '/' or '+'
token_enc=$(python3 -c "import urllib.parse, os; print(urllib.parse.quote(os.environ['BITBUCKET_DC_TOKEN'], safe=''))")

git -C "$workdir" remote add origin "$(echo "$BITBUCKET_DC_URL" | sed "s|://|://$BITBUCKET_DC_AUTHOR:$token_enc@|")/scm/${PROJECT,,}/$REPO.git"
git -C "$workdir" push -qu origin main

git -C "$workdir" checkout -qb feature/one
sed -i '/^line 2$/d' "$workdir/README.md"        # a removed line, commented on the old side
echo "line 11" >> "$workdir/README.md"
git -C "$workdir" commit -qam 'First feature'
git -C "$workdir" push -q origin feature/one

git -C "$workdir" checkout -q main
git -C "$workdir" checkout -qb feature/two
echo "feature two" >> "$workdir/README.md"
git -C "$workdir" commit -qam 'Second feature'
git -C "$workdir" push -q origin feature/two

git -C "$workdir" checkout -q main
git -C "$workdir" checkout -qb feature/three
echo "feature three" >> "$workdir/README.md"
git -C "$workdir" commit -qam 'Third feature'
git -C "$workdir" push -q origin feature/three

new_pr() { # title, branch -> id
	as_author -X POST "$API/pull-requests" -d "{
		\"title\": \"$1\", \"description\": \"Description of $1\",
		\"fromRef\": {\"id\": \"refs/heads/$2\"}, \"toRef\": {\"id\": \"refs/heads/main\"}
	}" | field id
}

# --- PR 1 (kept open): comments of every kind, "needs work", withdrawn approval --------------
step "Opening pull request 'First feature'"
pr1=$(new_pr 'First feature' feature/one)
step "PR $pr1: adding a general comment and its reply"
parent=$(as_author -X POST "$API/pull-requests/$pr1/comments" -d '{"text": "A general question"}' | field id)
as_reviewer -X POST "$API/pull-requests/$pr1/comments" \
	-d "{\"text\": \"A general answer\", \"parent\": {\"id\": $parent}}" > /dev/null
step "PR $pr1: adding a multi-line comment on README.md lines 2-4 and its reply"

# Despite the OpenAPI spec marking both fields read-only, the creation requires multilineMarker
# AND a pre-computed multilineSpan (src/dst = old/new file line numbers), all within a diff hunk.
parent=$(as_author -X POST "$API/pull-requests/$pr1/comments" -d '{
	"text": "This block needs a rewrite",
	"anchor": {"diffType": "EFFECTIVE", "path": "README.md", "line": 4, "lineType": "CONTEXT", "fileType": "TO",
	           "multilineMarker": {"startLine": 2, "startLineType": "CONTEXT"},
	           "multilineSpan": {"srcSpanStart": 3, "srcSpanEnd": 5, "dstSpanStart": 2, "dstSpanEnd": 4}}
}' | field id)
as_reviewer -X POST "$API/pull-requests/$pr1/comments" \
	-d "{\"text\": \"Rewritten in the next push\", \"parent\": {\"id\": $parent}}" > /dev/null
step "PR $pr1: adding a comment on the removed line 2 (old side of the diff)"
as_reviewer -X POST "$API/pull-requests/$pr1/comments" -d '{
	"text": "Why was this removed?",
	"anchor": {"diffType": "EFFECTIVE", "path": "README.md", "line": 2, "lineType": "REMOVED", "fileType": "FROM"}
}' > /dev/null
step "PR $pr1: approving, withdrawing the approval, then marking as 'needs work'"
as_reviewer -X PUT "$API/pull-requests/$pr1/participants/$BITBUCKET_DC_REVIEWER" -d '{"status": "APPROVED"}' > /dev/null
as_reviewer -X PUT "$API/pull-requests/$pr1/participants/$BITBUCKET_DC_REVIEWER" -d '{"status": "UNAPPROVED"}' > /dev/null
as_reviewer -X PUT "$API/pull-requests/$pr1/participants/$BITBUCKET_DC_REVIEWER" -d '{"status": "NEEDS_WORK"}' > /dev/null

# --- PR 2: merged / PR 3: declined ------------------------------------------------------------
step "Opening and merging pull request 'Second feature'"
pr2=$(new_pr 'Second feature' feature/two)
as_author -X POST "$API/pull-requests/$pr2/merge" -d "{\"version\": $(pr_version "$pr2")}" > /dev/null
step "Opening and declining pull request 'Third feature'"
pr3=$(new_pr 'Third feature' feature/three)
as_author -X POST "$API/pull-requests/$pr3/decline" -d "{\"version\": $(pr_version "$pr3")}" > /dev/null

# --- PR 4: from a fork in the author's personal project --------------------------------------
step "Forking the repository into the author's personal project"
as_author -X POST "$API" -d '{"name": "test-repo"}' > /dev/null
sleep 2
step "Pushing a branch to the fork and opening pull request 'Fork feature'"
git -C "$workdir" checkout -q main
git -C "$workdir" checkout -qb feature/fork
echo "fork feature" >> "$workdir/README.md"
git -C "$workdir" commit -qam 'Fork feature'
git -C "$workdir" remote add fork "$(echo "$BITBUCKET_DC_URL" | sed "s|://|://$BITBUCKET_DC_AUTHOR:$token_enc@|")/scm/~${BITBUCKET_DC_AUTHOR,,}/$REPO.git"
git -C "$workdir" push -q fork feature/fork
pr4=$(as_author -X POST "$API/pull-requests" -d "{
	\"title\": \"Fork feature\", \"description\": \"Description of Fork feature\",
	\"fromRef\": {\"id\": \"refs/heads/feature/fork\",
	              \"repository\": {\"slug\": \"$REPO\", \"project\": {\"key\": \"~${BITBUCKET_DC_AUTHOR^^}\"}}},
	\"toRef\": {\"id\": \"refs/heads/main\"}
}" | field id)

echo "Seeded $BITBUCKET_DC_URL/projects/$PROJECT/repos/$REPO (pull requests $pr1, $pr2, $pr3, $pr4)"
