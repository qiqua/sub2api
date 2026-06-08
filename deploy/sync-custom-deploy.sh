#!/usr/bin/env bash
set -Eeuo pipefail

# Sync and deploy a custom Sub2API fork without touching PostgreSQL/Redis.
#
# Defaults target the qiqua/sub2api custom/main branch. Override with env vars:
#   SUB2API_REPO_URL=https://github.com/qiqua/sub2api.git
#   SUB2API_BRANCH=custom/main
#   SUB2API_SRC_DIR=/opt/sub2api-src
#   # Default: pull a prebuilt image from GHCR. Use build only on larger servers.
#   SUB2API_DEPLOY_MODE=pull
#   SUB2API_REMOTE_IMAGE=ghcr.io/qiqua/sub2api:custom-main
#   # Optional local build mode:
#   # SUB2API_DEPLOY_MODE=build
#   SUB2API_IMAGE_REPO=sub2api-custom
#   SUB2API_DEPLOY_DIR=/path/to/current/deploy
#   SUB2API_COMPOSE_FILES=/path/to/docker-compose.yml,/path/to/docker-compose.local.yml
#   SUB2API_GOPROXY=https://proxy.golang.org,direct
#   SUB2API_GOSUMDB=sum.golang.org
#   SUB2API_DOCKER_BUILD_ARGS='--network host --build-arg HTTPS_PROXY=http://host:port'
#   SUB2API_SKIP_BACKUP=1

REPO_URL="${SUB2API_REPO_URL:-https://github.com/qiqua/sub2api.git}"
BRANCH="${SUB2API_BRANCH:-custom/main}"
SRC_DIR="${SUB2API_SRC_DIR:-/opt/sub2api-src}"
DEPLOY_MODE="${SUB2API_DEPLOY_MODE:-pull}"
REMOTE_IMAGE="${SUB2API_REMOTE_IMAGE:-ghcr.io/qiqua/sub2api:custom-main}"
IMAGE_REPO="${SUB2API_IMAGE_REPO:-sub2api-custom}"
APP_CONTAINER="${SUB2API_APP_CONTAINER:-sub2api}"
POSTGRES_CONTAINER="${SUB2API_POSTGRES_CONTAINER:-sub2api-postgres}"
GOPROXY_OVERRIDE="${SUB2API_GOPROXY:-}"
GOSUMDB_OVERRIDE="${SUB2API_GOSUMDB:-}"
DOCKER_BUILD_ARGS="${SUB2API_DOCKER_BUILD_ARGS:-}"
SKIP_BACKUP="${SUB2API_SKIP_BACKUP:-0}"

log() {
  printf '[sub2api-custom-deploy] %s\n' "$*"
}

die() {
  printf '[sub2api-custom-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    die "Docker Compose is not installed. Install docker compose plugin or docker-compose."
  fi
}

inspect_label() {
  local container="$1"
  local label="$2"
  docker inspect "$container" --format "{{ index .Config.Labels \"$label\" }}" 2>/dev/null || true
}

script_dir() {
  cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd
}

normalize_branch_tag() {
  printf '%s' "$1" | tr '/[:upper:]' '-[:lower:]' | tr -cd 'a-z0-9_.-'
}

require_cmd git
require_cmd docker

log "Repository: $REPO_URL"
log "Branch: $BRANCH"
log "Source dir: $SRC_DIR"
log "Deploy mode: $DEPLOY_MODE"

if [[ ! -d "$SRC_DIR/.git" ]]; then
  if [[ -e "$SRC_DIR" ]]; then
    die "$SRC_DIR exists but is not a git repository. Set SUB2API_SRC_DIR to another path."
  fi

  log "Cloning custom repository..."
  git clone --branch "$BRANCH" "$REPO_URL" "$SRC_DIR"
else
  log "Updating existing source repository..."
  git -C "$SRC_DIR" remote set-url origin "$REPO_URL"

  if [[ -n "$(git -C "$SRC_DIR" status --porcelain)" ]]; then
    die "$SRC_DIR has local changes. Commit/stash them first; this script will not overwrite local edits."
  fi

  git -C "$SRC_DIR" fetch origin "$BRANCH"

  if git -C "$SRC_DIR" rev-parse --verify --quiet "$BRANCH" >/dev/null; then
    git -C "$SRC_DIR" checkout "$BRANCH"
  else
    git -C "$SRC_DIR" checkout -b "$BRANCH" "origin/$BRANCH"
  fi

  git -C "$SRC_DIR" pull --ff-only origin "$BRANCH"
fi

COMMIT="$(git -C "$SRC_DIR" rev-parse --short=12 HEAD)"
BRANCH_TAG="$(normalize_branch_tag "$BRANCH")"
IMAGE_TAG="${SUB2API_IMAGE_TAG:-${BRANCH_TAG}-${COMMIT}}"

