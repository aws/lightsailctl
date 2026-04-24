#!/usr/bin/env bash
# lightsailctl integration test
# Prerequisites: docker (or compatible), goreleaser (make tools), AWS credentials
# Run from the repo root
set -euo pipefail

# --- Colors & symbols ---
if [[ -t 1 ]] && command -v tput >/dev/null 2>&1 && [[ $(tput colors 2>/dev/null || echo 0) -ge 8 ]]; then
  BOLD=$(tput bold); RESET=$(tput sgr0)
  RED=$(tput setaf 1); GREEN=$(tput setaf 2); YELLOW=$(tput setaf 3); BLUE=$(tput setaf 4); CYAN=$(tput setaf 6)
else
  BOLD=""; RESET=""; RED=""; GREEN=""; YELLOW=""; BLUE=""; CYAN=""
fi
CHECK="✅"; CROSS="❌"; INFO="ℹ️ "; ARROW="→"

step()   { echo; echo "${BOLD}${BLUE}=== $* ===${RESET}"; }
run()    { echo "${CYAN}${ARROW} \$ $*${RESET}"; "$@"; }
check()  { echo "${YELLOW}${INFO}${RESET} $*"; }
pass()   { echo "${GREEN}${CHECK} $*${RESET}"; }
fail()   { echo "${RED}${CROSS} $*${RESET}" >&2; }
die()    { fail "$*"; exit 1; }

# --- Config (defaults) ---
REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"
SERVICE_NAME="lightsailctl-test-$$"
IMAGE=hello-world:latest
SKIP_CLEANUP=false

usage() {
  cat <<EOF
${BOLD}lightsailctl integration test${RESET}

${BOLD}USAGE${RESET}
  $(basename "$0") [options]

${BOLD}OPTIONS${RESET}
  -r, --region REGION       AWS region
                            (default: \$AWS_REGION, else \$AWS_DEFAULT_REGION, else us-east-1)
  -s, --service-name NAME   Lightsail container service name
                            (default: lightsailctl-test-<pid>)
  -i, --image IMAGE         Local container image to push
                            (default: $IMAGE)
      --skip-cleanup        Don't delete the container service on exit
  -h, --help                Show this help and exit

${BOLD}PREREQUISITES${RESET}
  - aws CLI v2 with valid credentials
  - docker (real Docker, not Finch/Podman)
  - goreleaser (run: make tools)

${BOLD}EXAMPLES${RESET}
  $(basename "$0")
  $(basename "$0") --region us-west-2 --image myapp:latest
  $(basename "$0") --service-name my-existing-svc --skip-cleanup
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -r|--region)       REGION="$2"; shift 2 ;;
    -s|--service-name) SERVICE_NAME="$2"; shift 2 ;;
    -i|--image)        IMAGE="$2"; shift 2 ;;
    --skip-cleanup)    SKIP_CLEANUP=true; shift ;;
    -h|--help)         usage; exit 0 ;;
    *) fail "Unknown option: $1"; echo; usage; exit 2 ;;
  esac
done

# --- Final result banner ---
RESULT="FAIL"
EXIT_CODE=0
finish() {
  if [[ "$RESULT" == "PASS" ]]; then
    echo
    echo "${BOLD}${GREEN}============================================${RESET}"
    echo "${BOLD}${GREEN}  ${CHECK} ALL INTEGRATION TESTS PASSED${RESET}"
    echo "${BOLD}${GREEN}============================================${RESET}"
  else
    echo
    echo "${BOLD}${RED}============================================${RESET}"
    echo "${BOLD}${RED}  ${CROSS} INTEGRATION TESTS FAILED (exit=$EXIT_CODE)${RESET}"
    echo "${BOLD}${RED}============================================${RESET}"
  fi
}

cleanup() {
  EXIT_CODE=$?
  step "Cleanup"
  if $SKIP_CLEANUP; then
    check "--skip-cleanup set; leaving service $SERVICE_NAME in place."
    finish
    return
  fi
  # Only attempt deletion if we actually got past service creation.
  if [[ "${SERVICE_CREATED:-false}" == "true" ]]; then
    if aws lightsail delete-container-service \
        --region "$REGION" \
        --service-name "$SERVICE_NAME" 2>/dev/null; then
      pass "Container service deletion initiated: $SERVICE_NAME"
    else
      fail "Failed to delete service $SERVICE_NAME; please delete it manually."
    fi
  else
    check "No service to clean up (not created yet)."
  fi
  finish
}
trap cleanup EXIT

# --- Preflight ---
step "Preflight checks"

preflight_ok=true

check_cmd() {
  local cmd="$1" hint="$2"
  if command -v "$cmd" >/dev/null 2>&1; then
    pass "$cmd found: $(command -v "$cmd")"
  else
    fail "$cmd not found. $hint"
    preflight_ok=false
  fi
}

check_cmd aws        "Install the AWS CLI: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
check_cmd goreleaser "Run: make tools   (installs goreleaser + golangci-lint)"
check_cmd make       "Install GNU make (part of build-essential / Xcode Command Line Tools)."

if command -v aws >/dev/null 2>&1; then
  if aws sts get-caller-identity >/dev/null 2>&1; then
    IDENTITY=$(aws sts get-caller-identity --query 'Arn' --output text)
    pass "AWS credentials OK: $IDENTITY"
  else
    fail "AWS credentials not configured or invalid. Run: aws configure   (or set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN)"
    preflight_ok=false
  fi
