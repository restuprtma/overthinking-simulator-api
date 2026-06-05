#!/usr/bin/env bash
# =============================================================================
# running-order-consumer — build, validate, deploy automation
# =============================================================================
# Flow:
#   1. Validate prereqs (kubectl, git, kubeconfig, namespace reachability)
#   2. Ensure clean tree on production branch
#   3. Push to origin/production (Docker Hub Autobuild webhook fires here)
#   4. Poll registry until <image>:<tag> is pullable from inside the cluster
#   5. Bump tag in deployment.yaml
#   6. kubectl apply + rollout status
#   7. Tail post-deploy logs and verify dual-table routing is active
#
# Prerequisites:
#   - Docker Hub Automated Build configured for venturoid/tuai-running-order-consumer-production
#     with rules:  Branch=production → Tag=<sourceref|literal>
#                  Dockerfile=deployments/docker/running-order-consumer.Dockerfile
#                  Build Context=/
#   - dockerhub-secret-venturoid Secret exists in namespace tuai (image pull)
#   - running-order-consumer-env Secret exists in namespace tuai with required env vars
#
# Usage:
#   ./deploy.sh <tag>                # full flow: push, wait, deploy
#   ./deploy.sh <tag> --no-build     # skip git + wait (rollback to existing tag)
#   ./deploy.sh <tag> --no-push      # skip git push (image already pushed)
#   ./deploy.sh --help
# =============================================================================

set -euo pipefail

IMAGE="venturoid/tuai-running-order-consumer-production"
NAMESPACE="tuai"
DEPLOYMENT="running-order-consumer"
BRANCH="production"
PULL_SECRET="dockerhub-secret-venturoid"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
DEPLOY_FILE="$SCRIPT_DIR/deployment.yaml"

# Default kubeconfig is the committed prod one — caller can override.
KUBECONFIG="${KUBECONFIG:-$REPO_ROOT/deployments/kubernetes/production.yaml}"
export KUBECONFIG

# Args
NEW_TAG=""
SKIP_BUILD=false
NO_PUSH=false
TIMEOUT_SEC=600

usage() {
  cat <<'EOF'
running-order-consumer — build, validate, deploy automation

Usage:
  ./deploy.sh <tag>                Full flow: push, wait for autobuild, deploy
  ./deploy.sh <tag> --no-build     Skip git push & autobuild wait (rollback)
  ./deploy.sh <tag> --no-push      Skip git push only (image already pushed)
  ./deploy.sh <tag> --timeout 900  Custom wait timeout in seconds (default 600)
  ./deploy.sh -h | --help

Flow:
  1. Validate prereqs (kubectl, git, kubeconfig, namespace, pull secret)
  2. Ensure clean tree on production branch
  3. Push to origin/production (Docker Hub Autobuild webhook fires here)
  4. Poll registry until <image>:<tag> is pullable from inside the cluster
  5. Bump tag in deployment.yaml
  6. kubectl apply + rollout status
  7. Tail post-deploy logs and verify dual-table routing is active

Prereqs:
  - Docker Hub Automated Build configured for venturoid/tuai-running-order-consumer-production
    Branch=production → Tag=<your-tag>
    Dockerfile=deployments/docker/running-order-consumer.Dockerfile
    Build Context=/
  - Secret dockerhub-secret-venturoid in namespace tuai (image pull)
  - Secret running-order-consumer-env in namespace tuai (runtime env vars)

Env overrides:
  KUBECONFIG  (default: deployments/kubernetes/production.yaml)
EOF
}

# Parse args
while [ $# -gt 0 ]; do
  case "$1" in
    --no-build) SKIP_BUILD=true; NO_PUSH=true; shift;;
    --no-push)  NO_PUSH=true; shift;;
    --timeout)  TIMEOUT_SEC="${2:-}"; shift 2;;
    -h|--help)  usage; exit 0;;
    -*)         echo "unknown option: $1" >&2; usage >&2; exit 1;;
    *)          [ -z "$NEW_TAG" ] && NEW_TAG="$1" || { echo "unexpected arg: $1" >&2; exit 1; }; shift;;
  esac
done

[ -z "$NEW_TAG" ] && { echo "error: <tag> is required" >&2; usage >&2; exit 1; }

