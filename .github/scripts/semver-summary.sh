#!/bin/sh
set -eu
POSIXLY_CORRECT='no bashing shell'

##
# ${*} = message to print to stderr
# returns: 0
error() {
  : "error(msg='${*}')"
  echo "error: ${*}" >&2
}

##
# ${*} = fatal message
# returns: does not return (exits 1)
die() {
  : "die(msg='${*}')"
  error "${*}"
  exit 1
}

# Writes a semver summary to GITHUB_STEP_SUMMARY.
#
# Usage: semver-summary.sh <tag> <tag-type>

TAG="${1}"
TAG_TYPE="${2}"

test -n "${TAG}" || die 'TAG is required'
test -n "${TAG_TYPE}" || die 'TAG_TYPE is required'
test -n "${GITHUB_STEP_SUMMARY:-}" || die 'GITHUB_STEP_SUMMARY is not set'

printf '### Semver Summary\n- **Tag**: %s\n- **Type**: %s\n' "${TAG}" "${TAG_TYPE}" >> "${GITHUB_STEP_SUMMARY}"

unset TAG TAG_TYPE