fi

check_cmd docker     "Install Docker Desktop: https://docs.docker.com/desktop/"

if command -v docker >/dev/null 2>&1; then
  if docker info >/dev/null 2>&1; then
    INFO_OUTPUT=$(docker info 2>/dev/null || true)
    VERSION_OUTPUT=$(docker version 2>/dev/null || true)
    COMBINED="$INFO_OUTPUT"$'\n'"$VERSION_OUTPUT"
    if echo "$COMBINED" | grep -qi 'Docker Engine'; then
      API_VERSION=$(docker version --format '{{.Server.APIVersion}}' 2>/dev/null || echo unknown)
      pass "Docker daemon reachable (API version: $API_VERSION)"
    elif echo "$COMBINED" | grep -qiE 'finch|nerdctl'; then
      fail "'docker' on PATH is Finch/nerdctl, not Docker. lightsailctl requires real Docker."
      fail "  Install Docker Desktop (https://docs.docker.com/desktop/) and ensure its 'docker' is first on PATH."
      preflight_ok=false
    elif echo "$COMBINED" | grep -qi 'podman'; then
      fail "'docker' on PATH is Podman, not Docker. lightsailctl requires real Docker."
      fail "  Install Docker Desktop (https://docs.docker.com/desktop/) and ensure its 'docker' is first on PATH."
      preflight_ok=false
    else
      fail "'docker' on PATH does not appear to be real Docker."
      fail "  Install Docker Desktop (https://docs.docker.com/desktop/) and ensure its 'docker' is first on PATH."
      preflight_ok=false
    fi
  else
    fail "Docker daemon not reachable. Start Docker Desktop."
    preflight_ok=false
  fi
fi

if ! $preflight_ok; then
  die "Preflight failed. Fix the issues above and re-run."
fi
pass "All preflight checks passed"

# --- Detect goreleaser output directory for current platform ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        GORELEASER_ARCH="amd64_v1" ;;
  aarch64|arm64) GORELEASER_ARCH="arm64_v8.0" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac
DIST_DIR="dist/lightsailctl_${OS}_${GORELEASER_ARCH}"
check "Target region: $REGION"
check "Service name:  $SERVICE_NAME"
check "Test image:    $IMAGE"
check "Binary path:   $DIST_DIR/lightsailctl"

# --- 1. Build ---
step "1. Build via goreleaser snapshot"
check "Compiles lightsailctl for all platforms and puts binary in dist/"
run env GOPROXY=direct make snapshot
export PATH="$PWD/$DIST_DIR:$PATH"
check "Verifying the built binary is on PATH"
run lightsailctl --version
pass "Build complete"

# --- 2. Pull image ---
step "2. Pull test image"
check "Need a local linux/amd64 image to push to Lightsail"
run docker pull "$IMAGE"
pass "Image pulled"

# --- 3. Create service ---
step "3. Create Lightsail container service"
check "Creates a nano/scale-1 service (required target for push-container-image)"
run aws lightsail create-container-service \
  --region "$REGION" \
  --service-name "$SERVICE_NAME" \
  --power nano \
  --scale 1
SERVICE_CREATED=true

check "Polling until service reaches READY (this takes a few minutes)"
while true; do
  STATE=$(aws lightsail get-container-services \
    --region "$REGION" \
    --service-name "$SERVICE_NAME" \
    --query 'containerServices[0].state' --output text)
  echo "  state: ${BOLD}$STATE${RESET}"
  [[ "$STATE" == "READY" ]] && break
  sleep 15
done
pass "Service is READY"

# --- 4. AWS CLI plugin path ---
step "4. Push image via AWS CLI (plugin path)"
check "Validates that AWS CLI invokes our lightsailctl binary on PATH"
run aws lightsail push-container-image \
  --region "$REGION" \
  --service-name "$SERVICE_NAME" \
  --image "$IMAGE" \
  --label test-cli
pass "AWS CLI plugin path works"

# --- 5. Direct plugin invocation ---
step "5. Push image via direct plugin invocation"
check "Validates the --plugin --input-stdin contract that AWS CLI relies on"
PAYLOAD='{
  "inputVersion": "1",
  "operation": "PushContainerImage",
  "payload": {
    "service": "'"$SERVICE_NAME"'",
    "label":   "test-direct",
    "image":   "'"$IMAGE"'"
  },
  "configuration": {
    "region": "'"$REGION"'"
  }
}'
echo "${CYAN}${ARROW} \$ echo '<payload>' | lightsailctl --plugin --input-stdin${RESET}"
echo "$PAYLOAD" | lightsailctl --plugin --input-stdin
pass "Direct plugin invocation works"

# --- 6. Verify ---
step "6. Verify images were registered"
check "Expects two registered images (test-cli + test-direct)"
run aws lightsail get-container-images \
  --region "$REGION" \
  --service-name "$SERVICE_NAME" \
  --query 'containerImages[].image' --output table

REGISTERED=$(aws lightsail get-container-images \
  --region "$REGION" \
  --service-name "$SERVICE_NAME" \
  --query 'length(containerImages)' --output text)
if [[ "$REGISTERED" -ge 2 ]]; then
  pass "$REGISTERED images registered"
else
  die "Expected at least 2 registered images, got $REGISTERED"
fi

RESULT="PASS"
