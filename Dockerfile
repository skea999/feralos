# FeralOS CI image (P0-3). Multi-stage:
#   lint-tools — pinned golangci-lint + govulncheck (same versions as the
#                dev host, so local pre-commit and CI never drift)
#   shell      — pinned shellcheck (P3-3 gate)
#   test       — Go 1.22 + pre-warmed module caches + repo + tools;
#                default CMD runs scripts/ci.sh (the full gate)
# Build: docker build -t feralos-ci .
# Run:   docker run --rm feralos-ci bash scripts/ci.sh

# --- lint tools (pinned) ----------------------------------------------------
FROM golang:1.22-bookworm AS lint-tools
# golangci-lint pinned to the dev-host version (v1.64.8; binary built with a
# newer Go is fine — it type-checks the go1.22 modules exactly like the host)
ARG GOLANGCI_LINT_VERSION=v1.64.8
# govulncheck pinned to the dev-host version (v1.7.0)
ARG GOVULNCHECK_VERSION=v1.7.0
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
# GOTOOLCHAIN=auto: x/vuln v1.7.0 declares go >= 1.25 (the golang image pins
# GOTOOLCHAIN=local); the download happens once, inside this cached layer.
# The govulncheck BINARY then analyzes our go1.22 modules with the local
# go1.22 toolchain, same as the dev host.
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
      | sh -s -- -b /usr/local/bin "${GOLANGCI_LINT_VERSION}" \
 && GOTOOLCHAIN=auto go install golang.org/x/vuln/cmd/govulncheck@"${GOVULNCHECK_VERSION}" \
 && golangci-lint version \
 && govulncheck -version

# --- shellcheck (P3-3) -------------------------------------------------------
FROM koalaman/shellcheck:v0.10.0 AS shell

# --- test + gate runner -------------------------------------------------------
FROM golang:1.22-bookworm AS test
WORKDIR /src

# module layers first (cached until a go.mod/go.sum changes)
COPY src/go.mod src/go.sum src/
COPY tests/harness/go.mod tests/harness/go.sum tests/harness/
COPY tests/debug_run/go.mod tests/debug_run/go.sum tests/debug_run/
RUN cd src \
 && go mod download \
 && cd ../tests/harness && go mod download \
 && cd ../../tests/debug_run && go mod download

COPY . .
COPY --from=lint-tools /usr/local/bin/golangci-lint /usr/local/bin/golangci-lint
COPY --from=lint-tools /go/bin/govulncheck /usr/local/bin/govulncheck
COPY --from=shell /bin/shellcheck /usr/local/bin/shellcheck

CMD ["bash", "scripts/ci.sh"]