# Pretty output
B="\033[1m"; G="\033[32m"; Y="\033[33m"; R="\033[31m"; D="\033[2m"; N="\033[0m"
say()  { printf "${B}>> %s${N}\n" "$*"; }
ok()   { printf "${G}  ✓${N} %s\n" "$*"; }
warn() { printf "${Y}  ⚠${N} %s\n" "$*"; }
err()  { printf "${R}  ✗${N} %s\n" "$*" >&2; }
dim()  { printf "${D}     %s${N}\n" "$*"; }

# Portable in-place sed (BSD on macOS, GNU on Linux).
sed_inplace() {
  if sed --version >/dev/null 2>&1; then
    sed -i "$@"
  else
    sed -i '' "$@"
  fi
}

# ---------------------------------------------------------------------------
# Step 1 — prereqs
# ---------------------------------------------------------------------------
say "Step 1/7: validate prereqs"
command -v kubectl >/dev/null || { err "kubectl not found in PATH"; exit 1; }
command -v git     >/dev/null || { err "git not found in PATH";     exit 1; }
[ -f "$KUBECONFIG" ]   || { err "kubeconfig not found: $KUBECONFIG"; exit 1; }
[ -f "$DEPLOY_FILE" ]  || { err "deployment.yaml not found: $DEPLOY_FILE"; exit 1; }
kubectl get ns "$NAMESPACE" >/dev/null 2>&1 || { err "namespace $NAMESPACE not reachable via kubectl"; exit 1; }
kubectl get secret "$PULL_SECRET" -n "$NAMESPACE" >/dev/null 2>&1 \
  || { err "imagePullSecret $PULL_SECRET missing in namespace $NAMESPACE"; exit 1; }
ok  "kubectl, git, kubeconfig, namespace, pull secret"
dim "image=$IMAGE  tag=$NEW_TAG  ns=$NAMESPACE  deploy=$DEPLOYMENT"

# ---------------------------------------------------------------------------
# Step 2 — git state
# ---------------------------------------------------------------------------
if ! $SKIP_BUILD; then
  say "Step 2/7: validate git state on $BRANCH"
  cd "$REPO_ROOT"
  CURRENT_BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo "DETACHED")
  if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
    err "current branch is '$CURRENT_BRANCH', expected '$BRANCH' — checkout $BRANCH first"
    exit 1
  fi
  if ! git diff --quiet || ! git diff --cached --quiet; then
    err "working tree is dirty — commit or stash before deploy"
    git status --short >&2
    exit 1
  fi
  ok "branch=$BRANCH, working tree clean"
else
  say "Step 2/7: SKIPPED (--no-build)"
fi

# ---------------------------------------------------------------------------
# Step 3 — push (autobuild trigger)
# ---------------------------------------------------------------------------
if ! $NO_PUSH; then
  say "Step 3/7: push to origin/$BRANCH (Docker Hub Autobuild fires on webhook)"
  git fetch --quiet origin "$BRANCH"
  UNPUSHED=$(git rev-list --count "origin/$BRANCH..HEAD")
  if [ "$UNPUSHED" -gt 0 ]; then
    if git push origin "$BRANCH"; then
      ok "pushed $UNPUSHED commit(s) → autobuild webhook triggered"
    else
      err "git push failed — push manually then re-run with --no-push"
      exit 1
    fi
  else
    warn "no unpushed commits — autobuild won't retrigger; image $NEW_TAG must already exist or be in flight"
  fi
else
  say "Step 3/7: SKIPPED (--no-push)"
fi

