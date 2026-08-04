#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

readonly DEFAULT_BRANCH="token3-restricted-admin"
readonly DEFAULT_FORK_REPO="CarminBack/sub2api"
readonly DEFAULT_OFFICIAL_REPO="Wei-Shaw/sub2api"

DRY_RUN=0
ARG_TARGET=""
ARG_CURRENT=""
while (($#)); do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --target) ARG_TARGET="${2:-}"; shift ;;
    --current) ARG_CURRENT="${2:-}"; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

DEPLOY_DIR="${TOKEN3_DEPLOY_DIR:-/opt/sub2api2}"
COMPOSE_FILE="${TOKEN3_COMPOSE_FILE:-docker-compose.yml}"
APP_SERVICE="${TOKEN3_APP_SERVICE:-sub2api}"
POSTGRES_SERVICE="${TOKEN3_POSTGRES_SERVICE:-postgres}"
REDIS_SERVICE="${TOKEN3_REDIS_SERVICE:-redis}"
APP_CONTAINER="${TOKEN3_APP_CONTAINER:-sub2api2}"
STATE_DIR="${TOKEN3_STATE_DIR:-${DEPLOY_DIR}/data/token3-update}"
UPDATER_DIR="${TOKEN3_UPDATER_DIR:-/opt/token3-updater}"
REPO_DIR="${TOKEN3_REPO_DIR:-${UPDATER_DIR}/repo}"
BACKUP_ROOT="${TOKEN3_BACKUP_ROOT:-${DEPLOY_DIR}/backups/token3-updater}"
DEPLOY_KEY="${TOKEN3_DEPLOY_KEY:-/root/.ssh/token3_updater_ed25519}"
BRANCH="${TOKEN3_BRANCH:-$DEFAULT_BRANCH}"
FORK_REPO="${TOKEN3_FORK_REPO:-$DEFAULT_FORK_REPO}"
OFFICIAL_REPO="${TOKEN3_OFFICIAL_REPO:-$DEFAULT_OFFICIAL_REPO}"
WORKFLOW_TIMEOUT_SECONDS="${TOKEN3_WORKFLOW_TIMEOUT_SECONDS:-2700}"
HEALTH_TIMEOUT_SECONDS="${TOKEN3_HEALTH_TIMEOUT_SECONDS:-300}"

REQUEST_FILE="${STATE_DIR}/request.json"
PROCESSING_FILE="${STATE_DIR}/request.processing.json"
STATUS_FILE="${STATE_DIR}/status.json"
LOCK_FILE="${UPDATER_DIR}/update.lock"
STARTED_AT=""
CURRENT_VERSION="${ARG_CURRENT}"
TARGET_VERSION="${ARG_TARGET}"
NEW_VERSION=""
STAGE="initialization"
DEPLOY_STARTED=0
COMPOSE_BACKUP=""

log() {
  printf '[token3-updater] %s\n' "$*"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

validate_versions() {
  [[ "$TARGET_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
    echo "target version must match X.Y.Z" >&2
    exit 2
  }
  [[ "$CURRENT_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-token3\.[0-9]+$ ]] || {
    echo "current version must match X.Y.Z-token3.N" >&2
    exit 2
  }
}

current_official_version() {
  printf '%s\n' "${CURRENT_VERSION%%-token3.*}"
}

target_is_newer() {
  local current current_parts target_parts index
  current="$(current_official_version)"
  IFS=. read -r -a current_parts <<<"$current"
  IFS=. read -r -a target_parts <<<"$TARGET_VERSION"
  for index in 0 1 2; do
    if ((10#${target_parts[$index]} > 10#${current_parts[$index]})); then
      return 0
    fi
    if ((10#${target_parts[$index]} < 10#${current_parts[$index]})); then
      return 1
    fi
  done
  return 1
}

write_status() {
  local state="$1" message="$2" finished_at="${3:-}" now tmp
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  tmp="$(mktemp "${STATE_DIR}/.status.XXXXXX")"
  jq -n \
    --arg state "$state" \
    --arg message "$message" \
    --arg current "$CURRENT_VERSION" \
    --arg target "$TARGET_VERSION" \
    --arg started "$STARTED_AT" \
    --arg updated "$now" \
    --arg finished "$finished_at" \
    '{state:$state,message:$message,current_version:$current,target_version:$target,updated_at:$updated}
      + (if $started != "" then {started_at:$started} else {} end)
      + (if $finished != "" then {finished_at:$finished} else {} end)' >"$tmp"
  chmod 0644 "$tmp"
  mv -f "$tmp" "$STATUS_FILE"
}

rollback_application() {
  if ((DEPLOY_STARTED == 0)) || [[ -z "$COMPOSE_BACKUP" || ! -f "$COMPOSE_BACKUP" ]]; then
    return
  fi
  log "restoring the previous application image"
  cp -p "$COMPOSE_BACKUP" "${DEPLOY_DIR}/${COMPOSE_FILE}"
  (cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate "$APP_SERVICE") || true
}

finish_failed() {
  local message="$1"
  trap - ERR
  set +e
  rollback_application
  write_status "failed" "$message" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  rm -f "$PROCESSING_FILE"
  log "$message"
  exit 1
}

on_error() {
  local line="$1"
  finish_failed "Update failed during ${STAGE} (line ${line}); review token3-updater.service logs"
}

trap 'on_error "$LINENO"' ERR

if ((DRY_RUN)); then
  validate_versions
  if target_is_newer; then
    log "dry run: ${CURRENT_VERSION} can update from official v${TARGET_VERSION}"
  else
    log "dry run: ${CURRENT_VERSION} already includes official v${TARGET_VERSION} or newer"
  fi
  exit 0
fi

[[ "$(id -u)" == "0" ]] || {
  echo "token3-updater must run as root" >&2
  exit 1
}

for command_name in curl docker flock git jq sha256sum ssh; do
  require_command "$command_name"
done

mkdir -p "$STATE_DIR" "$UPDATER_DIR" "$BACKUP_ROOT"
chmod 0755 "$STATE_DIR"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  log "another update is already running"
  exit 0
fi

if [[ -z "$TARGET_VERSION" ]]; then
  [[ -f "$REQUEST_FILE" ]] || {
    log "no update request found"
    exit 0
  }
  mv "$REQUEST_FILE" "$PROCESSING_FILE"
  CURRENT_VERSION="$(jq -er '.current_version' "$PROCESSING_FILE")"
  TARGET_VERSION="$(jq -er '.target_version' "$PROCESSING_FILE")"
fi

validate_versions
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
write_status "queued" "Update request accepted"

if ! target_is_newer; then
  write_status "succeeded" "Current Token3 build already includes this official version" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  rm -f "$PROCESSING_FILE"
  exit 0
fi

[[ -r "$DEPLOY_KEY" ]] || finish_failed "Dedicated GitHub deploy key is missing"
GIT_SSH_COMMAND="ssh -i ${DEPLOY_KEY} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
export GIT_SSH_COMMAND

STAGE="official source synchronization"
write_status "syncing" "Fetching and merging the official stable tag"
if [[ ! -d "${REPO_DIR}/.git" ]]; then
  git clone "git@github.com:${FORK_REPO}.git" "$REPO_DIR"
fi
git -C "$REPO_DIR" remote set-url origin "git@github.com:${FORK_REPO}.git"
if git -C "$REPO_DIR" remote get-url upstream >/dev/null 2>&1; then
  git -C "$REPO_DIR" remote set-url upstream "https://github.com/${OFFICIAL_REPO}.git"
else
  git -C "$REPO_DIR" remote add upstream "https://github.com/${OFFICIAL_REPO}.git"
fi
git -C "$REPO_DIR" fetch --prune origin "$BRANCH"
git -C "$REPO_DIR" fetch --force upstream "refs/tags/v${TARGET_VERSION}:refs/tags/v${TARGET_VERSION}"
git -C "$REPO_DIR" checkout -B "$BRANCH" "origin/$BRANCH"
git -C "$REPO_DIR" reset --hard "origin/$BRANCH"
git -C "$REPO_DIR" clean -fd
git -C "$REPO_DIR" config user.name "Token3 Updater"
git -C "$REPO_DIR" config user.email "token3-updater@localhost"
if ! git -C "$REPO_DIR" merge --no-ff --no-edit "v${TARGET_VERSION}"; then
  git -C "$REPO_DIR" merge --abort || true
  finish_failed "Official v${TARGET_VERSION} has merge conflicts; no code was pushed or deployed"
fi

NEW_VERSION="${TARGET_VERSION}-token3.1"
printf '%s\n' "$NEW_VERSION" >"${REPO_DIR}/TOKEN3_VERSION"
git -C "$REPO_DIR" add TOKEN3_VERSION
if ! git -C "$REPO_DIR" diff --cached --quiet; then
  git -C "$REPO_DIR" commit -m "chore: set Token3 version ${NEW_VERSION}"
fi
COMMIT_SHA="$(git -C "$REPO_DIR" rev-parse HEAD)"
git -C "$REPO_DIR" push origin "HEAD:${BRANCH}"

STAGE="GitHub workflows"
write_status "building" "Waiting for CI, security scan, and image build"
workflow_deadline=$((SECONDS + WORKFLOW_TIMEOUT_SECONDS))
required_workflows=("CI" "Security Scan" "Token3 Image")
all_succeeded=0
while ((SECONDS < workflow_deadline)); do
  runs_json="$(curl -fsSL --retry 3 --retry-delay 2 \
    "https://api.github.com/repos/${FORK_REPO}/actions/runs?head_sha=${COMMIT_SHA}&per_page=100")"
  all_succeeded=1
  for workflow_name in "${required_workflows[@]}"; do
    run_state="$(jq -r --arg name "$workflow_name" '
      [.workflow_runs[] | select(.name == $name)] | sort_by(.created_at) | last
      | if . == null then "missing" else (.status + ":" + (.conclusion // "")) end
    ' <<<"$runs_json")"
    case "$run_state" in
      completed:success) ;;
      completed:*) finish_failed "GitHub workflow ${workflow_name} did not succeed" ;;
      *) all_succeeded=0 ;;
    esac
  done
  ((all_succeeded == 1)) && break
  # Unauthenticated public API calls are limited to 60/hour per source IP.
  sleep 60
done
((all_succeeded == 1)) || finish_failed "Timed out waiting for GitHub workflows"

STAGE="immutable image resolution"
IMAGE_TAG="ghcr.io/carminback/sub2api:sha-${COMMIT_SHA}"
docker pull "$IMAGE_TAG"
IMAGE_REF="$(docker image inspect "$IMAGE_TAG" --format '{{range .RepoDigests}}{{println .}}{{end}}' | awk '/^ghcr.io\/carminback\/sub2api@sha256:/{print; exit}')"
[[ "$IMAGE_REF" =~ ^ghcr\.io/carminback/sub2api@sha256:[0-9a-f]{64}$ ]] || finish_failed "Could not resolve an immutable image digest"

STAGE="production backup"
write_status "deploying" "Backing up production before replacing the application"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="${BACKUP_ROOT}/${timestamp}-${COMMIT_SHA:0:12}"
mkdir -p "$backup_dir"
chmod 0700 "$backup_dir"
COMPOSE_BACKUP="${backup_dir}/${COMPOSE_FILE}"
cp -p "${DEPLOY_DIR}/${COMPOSE_FILE}" "$COMPOSE_BACKUP"
for config_file in .env data/config.yaml; do
  if [[ -f "${DEPLOY_DIR}/${config_file}" ]]; then
    mkdir -p "${backup_dir}/$(dirname "$config_file")"
    cp -p "${DEPLOY_DIR}/${config_file}" "${backup_dir}/${config_file}"
  fi
done
(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" exec -T "$POSTGRES_SERVICE" \
  sh -ec 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc') >"${backup_dir}/postgres.dump"
sha256sum "${backup_dir}/postgres.dump" >"${backup_dir}/postgres.dump.sha256"
(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" exec -T "$POSTGRES_SERVICE" \
  pg_restore --list) <"${backup_dir}/postgres.dump" >/dev/null

postgres_id_before="$(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE")"
redis_id_before="$(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" ps -q "$REDIS_SERVICE")"
postgres_restarts_before="$(docker inspect -f '{{.RestartCount}}' "$postgres_id_before")"
redis_restarts_before="$(docker inspect -f '{{.RestartCount}}' "$redis_id_before")"

STAGE="application deployment"
compose_tmp="$(mktemp "${DEPLOY_DIR}/.compose.XXXXXX")"
awk -v image="$IMAGE_REF" -v service="$APP_SERVICE" '
  $0 == "  " service ":" { in_service=1 }
  in_service && /^  [^ ]/ && $0 != "  " service ":" { in_service=0 }
  in_service && /^    image:/ {
    if (replaced) exit 42
    print "    image: " image
    replaced=1
    next
  }
  { print }
  END { if (!replaced) exit 43 }
' "${DEPLOY_DIR}/${COMPOSE_FILE}" >"$compose_tmp"
chmod --reference="${DEPLOY_DIR}/${COMPOSE_FILE}" "$compose_tmp"
(cd "$DEPLOY_DIR" && docker compose -f "$compose_tmp" config -q)
mv -f "$compose_tmp" "${DEPLOY_DIR}/${COMPOSE_FILE}"
DEPLOY_STARTED=1
(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate "$APP_SERVICE")

health_deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
while ((SECONDS < health_deadline)); do
  app_id="$(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" ps -q "$APP_SERVICE")"
  health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$app_id" 2>/dev/null || true)"
  [[ "$health" == "healthy" ]] && break
  sleep 5
done
[[ "${health:-}" == "healthy" ]] || finish_failed "New application container did not become healthy"

version_output="$(docker exec "$APP_CONTAINER" /app/sub2api -version 2>&1)"
[[ "$version_output" == *"$NEW_VERSION"* ]] || finish_failed "New application reports an unexpected version"
curl -fsS --retry 5 --retry-delay 2 "http://127.0.0.1:8811/health" >/dev/null

postgres_id_after="$(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" ps -q "$POSTGRES_SERVICE")"
redis_id_after="$(cd "$DEPLOY_DIR" && docker compose -f "$COMPOSE_FILE" ps -q "$REDIS_SERVICE")"
[[ "$postgres_id_after" == "$postgres_id_before" ]] || finish_failed "PostgreSQL container was unexpectedly replaced"
[[ "$redis_id_after" == "$redis_id_before" ]] || finish_failed "Redis container was unexpectedly replaced"
[[ "$(docker inspect -f '{{.RestartCount}}' "$postgres_id_after")" == "$postgres_restarts_before" ]] || finish_failed "PostgreSQL restarted during application deployment"
[[ "$(docker inspect -f '{{.RestartCount}}' "$redis_id_after")" == "$redis_restarts_before" ]] || finish_failed "Redis restarted during application deployment"

CURRENT_VERSION="$NEW_VERSION"
write_status "succeeded" "Official v${TARGET_VERSION} was merged and deployed successfully" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
rm -f "$PROCESSING_FILE"
log "deployed ${NEW_VERSION} from ${COMMIT_SHA}"
