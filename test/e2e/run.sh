#!/usr/bin/env bash
#
# End-to-end test for provider-ctfd.
#
# It spins up a kind cluster, deploys a minimal CTFd, installs Crossplane and the
# provider (built from source and loaded into kind), applies the example
# Challenge/Page/Theme resources, waits for them to become Ready and then asserts
# — through the CTFd API — that the corresponding objects were created.
#
# Requirements: docker (running), kind, kubectl, go. helm is required unless
# INSTALL_CROSSPLANE=false.
#
# Useful overrides:
#   KEEP=1                 keep the kind cluster on exit (for debugging)
#   INSTALL_CROSSPLANE=0   skip installing Crossplane core (faster)
#   CTFD_IMAGE=ctfd/ctfd:X use a different CTFd image
#   CLUSTER=name           kind cluster name
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

CLUSTER="${CLUSTER:-provider-ctfd-e2e}"
IMG="${IMG:-provider-ctfd:e2e}"
IMG_BOOTSTRAP="${IMG_BOOTSTRAP:-ctfd-bootstrap:e2e}"
CTFD_IMAGE="${CTFD_IMAGE:-ctfd/ctfd:latest}"
INSTALL_CROSSPLANE="${INSTALL_CROSSPLANE:-1}"
KEEP="${KEEP:-0}"
PF_PORT="${PF_PORT:-8000}"
ARCH="$(go env GOARCH)"
OUT="$ROOT/_output/e2e"

PF_PID=""

log()  { printf '\033[1;34m[e2e]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[e2e] FAIL:\033[0m %s\n' "$*" >&2; exit 1; }

cleanup() {
  local code=$?
  [ -n "$PF_PID" ] && kill "$PF_PID" >/dev/null 2>&1 || true
  if [ "$KEEP" = "1" ]; then
    log "KEEP=1 set; leaving cluster '$CLUSTER' running"
    print_access
  else
    log "deleting kind cluster '$CLUSTER'"
    kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
  fi
  exit $code
}
trap cleanup EXIT

# print_access prints how to reach the live CTFd web UI and the Crossplane
# resources for manual testing (only shown when KEEP=1).
print_access() {
  printf '%s\n' \
    '' \
    '──────────────────────────────────────────────────────────────────────' \
    ' Manual testing — the environment is live.' \
    '' \
    ' 1. Open the CTFd web UI. In a separate terminal, keep this running:' \
    "      kubectl --context kind-${CLUSTER} -n ctfd port-forward svc/ctfd ${PF_PORT}:8000" \
    "    then browse to:  http://localhost:${PF_PORT}" \
    '' \
    ' 2. Log in as admin:        username: admin    password: password' \
    '' \
    ' 3. Inspect the Crossplane-managed resources:' \
    "      kubectl --context kind-${CLUSTER} -n default get challenge,page,settings" \
    "      kubectl --context kind-${CLUSTER} -n default describe challenge welcome" \
    '' \
    ' 4. Create / edit your own (the provider reconciles them into CTFd):' \
    "      kubectl --context kind-${CLUSTER} -n default edit challenge welcome" \
    "      kubectl --context kind-${CLUSTER} apply -f examples/resources/page.yaml" \
    '' \
    ' 5. Watch the provider logs:' \
    "      kubectl --context kind-${CLUSTER} -n provider-ctfd-system logs -f deploy/provider-ctfd" \
    '' \
    " 6. When done, tear it down:  kind delete cluster --name ${CLUSTER}" \
    '──────────────────────────────────────────────────────────────────────' \
    ''
}

require() { command -v "$1" >/dev/null 2>&1 || fail "missing required tool: $1"; }

require docker
require kind
require kubectl
require go
docker info >/dev/null 2>&1 || fail "docker daemon is not running"
[ "$INSTALL_CROSSPLANE" = "1" ] && require helm

mkdir -p "$OUT"