# ---------------------------------------------------------------------------
# Step 4 — wait for image
# ---------------------------------------------------------------------------
if ! $SKIP_BUILD; then
  say "Step 4/7: wait for $IMAGE:$NEW_TAG (in-cluster pull probe, timeout ${TIMEOUT_SEC}s)"
  TIMEOUT_AT=$(($(date +%s) + TIMEOUT_SEC))
  ATTEMPT=0
  IMAGE_READY=false
  while [ "$(date +%s)" -lt "$TIMEOUT_AT" ]; do
    ATTEMPT=$((ATTEMPT + 1))
    POD="rh-probe-$$-$ATTEMPT"
    printf "${D}  [%s] attempt %d... ${N}" "$(date +%H:%M:%S)" "$ATTEMPT"

    kubectl run "$POD" -n "$NAMESPACE" \
      --image="$IMAGE:$NEW_TAG" \
      --image-pull-policy=Always \
      --restart=Never \
      --overrides="{\"spec\":{\"imagePullSecrets\":[{\"name\":\"$PULL_SECRET\"}]}}" \
      >/dev/null 2>&1 || true

    sleep 22

    STATE=$(kubectl get pod "$POD" -n "$NAMESPACE" \
      -o jsonpath='phase={.status.phase} waiting={.status.containerStatuses[0].state.waiting.reason} terminated={.status.containerStatuses[0].state.terminated.reason}' \
      2>/dev/null || echo "phase=unknown")
    kubectl delete pod "$POD" -n "$NAMESPACE" --grace-period=0 --force --wait=false >/dev/null 2>&1 || true

    case "$STATE" in
      *terminated=Completed*|*terminated=Error*|*terminated=ContainerCannotRun*|*phase=Running*|*phase=Succeeded*|*phase=Failed*)
        printf "ready ${G}[%s]${N}\n" "$STATE"
        IMAGE_READY=true
        break
        ;;
      *waiting=ImagePullBackOff*|*waiting=ErrImagePull*|*phase=Pending*)
        printf "not yet ${Y}[%s]${N}\n" "$STATE"
        sleep 13
        ;;
      *)
        printf "unknown ${Y}[%s]${N}\n" "$STATE"
        sleep 13
        ;;
    esac
  done
  if ! $IMAGE_READY; then
    err "image $NEW_TAG not pullable after ${TIMEOUT_SEC}s — check Docker Hub build status"
    exit 1
  fi
  ok "$IMAGE:$NEW_TAG is pullable"
else
  say "Step 4/7: SKIPPED (--no-build)"
fi

# ---------------------------------------------------------------------------
# Step 5 — bump tag in deployment.yaml
# ---------------------------------------------------------------------------
say "Step 5/7: bump tag in deployment.yaml"
OLD_TAG=$(grep -E "^[[:space:]]*image:.*$IMAGE:" "$DEPLOY_FILE" | head -1 \
  | sed -E "s|.*$IMAGE:([^[:space:]]+).*|\1|")
if [ -z "$OLD_TAG" ]; then
  err "could not parse current image tag from $DEPLOY_FILE"
  exit 1
fi
if [ "$OLD_TAG" = "$NEW_TAG" ]; then
  warn "tag already $NEW_TAG in deployment.yaml — applying anyway to force restart"
else
  sed_inplace "s|$IMAGE:[^[:space:]]*|$IMAGE:$NEW_TAG|" "$DEPLOY_FILE"
  ok "$OLD_TAG → $NEW_TAG"
fi

# ---------------------------------------------------------------------------
# Step 6 — apply + rollout
# ---------------------------------------------------------------------------
say "Step 6/7: kubectl apply + rollout"
kubectl apply -f "$DEPLOY_FILE"
if ! kubectl rollout status -n "$NAMESPACE" "deploy/$DEPLOYMENT" --timeout=3m; then
  err "rollout did not complete within 3 minutes — check pod events:"
  kubectl describe pod -n "$NAMESPACE" -l "app.kubernetes.io/name=$DEPLOYMENT" \
    | tail -40 >&2
  exit 1
fi
ok "rollout complete"

# ---------------------------------------------------------------------------
# Step 7 — verify
# ---------------------------------------------------------------------------
say "Step 7/7: verify post-deploy"
sleep 3
POD_NAME=$(kubectl get pod -n "$NAMESPACE" -l "app.kubernetes.io/name=$DEPLOYMENT" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD_NAME" ]; then
  err "no pod found for $DEPLOYMENT"
  exit 1
fi
echo "  pod: $POD_NAME"

# Show last log lines
echo
echo "  --- recent log ---"
kubectl logs -n "$NAMESPACE" "$POD_NAME" --tail=15 2>/dev/null || warn "could not fetch logs yet"

# Verify single-table sink visible in startup log
STARTUP=$(kubectl logs -n "$NAMESPACE" "$POD_NAME" 2>/dev/null \
  | grep -m1 'running-order-consumer starting' || echo "")
echo
if echo "$STARTUP" | grep -q "questdb_table"; then
  ok "questdb sink configured ($(echo "$STARTUP" | grep -oE 'questdb_table=[^ ]+' || true))"
else
  warn "could not parse startup log; check manually:  kubectl logs -n $NAMESPACE $POD_NAME"
fi

echo
ok "deploy done — image=$IMAGE:$NEW_TAG  pod=$POD_NAME"
