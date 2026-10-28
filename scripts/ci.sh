#!/usr/bin/env bash
# ci.sh — the CI gate (P0-3). Mirrors .githooks/pre-commit (go vet +
# golangci-lint + govulncheck + secret scan) and adds gofmt, go test, all
# binary builds, shellcheck (P3-3), the docs checks and the static-analysis
# suite (yamllint + gitleaks = blocking; typos, markdownlint-cli2,
# editorconfig-checker, taplo = warn-only). Runs inside the feralos-ci image
# (see Dockerfile) but works on any host with Go 1.22, golangci-lint,
# govulncheck, shellcheck and bash.
#
# Builds go to a temp dir so the repo tree stays clean.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="$(mktemp -d)"
trap 'rm -rf "${OUT:?}"' EXIT
BIN="$OUT/bin"
mkdir -p "$BIN"

fail() { echo "✗ $1" >&2; exit 1; }
ok() { echo "✓ $1"; }

echo "=== CI gate: gofmt + vet + lint + test + builds + govulncheck + secret scan + shellcheck + docs + static analysis ==="

echo "=== gofmt ==="
FMT_DIRTY="$(gofmt -l src tests/harness tests/debug_run)"
if [ -n "$FMT_DIRTY" ]; then
  echo "$FMT_DIRTY"
  fail "gofmt: unformatted files (run gofmt -w on the list above)"
fi
ok "gofmt clean"

echo "=== Go vet (src + tests/harness + tests/debug_run) ==="
(cd src && go vet ./...) || fail "go vet failed in src/"
(cd tests/harness && go vet ./...) || fail "go vet failed in tests/harness/"
(cd tests/debug_run && go vet ./...) || fail "go vet failed in tests/debug_run/"
ok "go vet passed"

echo "=== golangci-lint (src + tests/harness) ==="
command -v golangci-lint >/dev/null 2>&1 ||
  fail "golangci-lint not found (CI image must pin it; host: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8)"
(cd src && golangci-lint run ./...) || fail "golangci-lint failed in src/"
(cd tests/harness && golangci-lint run ./...) || fail "golangci-lint failed in tests/harness/"
ok "golangci-lint passed"

echo "=== go test (src + tests/harness) ==="
(cd src && go test ./...) || fail "go test failed in src/"
(cd tests/harness && go test ./...) || fail "go test failed in tests/harness/"
ok "go test passed"

echo "=== Builds (temp dir: $BIN) ==="
(cd src && go build -o "$BIN/feralos_installer" .) ||
  fail "build failed: feralos_installer"
(cd src/utils/createConfig/createValues && go build -o "$BIN/feralos_create_config" .) ||
  fail "build failed: feralos_create_config"
(cd src && go build -o "$BIN/feralos_secureboot" ./firstBoot/secureboot/) ||
  fail "build failed: feralos_secureboot"
(cd src && go build -o "$BIN/feralos_tpm" ./firstBoot/tpm/) ||
  fail "build failed: feralos_tpm"
(cd src && go build -o "$BIN/feralos_webmin" ./firstBoot/webmin/) ||
  fail "build failed: feralos_webmin"
(cd tests/harness && go build -o "$BIN/skea_harness" ./cmd/skea_harness/) ||
  fail "build failed: skea_harness"
(cd tests/debug_run && go build -o "$BIN/debug_run" .) ||
  fail "build failed: debug_run"
ls -la "$BIN"
ok "all 7 binaries built"

echo "=== govulncheck (warning only, mirrors pre-commit) ==="
if command -v govulncheck >/dev/null 2>&1; then
  for m in src tests/harness; do
    if ! (cd "$m" && govulncheck ./...); then
      echo "⚠ govulncheck found vulnerabilities in $m/ (warning only, not blocking)"
    else
      ok "govulncheck passed in $m/"
    fi
  done
else
  echo "⚠ govulncheck not found — skipping"
fi

echo "=== Secret scan (gitleaks, blocking) ==="
# gitleaks is the primary scanner (.gitleaks.toml: default rules + fixture/
# lockfile allowlist; known historical findings fingerprinted in
# .gitleaksignore). The pattern grep below stays as the fallback for hosts
# without gitleaks — its patterns stay in sync with .githooks/pre-commit
# (same entries, same fixture/docs allowlist for the generic-secret pattern).
# NOTE: .githooks/pre-commit carries the same list — edit BOTH or neither.
if command -v gitleaks >/dev/null 2>&1; then
  if [ -d .git ]; then
    # git checkout: full commit-history scan (docker context ships no .git)
    gitleaks detect --source . --config .gitleaks.toml --no-banner --redact ||
      fail "gitleaks: secrets found in history (see output above)"
  else
    # no .git (docker image): scan the working tree instead
    gitleaks dir . --config .gitleaks.toml --no-banner --redact ||
      fail "gitleaks: secrets found (see output above)"
  fi
  ok "gitleaks clean"