# ---------------------------------------------------------------------------
log "creating kind cluster '$CLUSTER'"
if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --wait 120s
fi
KCTL=(kubectl --context "kind-$CLUSTER")

# kapply applies a manifest file with a bounded request timeout and a few
# retries. Installing Crossplane churns aggregated discovery, which can make an
# unlucky `kubectl apply` stall; retrying with a timeout keeps the run moving.
kapply() {
  local f="$1" i
  for i in 1 2 3 4 5; do
    if "${KCTL[@]}" apply --request-timeout=90s -f "$f"; then return 0; fi
    log "kubectl apply -f $f failed (attempt $i/5); retrying in 5s…"
    sleep 5
  done
  fail "kubectl apply -f $f failed after 5 attempts"
}

# buildimg <dockerfile> <tag>: build a linux image from $OUT (which holds the
# bin/<os>_<arch>/ binaries) and load it into the kind cluster. --load is
# required so the result lands in the local docker image store (the default
# buildx builder may use the docker-container driver, keeping it in cache only).
buildimg() {
  local dockerfile="$1" tag="$2"
  cp "$dockerfile" "$OUT/Dockerfile"
  DOCKER_BUILDKIT=1 docker buildx build --platform "linux/${ARCH}" --load \
    -f "$OUT/Dockerfile" -t "$tag" "$OUT"
  kind load docker-image "$tag" --name "$CLUSTER"
}

# ---------------------------------------------------------------------------
log "building provider + bootstrap images and loading them into kind"
mkdir -p "$OUT/bin/linux_${ARCH}"
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -o "$OUT/bin/linux_${ARCH}/provider" ./cmd/provider
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -o "$OUT/bin/linux_${ARCH}/ctfdctl" ./test/e2e/ctfdctl
buildimg cluster/images/provider-ctfd/Dockerfile "$IMG"
buildimg cluster/images/ctfd-bootstrap/Dockerfile "$IMG_BOOTSTRAP"

# ---------------------------------------------------------------------------
log "applying provider CRDs"
kapply package/crds

if [ "$INSTALL_CROSSPLANE" = "1" ]; then
  log "installing Crossplane core"
  helm repo add crossplane-stable https://charts.crossplane.io/stable >/dev/null 2>&1 || true
  # Update only this repo: a global `helm repo update` fails if any unrelated
  # repo already configured in the environment is momentarily unreachable.
  helm repo update crossplane-stable >/dev/null
  helm upgrade --install crossplane crossplane-stable/crossplane \
    --kube-context "kind-$CLUSTER" \
    --namespace crossplane-system --create-namespace --wait --timeout 10m
fi

# ---------------------------------------------------------------------------
# Everything below is declarative: CTFd, the bootstrap Job (which runs the setup
# wizard and writes the credentials Secret), the (Cluster)ProviderConfig and the
# provider. No imperative setup or manual credential wiring.
log "deploying CTFd ($CTFD_IMAGE)"
sed "s#ctfd/ctfd:latest#${CTFD_IMAGE}#" test/e2e/manifests/ctfd.yaml > "$OUT/ctfd.yaml"
kapply "$OUT/ctfd.yaml"

log "deploying CTFd bootstrap Job (runs the setup wizard, writes the creds Secret)"
sed "s#BOOTSTRAP_IMAGE#${IMG_BOOTSTRAP}#" test/e2e/manifests/bootstrap.yaml > "$OUT/bootstrap.yaml"
kapply "$OUT/bootstrap.yaml"
kapply test/e2e/manifests/providerconfig.yaml

log "deploying the provider"
sed "s#PROVIDER_IMAGE#${IMG}#" test/e2e/manifests/provider.yaml > "$OUT/provider.yaml"
kapply "$OUT/provider.yaml"

log "waiting for the bootstrap Job to complete (CTFd setup + credentials Secret)"
if ! "${KCTL[@]}" -n ctfd wait --for=condition=complete --timeout=360s job/ctfd-bootstrap; then
  log "bootstrap Job did not complete; logs:"
  "${KCTL[@]}" -n ctfd logs job/ctfd-bootstrap --tail=60 || true
  fail "ctfd-bootstrap Job failed"
