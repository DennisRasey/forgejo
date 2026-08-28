#!/usr/bin/env bash
# Sanitizes the fixtures recorded by TestBitbucketDataCenterDownloadRepo before they are
# committed: replaces the instance host (http and ssh clone links alike) and the seed
# accounts with neutral values. The test asserts no exact user name, e-mail or display
# name, so wholesale replacement is safe.
#
# Environment (same values as for seed.sh):
#   BITBUCKET_DC_URL       base URL the fixtures were recorded against
#   BITBUCKET_DC_AUTHOR    username of the author account
#   BITBUCKET_DC_REVIEWER  username of the reviewer account
set -euo pipefail

: "${BITBUCKET_DC_URL:?}" "${BITBUCKET_DC_AUTHOR:?}" "${BITBUCKET_DC_REVIEWER:?}"
cd "$(dirname "$0")/full_download"

host=$(echo "$BITBUCKET_DC_URL" | sed -E 's|^https?://||')
# Personal project keys surface the username in uppercase (~AUTHOR): replace both cases.
sed -i "s|$host|bitbucket.example.com|g;
        s|$BITBUCKET_DC_AUTHOR|test-author|g;
        s|${BITBUCKET_DC_AUTHOR^^}|TEST-AUTHOR|g;
        s|$BITBUCKET_DC_REVIEWER|test-reviewer|g;
        s|${BITBUCKET_DC_REVIEWER^^}|TEST-REVIEWER|g" ./*
sed -i -E 's|"emailAddress": *"[^"]*"|"emailAddress":"user@example.com"|g;
           s|"displayName": *"[^"]*"|"displayName":"Test User"|g;
           s|"name": *"[^"]*", *"type": *"PERSONAL"|"name":"Test User","type":"PERSONAL"|g' ./*

if remains=$(grep -ril "$host\|$BITBUCKET_DC_AUTHOR\|$BITBUCKET_DC_REVIEWER" .); then
	echo "instance data still present in: $remains" >&2
	exit 1
fi
echo "Fixtures sanitized."