else
  echo "⚠ gitleaks not found — falling back to the pattern grep scan (host: install gitleaks)"
  PATTERNS=(
    'AKIA[0-9A-Z]{16}'                          # AWS Access Key
    '-----BEGIN[[:space:]]*(RSA|EC|OPENSSH|DSA)[[:space:]]*PRIVATE KEY-----'  # Private key
    'ghp_[0-9a-zA-Z]{36}'                       # GitHub PAT
    'gho_[0-9a-zA-Z]{36}'                       # GitHub OAuth
    'ghu_[0-9a-zA-Z]{36}'                       # GitHub user token
    'sk-[0-9a-zA-Z]{32,}'                       # OpenAI API key
    'xox[baprs]-[0-9a-zA-Z]{10,}'               # Slack token
    'PGP[[:space:]]+PRIVATE[[:space:]]+KEY'     # PGP key
    $'(password|secret|api_key|apikey)[[:space:]]*[:=][[:space:]]*["\'][^"\']{8,}["\']'  # Generic secret
  )

  # All repo files except VCS state and gitignored artifacts (the docker
  # context carries no .git; on the CI checkout these exclusions are a no-op).
  BLOCKED=0
  while IFS= read -r FILE; do
    [ -f "$FILE" ] || continue
    for PATTERN in "${PATTERNS[@]}"; do
      # Allow dummy passwords in test fixtures/harness (QEMU fixtures, not real secrets)
      if echo "$PATTERN" | grep -q "password|secret"; then
        case "$FILE" in
          tests/fixtures/*|tests/harness/variables_*.yaml|tests/harness/*.go|docs/*|src/variables/variables.yaml|.opencode/*) continue ;;
        esac
      fi
      if grep -qiE -- "$PATTERN" "$FILE"; then
        echo "⚠️  BLOCKED: $FILE matches secret pattern"
        BLOCKED=1
      fi
    done
  done < <(find . -type f ! -path './.git/*' ! -path './tests/artifacts/*' -printf '%P\n')

  if [ "$BLOCKED" -eq 1 ]; then
    fail "secret scan: remove the secrets above"
  fi
  ok "secret scan passed (grep fallback)"
fi

echo "=== ShellCheck + sh -n (project scripts, P3-3) ==="
SCRIPTS=(
  setup.sh
  tests/harness/niri_realhw_check.sh
  tests/harness/debug_target_tpm_boot.sh
  tests/harness/debug.sh
  tests/harness/run_mcp.sh
  tests/harness/generate_mcp_config.sh
  scripts/history-scrub.sh
  scripts/ci.sh
)
command -v shellcheck >/dev/null 2>&1 ||
  fail "shellcheck not found (CI image bundles it; host: see Dockerfile shell stage)"
for s in "${SCRIPTS[@]}"; do
  shellcheck "$s" || fail "shellcheck failed on $s"
  # syntax check: bash shebang -> bash -n, POSIX -> sh -n
  if head -n 1 "$s" | grep -q bash; then
    bash -n "$s" || fail "bash -n failed on $s"
  else
    sh -n "$s" || fail "sh -n failed on $s"
  fi
done
ok "shellcheck + sh -n clean on ${#SCRIPTS[@]} scripts"

echo "=== Docs check ==="
bash scripts/docs_check.sh

echo "=== yamllint (blocking — broken YAML breaks the installer) ==="
command -v yamllint >/dev/null 2>&1 ||
  fail "yamllint not found (CI image must pin it; host: pip install yamllint==1.38.0)"
# every YAML in the tree except gitignored/vendor dirs (docker context has
# no .git, so a plain find with the usual exclusions works in both worlds)
mapfile -t YAML_FILES < <(find . -type f \( -name '*.yaml' -o -name '*.yml' \) \
  ! -path './.git/*' ! -path './tests/artifacts/*' ! -path './references/*' \
  ! -path './.agents/*' ! -path './.opencode/*' ! -path './handoff/*' \
  ! -path './state/*' ! -path './.agent-memory/*' ! -path './node_modules/*' \
  | sort)
if [ "${#YAML_FILES[@]}" -eq 0 ]; then
  fail "yamllint: no YAML files found"
fi
yamllint -c .yamllint.yml "${YAML_FILES[@]}" || fail "yamllint found errors"
ok "yamllint clean (${#YAML_FILES[@]} files)"

echo "=== taplo (TOML lint, warning only — tool optional) ==="
taplo lint --config taplo.toml .gitleaks.toml typos.toml 2>/dev/null ||
  echo "⚠ taplo lint findings or taplo not installed (warning only, not blocking)"

echo "=== typos (warning only) ==="
if command -v typos >/dev/null 2>&1; then
  typos --config typos.toml --format brief . ||
    echo "⚠ typos found spelling issues (warning only, not blocking)"
else
  echo "⚠ typos not found — skipping"
fi

echo "=== markdownlint-cli2 (warning only) ==="
if command -v markdownlint-cli2 >/dev/null 2>&1; then
  markdownlint-cli2 "docs/**/*.md" "*.md" ".github/**/*.md" ||
    echo "⚠ markdownlint findings (warning only, not blocking)"
else
  echo "markdownlint not available, skipping"
fi

echo "=== editorconfig-checker (warning only) ==="
if command -v ec >/dev/null 2>&1; then
  ec --config .ecrc . ||
    echo "⚠ editorconfig-checker findings (warning only, not blocking)"
else
  echo "ec not available, skipping"
fi

echo ""
echo "=== CI gate: ALL GREEN ==="
