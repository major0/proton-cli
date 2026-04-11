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

# Ensures the pushed tag is annotated. If it's lightweight, replaces it
# with an annotated tag pointing at the same commit.
#
# Usage: release-ensure-annotated.sh <tag>
# Requires: GH_TOKEN or git push credentials

TAG="${1}"
test -n "${TAG}" || die 'TAG is required'

TYPE="$(git cat-file -t "${TAG}")"

if test "${TYPE}" = 'tag'; then
  echo "::notice::${TAG} is already annotated"
  exit 0
fi

echo "::warning::${TAG} is a lightweight tag — converting to annotated"

COMMIT="$(git rev-parse "${TAG}^{commit}")"

# Delete the lightweight tag locally and remotely
git tag -d "${TAG}" || die "failed to delete local tag ${TAG}"
git push origin ":refs/tags/${TAG}" || die "failed to delete remote tag ${TAG}"

# Re-create as annotated and push
git tag -a "${TAG}" "${COMMIT}" -m "${TAG}" || die "failed to create annotated tag ${TAG}"
git push origin "${TAG}" || die "failed to push annotated tag ${TAG}"

echo "::notice::${TAG} converted to annotated tag on ${COMMIT}"

unset TAG TYPE COMMIT
