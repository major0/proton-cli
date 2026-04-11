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

# Finds the previous tag in the same series for changelog generation.
#
# RC tags: find the nearest ancestor tag.
# Stable tags: skip past RC tags to find the previous stable release.
#
# Usage: release-find-previous.sh <tag>
# Outputs: tag

TAG="${1}"
test -n "${TAG}" || die 'TAG is required'
test -n "${GITHUB_OUTPUT:-}" || die 'GITHUB_OUTPUT is not set'

PREV=''

case "${TAG}" in
(*-rc*)
  PREV="$(git describe --tags --abbrev=0 --match 'v*' "${TAG}^" 2>/dev/null || true)"
  ;;
(*)
  COMMIT="${TAG}^"
  I=0
  while test "${I}" -lt 50; do
    CANDIDATE="$(git describe --tags --abbrev=0 --match 'v*' "${COMMIT}" 2>/dev/null || true)"
    test -n "${CANDIDATE}" || break
    case "${CANDIDATE}" in
    (*-rc*)
      COMMIT="${CANDIDATE}^"
      ;;
    (*)
      PREV="${CANDIDATE}"
      break
      ;;
    esac
    I=$((I + 1))
  done
  unset COMMIT I CANDIDATE
  ;;
esac

if test -z "${PREV}"; then
  echo '::notice::No previous tag found; changelog will cover full history'
else
  echo "::notice::Previous tag: ${PREV}"
fi

printf 'tag=%s\n' "${PREV}" >> "${GITHUB_OUTPUT}"

unset TAG PREV
