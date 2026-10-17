// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package method

import (
	"testing"
	"time"

	"forgejo.org/modules/json"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-world Forgejo Actions claims, Forgejo v15
const forgejoClaims = `
{
  "actor": "coolguy",
  "aud": "https://example.org/-/coolguy/authorized-integration/346e1496",
  "base_ref": "main",
  "event_name": "pull_request",
  "exp": 1776979110,
  "head_ref": "forgejo-oidc-test",
  "iat": 1776975510,
  "iss": "https://example.org/api/actions",
  "nbf": 1776975510,
  "ref": "refs/pull/113/head",
  "ref_protected": "false",
  "ref_type": "",
  "repository": "coolguy/test",
  "repository_owner": "coolguy",
  "run_attempt": "4",
  "run_id": "3572",
  "run_number": "2054",
  "sha": "d4083cc0f4e7452dc00f7a5f73ec5486a549adb9",
  "sub": "repo:coolguy/test:pull_request",
  "workflow": "main.yml",
  "workflow_ref": "coolguy/test/.forgejo/workflows/main.yml@refs/pull/113/head"
}
`

// Real-world GitHub Actions claims
const githubClaims = `
{
  "actor": "coolguy",
  "actor_id": "91093",
  "aud": "https://example.org/-/coolguy/authorized-integration/6cc55ba0",
  "base_ref": "main",
  "check_run_id": "72783197645",
  "event_name": "pull_request",
  "exp": 1776980496,
  "head_ref": "github-oidc-test",
  "iat": 1776980196,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "coolguy/forgejo-runner-testrepo/.github/workflows/main.yml@refs/pull/3/merge",
  "job_workflow_sha": "62a34e2bf42fda53a0209bfd485dcab3013b1160",
  "jti": "83545042-379e-4328-8e60-3b6d46594a5f",
  "nbf": 1776979896,
  "ref": "refs/pull/3/merge",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "coolguy/forgejo-runner-testrepo",
  "repository_id": "1113890566",
  "repository_owner": "coolguy",
  "repository_owner_id": "91093",
  "repository_visibility": "private",
  "run_attempt": "8",
  "run_id": "24846522812",
  "run_number": "10",
  "runner_environment": "github-hosted",
  "sha": "62a34e2bf42fda53a0209bfd485dcab3013b1160",
  "sub": "repo:coolguy/forgejo-runner-testrepo:pull_request",
  "workflow": ".github/workflows/main.yml",
  "workflow_ref": "coolguy/forgejo-runner-testrepo/.github/workflows/main.yml@refs/pull/3/merge",
  "workflow_sha": "62a34e2bf42fda53a0209bfd485dcab3013b1160"
}
`

// Real-world AWS Federated Web Identity claims
const awsClaims = `
{
  "aud": "https://example.org/-/coolguy/authorized-integration/7895835c",
  "sub": "arn:aws:iam::1234567890:role/service-role/forgejo-oidc-accepting-test-role-x7t3fgko",
  "https://sts.amazonaws.com/": {
    "aws_account": "1234567890",
    "original_session_exp": "2026-04-24T09:34:34Z",
    "source_region": "us-west-2",
    "principal_id": "arn:aws:iam::1234567890:role/service-role/forgejo-oidc-accepting-test-role-x7t3fgko",
    "lambda_source_function_arn": "arn:aws:lambda:us-west-2:1234567890:function:forgejo-oidc-accepting-test"
  },
  "iss": "https://a103a2cc-b461-473d-84fe-6c4f6d45af88.tokens.sts.global.api.aws",
  "exp": 1776980375,
  "iat": 1776980075,
  "jti": "0afcbeb7-512d-479f-b596-703c03ae65a5"
}
`

func TestFlexibleClaimsUnmarshal(t *testing.T) {
	t.Run("Forgejo", func(t *testing.T) {
		var retval flexibleClaims
		data := []byte(forgejoClaims)
		require.NoError(t, json.Unmarshal(data, &retval))
		// assert the claims that are handled specially in flexibleClaims UnmarshalJSON
		assert.Equal(t, "https://example.org/api/actions", retval.Issuer)
		assert.Equal(t, "repo:coolguy/test:pull_request", retval.Subject)
		assert.Equal(t, jwt.ClaimStrings{"https://example.org/-/coolguy/authorized-integration/346e1496"}, retval.Audience)
		assert.IsType(t, &jwt.NumericDate{}, retval.ExpiresAt)
		assert.IsType(t, &jwt.NumericDate{}, retval.NotBefore)
		assert.IsType(t, &jwt.NumericDate{}, retval.IssuedAt)
		assert.Equal(t, time.Date(2026, time.April, 23, 21, 18, 30, 0, time.UTC), retval.ExpiresAt.Time.UTC())
		assert.Equal(t, time.Date(2026, time.April, 23, 20, 18, 30, 0, time.UTC), retval.NotBefore.Time.UTC())
		assert.Equal(t, time.Date(2026, time.April, 23, 20, 18, 30, 0, time.UTC), retval.IssuedAt.Time.UTC())
		assert.Empty(t, retval.ID)
		// short check that the 'other' claims were stored as well
		assert.Equal(t, "d4083cc0f4e7452dc00f7a5f73ec5486a549adb9", retval.other["sha"])
		assert.Len(t, retval.other, 15)
	})
	t.Run("GitHub", func(t *testing.T) {
		var retval flexibleClaims
		data := []byte(githubClaims)
		require.NoError(t, json.Unmarshal(data, &retval))
		// assert the claims that are handled specially in flexibleClaims UnmarshalJSON
		assert.Equal(t, "https://token.actions.githubusercontent.com", retval.Issuer)
		assert.Equal(t, "repo:coolguy/forgejo-runner-testrepo:pull_request", retval.Subject)
		assert.Equal(t, jwt.ClaimStrings{"https://example.org/-/coolguy/authorized-integration/6cc55ba0"}, retval.Audience)
		assert.IsType(t, &jwt.NumericDate{}, retval.ExpiresAt)
		assert.IsType(t, &jwt.NumericDate{}, retval.NotBefore)
		assert.IsType(t, &jwt.NumericDate{}, retval.IssuedAt)
		assert.Equal(t, time.Date(2026, time.April, 23, 21, 41, 36, 0, time.UTC), retval.ExpiresAt.Time.UTC())
		assert.Equal(t, time.Date(2026, time.April, 23, 21, 31, 36, 0, time.UTC), retval.NotBefore.Time.UTC())
		assert.Equal(t, time.Date(2026, time.April, 23, 21, 36, 36, 0, time.UTC), retval.IssuedAt.Time.UTC())
		assert.Equal(t, "83545042-379e-4328-8e60-3b6d46594a5f", retval.ID)
		// short check that the 'other' claims were stored as well
		assert.Equal(t, "62a34e2bf42fda53a0209bfd485dcab3013b1160", retval.other["sha"])
		assert.Len(t, retval.other, 24)
	})
	t.Run("AWS", func(t *testing.T) {
		var retval flexibleClaims
		data := []byte(awsClaims)
		require.NoError(t, json.Unmarshal(data, &retval))
		// assert the claims that are handled specially in flexibleClaims UnmarshalJSON
		assert.Equal(t, "https://a103a2cc-b461-473d-84fe-6c4f6d45af88.tokens.sts.global.api.aws", retval.Issuer)
		assert.Equal(t, "arn:aws:iam::1234567890:role/service-role/forgejo-oidc-accepting-test-role-x7t3fgko", retval.Subject)
		assert.Equal(t, jwt.ClaimStrings{"https://example.org/-/coolguy/authorized-integration/7895835c"}, retval.Audience)
		assert.IsType(t, &jwt.NumericDate{}, retval.ExpiresAt)
		assert.Nil(t, retval.NotBefore)
		assert.IsType(t, &jwt.NumericDate{}, retval.IssuedAt)
		assert.Equal(t, time.Date(2026, time.April, 23, 21, 39, 35, 0, time.UTC), retval.ExpiresAt.Time.UTC())
		assert.Equal(t, time.Date(2026, time.April, 23, 21, 34, 35, 0, time.UTC), retval.IssuedAt.Time.UTC())
		assert.Equal(t, "0afcbeb7-512d-479f-b596-703c03ae65a5", retval.ID)
		// short check that the 'other' claims were stored as well
		assert.Equal(t, "1234567890", retval.other["https://sts.amazonaws.com/"].(map[string]any)["aws_account"])
		assert.Len(t, retval.other, 1)
	})
}