fi
"${KCTL[@]}" -n provider-ctfd-system rollout status deploy/provider-ctfd --timeout=180s

# ---------------------------------------------------------------------------
log "applying example resources"
kapply examples/resources/challenge.yaml
kapply examples/resources/page.yaml
kapply examples/resources/settings.yaml

log "waiting for managed resources to become Ready"
if ! "${KCTL[@]}" -n default wait --for=condition=Ready --timeout=240s \
  challenge/break-the-license challenge/welcome page/rules settings/instance; then
  log "resources did not become Ready; recent provider logs:"
  "${KCTL[@]}" -n provider-ctfd-system logs deploy/provider-ctfd --tail=80 || true
  log "managed resource status:"
  "${KCTL[@]}" -n default get challenge,page,settings -o wide || true
  fail "managed resources never reached Ready"
fi

# ---------------------------------------------------------------------------
# Verify through the CTFd API. The token was minted by the bootstrap Job and
# stored in the Secret; read it back, and reach CTFd via a port-forward.
log "verifying CTFd state through the API"
go build -o "$OUT/ctfdctl" ./test/e2e/ctfdctl
TOKEN="$("${KCTL[@]}" -n default get secret ctfd-creds -o jsonpath='{.data.credentials}' \
  | base64 --decode | python3 -c 'import sys,json; print(json.load(sys.stdin)["api_key"])')"
[ -n "$TOKEN" ] || fail "could not read api_key from the ctfd-creds Secret"

"${KCTL[@]}" -n ctfd port-forward svc/ctfd "${PF_PORT}:8000" </dev/null >/dev/null 2>&1 &
PF_PID=$!
for i in $(seq 1 30); do
  curl -fsS -o /dev/null "http://localhost:${PF_PORT}/" 2>/dev/null && break
  sleep 2
  [ "$i" = "30" ] && fail "CTFd not reachable on localhost:${PF_PORT}"
done

CTFD_URL="http://localhost:${PF_PORT}" "$OUT/ctfdctl" -mode verify -token "$TOKEN"

# ---------------------------------------------------------------------------
# Regression: a Challenge that omits `state` must be left unmanaged — the
# provider lets CTFd own the field and never reconciles it. Create such a
# challenge, flip its state out-of-band, nudge a reconcile, and assert the
# provider does NOT revert it (older builds forced unset state back to "hidden").
log "asserting an unset Challenge state is not reconciled"
kapply test/e2e/manifests/challenge-no-state.yaml
if ! "${KCTL[@]}" -n default wait --for=condition=Ready --timeout=180s challenge/no-state; then
  "${KCTL[@]}" -n provider-ctfd-system logs deploy/provider-ctfd --tail=80 || true
  "${KCTL[@]}" -n default get challenge/no-state -o wide || true
  fail "challenge/no-state never reached Ready"
fi

CTFD_BASE="http://localhost:${PF_PORT}"
log "flipping 'No State Challenge' to visible directly in CTFd (out-of-band)"
CTFD_URL="$CTFD_BASE" "$OUT/ctfdctl" -mode setstate -challenge "No State Challenge" -state visible -token "$TOKEN"

# Force an immediate reconcile (annotation change -> watch event) so we don't
# merely wait out the poll interval, then give the provider time to act.
"${KCTL[@]}" -n default annotate challenge/no-state "e2e.ctfd/nudge=$(date +%s)" --overwrite >/dev/null
sleep 20

if ! CTFD_URL="$CTFD_BASE" "$OUT/ctfdctl" -mode checkstate -challenge "No State Challenge" -state visible -token "$TOKEN"; then
  "${KCTL[@]}" -n provider-ctfd-system logs deploy/provider-ctfd --tail=80 || true
  fail "provider reverted the unmanaged state (expected it to stay 'visible')"
fi

log "✅ e2e succeeded"
