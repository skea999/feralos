# FeralOS CI image (P0-3). Multi-stage:
#   lint-tools — pinned golangci-lint + govulncheck (same versions as the
#                dev host, so local pre-commit and CI never drift) + the
#                pinned static-analysis suite (gitleaks, typos, yamllint,
#                markdownlint-cli2 on Node, editorconfig-checker)
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
# static-analysis suite (ursine pattern) — exact-pinned versions
ARG GITLEAKS_VERSION=8.30.1
ARG TYPOS_VERSION=1.51.1
ARG EDITORCONFIG_CHECKER_VERSION=4.0.2
ARG YAMLLINT_VERSION=1.38.0
# Node LTS + markdownlint-cli2 (0.24.0 requires node >= 22; bookworm apt
# would only give node 18, so the official tarball is installed)
ARG NODE_VERSION=22.23.4
ARG MARKDOWNLINT_CLI2_VERSION=0.24.0
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
# tarball Node lives in /opt/node — npm's #!/usr/bin/env node needs it on
# PATH already during this stage (the test stage gets its own ENV below)
ENV PATH="/opt/node/bin:${PATH}"
# GOTOOLCHAIN=auto: x/vuln v1.7.0 declares go >= 1.25 (the golang image pins
# GOTOOLCHAIN=local); the download happens once, inside this cached layer.
# The govulncheck BINARY then analyzes our go1.22 modules with the local
# go1.22 toolchain, same as the dev host.
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
      | sh -s -- -b /usr/local/bin "${GOLANGCI_LINT_VERSION}" \
  && GOTOOLCHAIN=auto go install golang.org/x/vuln/cmd/govulncheck@"${GOVULNCHECK_VERSION}" \
  && golangci-lint version \
  && govulncheck -version

# static-analysis tools: gitleaks/typos/editorconfig-checker = release
# binaries into /usr/local/bin; yamllint = self-contained python venv at
# /opt/yamllint (venv python symlinks the distro python3 — the test stage
# installs python3 too); markdownlint-cli2 = npm -g under the tarball Node
# at /opt/node (whole tree copied into the test stage).
RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 python3-venv xz-utils \
  && curl -sSfL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" \
      | tar -xz -C /tmp gitleaks \
  && curl -sSfL "https://github.com/crate-ci/typos/releases/download/v${TYPOS_VERSION}/typos-v${TYPOS_VERSION}-x86_64-unknown-linux-musl.tar.gz" \
      | tar -xz -C /tmp ./typos \
  && curl -sSfL "https://github.com/editorconfig-checker/editorconfig-checker/releases/download/v${EDITORCONFIG_CHECKER_VERSION}/editorconfig-checker-linux-amd64.tar.gz" \
      | tar -xz -C /tmp editorconfig-checker \
  && install -m 0755 /tmp/gitleaks /tmp/typos /tmp/editorconfig-checker /usr/local/bin/ \
  && ln -sf /usr/local/bin/editorconfig-checker /usr/local/bin/ec \
  && rm -f /tmp/gitleaks /tmp/typos /tmp/editorconfig-checker \
  && python3 -m venv /opt/yamllint \
  && /opt/yamllint/bin/pip install --no-cache-dir "yamllint==${YAMLLINT_VERSION}" \
  && mkdir -p /opt/node \
  && curl -sSfL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" \
      | tar -xJ -C /opt/node --strip-components=1 \
  && /opt/node/bin/npm install -g --prefix /opt/node "markdownlint-cli2@${MARKDOWNLINT_CLI2_VERSION}" \
  && gitleaks version \
  && typos --version \
  && ec --version \
  && /opt/yamllint/bin/yamllint --version \
  && /opt/node/bin/markdownlint-cli2 --version \
  && rm -rf /var/lib/apt/lists/*

# --- shellcheck (P3-3) -------------------------------------------------------
FROM koalaman/shellcheck:v0.10.0 AS shell

# --- test + gate runner -------------------------------------------------------
FROM golang:1.22-bookworm AS test
WORKDIR /src

# runtime interpreter for the copied yamllint venv (venv python -> python3)
RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 \
  && rm -rf /var/lib/apt/lists/*

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
# static-analysis suite (ursine pattern)
COPY --from=lint-tools /usr/local/bin/gitleaks /usr/local/bin/gitleaks
COPY --from=lint-tools /usr/local/bin/typos /usr/local/bin/typos
COPY --from=lint-tools /usr/local/bin/editorconfig-checker /usr/local/bin/editorconfig-checker
COPY --from=lint-tools /usr/local/bin/ec /usr/local/bin/ec
COPY --from=lint-tools /opt/yamllint /opt/yamllint
COPY --from=lint-tools /opt/node /opt/node
ENV PATH="/opt/yamllint/bin:/opt/node/bin:${PATH}"

CMD ["bash", "scripts/ci.sh"]