case "$DEPLOY_MODE" in
  pull)
    IMAGE_NAME="$REMOTE_IMAGE"
    log "Pulling image: $IMAGE_NAME"
    docker pull "$IMAGE_NAME"
    ;;
  build)
    IMAGE_NAME="${IMAGE_REPO}:${IMAGE_TAG}"
    build_args=()
    if [[ -n "$GOPROXY_OVERRIDE" ]]; then
      build_args+=("--build-arg" "GOPROXY=$GOPROXY_OVERRIDE")
    fi
    if [[ -n "$GOSUMDB_OVERRIDE" ]]; then
      build_args+=("--build-arg" "GOSUMDB=$GOSUMDB_OVERRIDE")
    fi
    if [[ -n "$DOCKER_BUILD_ARGS" ]]; then
      read -r -a extra_build_args <<< "$DOCKER_BUILD_ARGS"
      build_args+=("${extra_build_args[@]}")
    fi

    log "Building image: $IMAGE_NAME"
    if [[ ${#build_args[@]} -gt 0 ]]; then
      log "Docker build extra args: ${build_args[*]}"
    fi
    docker build "${build_args[@]}" -t "$IMAGE_NAME" -t "${IMAGE_REPO}:latest" "$SRC_DIR"
    ;;
  *)
    die "Unsupported SUB2API_DEPLOY_MODE: $DEPLOY_MODE. Use pull or build."
    ;;
esac

if [[ -z "$IMAGE_NAME" ]]; then
  die "Image name is empty."
fi

detected_deploy_dir="$(inspect_label "$APP_CONTAINER" "com.docker.compose.project.working_dir")"
DEPLOY_DIR="${SUB2API_DEPLOY_DIR:-${detected_deploy_dir:-$(script_dir)}}"
[[ -d "$DEPLOY_DIR" ]] || die "Deploy dir does not exist: $DEPLOY_DIR"

detected_project="$(inspect_label "$APP_CONTAINER" "com.docker.compose.project")"
PROJECT_NAME="${SUB2API_COMPOSE_PROJECT_NAME:-$detected_project}"

detected_files="$(inspect_label "$APP_CONTAINER" "com.docker.compose.project.config_files")"
config_files="${SUB2API_COMPOSE_FILES:-$detected_files}"
if [[ -z "$config_files" ]]; then
  config_files="$DEPLOY_DIR/docker-compose.yml"
fi

IFS=',' read -r -a raw_compose_files <<< "$config_files"
compose_args=()
for file in "${raw_compose_files[@]}"; do
  [[ -n "$file" ]] || continue

  if [[ "$(basename "$file")" == "docker-compose.custom-image.yml" ]]; then
    continue
  fi

  if [[ "$file" = /* ]]; then
    compose_file="$file"
  else
    compose_file="$DEPLOY_DIR/$file"
  fi

  [[ -f "$compose_file" ]] || die "Compose file not found: $compose_file"
  compose_args+=("-f" "$compose_file")
done

override_file="$DEPLOY_DIR/docker-compose.custom-image.yml"
cat > "$override_file" <<EOF
services:
  sub2api:
    image: $IMAGE_NAME
EOF

compose_args+=("-f" "$override_file")

log "Deploy dir: $DEPLOY_DIR"
if [[ -n "$PROJECT_NAME" ]]; then
  log "Compose project: $PROJECT_NAME"
fi
log "Custom compose override: $override_file"

if [[ "$SKIP_BACKUP" != "1" ]]; then
  if docker inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
    backup_dir="$DEPLOY_DIR/backups"
    backup_file="$backup_dir/sub2api_$(date +%Y%m%d_%H%M%S).sql"
    mkdir -p "$backup_dir"
    log "Backing up PostgreSQL to: $backup_file"
    docker exec "$POSTGRES_CONTAINER" sh -lc 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' > "$backup_file"
  else
    log "PostgreSQL container $POSTGRES_CONTAINER not found; skipping backup."
  fi
else
  log "Backup skipped because SUB2API_SKIP_BACKUP=1."
fi

compose_base=("--project-directory" "$DEPLOY_DIR")
if [[ -n "$PROJECT_NAME" ]]; then
  compose_base+=("-p" "$PROJECT_NAME")
fi

log "Recreating only the application container..."
compose "${compose_base[@]}" "${compose_args[@]}" up -d --no-deps --force-recreate sub2api

log "Waiting for application health..."
deadline=$((SECONDS + 90))
while (( SECONDS < deadline )); do
  state="$(docker inspect "$APP_CONTAINER" --format '{{.State.Status}}' 2>/dev/null || true)"
  health="$(docker inspect "$APP_CONTAINER" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' 2>/dev/null || true)"

  if [[ "$health" == "healthy" ]]; then
    log "Deployment complete: $APP_CONTAINER is healthy."
    exit 0
  fi

  if [[ -z "$health" && "$state" == "running" ]]; then
    log "Deployment complete: $APP_CONTAINER is running."
    exit 0
  fi

  sleep 3
done

log "Application did not become healthy in time. Recent logs:"
docker logs --tail 120 "$APP_CONTAINER" >&2 || true
die "Deployment finished with an unhealthy or unknown application state."
