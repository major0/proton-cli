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

# Installs Go development tools (goimports, golangci-lint).
# Appends GOPATH/bin to GITHUB_PATH if running in CI.

go install golang.org/x/tools/cmd/goimports@latest || die 'failed to install goimports'

GOBIN="$(go env GOPATH)/bin"
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "${GOBIN}" latest || die 'failed to install golangci-lint'

if test -n "${GITHUB_PATH:-}"; then
	echo "${GOBIN}" >> "${GITHUB_PATH}"
fi

unset GOBIN
