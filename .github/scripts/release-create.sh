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

# Creates a GitHub release with auto-generated changelog.
#
# Usage: release-create.sh <tag> <name> <prev-tag>
# Requires: GH_TOKEN (env)

TAG="${1}"
NAME="${2}"
PREV="${3}"

test -n "${TAG}" || die 'TAG is required'
test -n "${NAME}" || die 'NAME is required'

ARGS="${TAG} --title '${NAME} ${TAG}' --generate-notes"

# If we found a previous tag, scope the changelog to that range
if test -n "${PREV}"; then
  ARGS="${ARGS} --notes-start-tag '${PREV}'"
fi

# Mark pre-release for rc tags
case "${TAG}" in
(*-rc*)
  ARGS="${ARGS} --prerelease"
  ;;
esac

eval gh release create ${ARGS}

unset TAG NAME PREV ARGS
