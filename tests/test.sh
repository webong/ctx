#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

export HOME="$TEST_ROOT/home"
export CTX_HOME="$HOME/.config/ctx"
export CTX_BIN_DIR="$HOME/.local/bin"
mkdir -p "$TEST_ROOT/fake-bin" "$TEST_ROOT/project/child" "$TEST_ROOT/mapped" "$HOME"
cp "$ROOT/tests/fake-docker" "$TEST_ROOT/fake-bin/docker"
cp "$ROOT/tests/fake-podman" "$TEST_ROOT/fake-bin/podman"
cp "$ROOT/tests/fake-nerdctl" "$TEST_ROOT/fake-bin/nerdctl"
cp "$ROOT/tests/fake-container" "$TEST_ROOT/fake-bin/container"
chmod +x "$TEST_ROOT/fake-bin/docker" "$TEST_ROOT/fake-bin/podman" "$TEST_ROOT/fake-bin/nerdctl" "$TEST_ROOT/fake-bin/container"
export PATH="$CTX_BIN_DIR:$TEST_ROOT/fake-bin:$PATH"

"$ROOT/install.sh" >/dev/null
"$ROOT/install.sh" >/dev/null
test "$(ctx version)" = 'ctx 0.5.0'
test -x "$CTX_BIN_DIR/ctx"
test -x "$CTX_BIN_DIR/docker"
test -x "$CTX_BIN_DIR/podman"
test -x "$CTX_BIN_DIR/nerdctl"
test -f "$CTX_HOME/config.toml"

(
  cd "$TEST_ROOT/mapped"
  CTX_BIN_DIR="$TEST_ROOT/stream-bin" CTX_HOME="$TEST_ROOT/stream-config" CTX_SOURCE_BASE="file://$ROOT" sh < "$ROOT/install.sh" >/dev/null
  test -x "$TEST_ROOT/stream-bin/ctx"
  test -x "$TEST_ROOT/stream-bin/docker"
  test -x "$TEST_ROOT/stream-bin/podman"
  test -x "$TEST_ROOT/stream-bin/nerdctl"
  test -f "$TEST_ROOT/stream-config/config.toml"
)

git init -q "$TEST_ROOT/project"
cd "$TEST_ROOT/project"
ctx set docker alpha >/dev/null
ctx set podman red >/dev/null
ctx set nerdctl k8s.io >/dev/null
grep -Fxq 'docker = "alpha"' .ctx
grep -Fxq 'podman = "red"' .ctx
grep -Fxq 'nerdctl = "k8s.io"' .ctx
git check-ignore -q .ctx

cd child
test "$(docker ps)" = '--context alpha ps'
test "$(podman ps)" = '--connection red ps'
test "$(docker build -f Containerfile .)" = '--context alpha build -f Containerfile .'
test "$(podman build -f Containerfile .)" = '--connection red build -f Containerfile .'
test "$(nerdctl ps)" = '--namespace k8s.io ps'
test "$(nerdctl build -f Containerfile .)" = '--namespace k8s.io build -f Containerfile .'
test "$(DOCKER_CONTEXT=beta docker ps)" = ps
test "$(CONTAINER_CONNECTION=blue podman ps)" = ps
test "$(CONTAINERD_NAMESPACE=default nerdctl ps)" = ps
test "$(docker --context beta ps)" = '--context beta ps'
test "$(podman --connection blue ps)" = '--connection blue ps'
test "$(podman system connection list)" = "$(printf 'red\nblue')"
test "$(nerdctl namespace ls)" = "$(printf 'default\nk8s.io')"
test "$(ctx status docker)" = "docker: alpha ($TEST_ROOT/project/.ctx)"
test "$(ctx build --cache-ref registry.example/app:buildcache -- --tag registry.example/app:dev .)" = '--context alpha buildx build --cache-from type=registry,ref=registry.example/app:buildcache --cache-to type=registry,ref=registry.example/app:buildcache,mode=max --tag registry.example/app:dev .'
test "$(ctx build podman --cache-ref registry.example/app:podman-cache -- --tag registry.example/app:dev .)" = '--connection red build --layers --cache-from registry.example/app:podman-cache --cache-to registry.example/app:podman-cache --tag registry.example/app:dev .'
test "$(ctx build nerdctl --cache-ref registry.example/app:nerdctl-cache -- --tag registry.example/app:dev .)" = '--namespace k8s.io build --cache-from type=registry,ref=registry.example/app:nerdctl-cache --cache-to type=registry,ref=registry.example/app:nerdctl-cache,mode=max --tag registry.example/app:dev .'
test "$(ctx image sync alpha beta registry.example/app:dev)" = "$(printf '%s\n%s' '--context alpha image push registry.example/app:dev' '--context beta image pull registry.example/app:dev')"
ctx image sync --tar alpha beta registry.example/app:dev >/dev/null
image_copy_output=$(ctx image copy docker:alpha podman:red registry.example/app:dev)
case "$image_copy_output" in *'--connection red image load -i '*) ;; *) exit 1 ;; esac
image_copy_output=$(ctx image copy podman:red docker:beta registry.example/app:dev)
case "$image_copy_output" in *'--context beta image load -i '*) ;; *) exit 1 ;; esac
image_copy_output=$(ctx image copy docker:alpha nerdctl:k8s.io registry.example/app:dev)
case "$image_copy_output" in *'--namespace k8s.io load -i '*) ;; *) exit 1 ;; esac
image_copy_output=$(ctx image copy nerdctl:k8s.io apple:local registry.example/app:dev)
case "$image_copy_output" in *'image load --input '*) ;; *) exit 1 ;; esac
if CTX_TEST_APPLE_VERSION=1.2.0 ctx image copy docker:alpha apple:local registry.example/app:dev >/dev/null 2>&1; then
  printf 'unsafe Apple Container image loader was accepted\n' >&2
  exit 1
fi
test "$(ctx volume export alpha data 2>/dev/null)" = '--context alpha run --rm -v data:/volume:ro alpine:3.21 tar -C /volume -cf - .'
test "$(ctx volume import alpha restored </dev/null)" = "$(printf '%s\n%s' '--context alpha volume create restored' '--context alpha run --rm -i -v restored:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy docker:alpha podman:red data restored-podman 2>/dev/null)" = "$(printf '%s\n%s' '--connection red volume create restored-podman' '--connection red run --rm -i -v restored-podman:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy podman:red docker:beta data restored-docker 2>/dev/null)" = "$(printf '%s\n%s' '--context beta volume create restored-docker' '--context beta run --rm -i -v restored-docker:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy docker:alpha nerdctl:k8s.io data restored-nerdctl 2>/dev/null)" = "$(printf '%s\n%s' '--namespace k8s.io volume create restored-nerdctl' '--namespace k8s.io run --rm -i -v restored-nerdctl:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy nerdctl:k8s.io apple:local data restored-apple 2>/dev/null)" = "$(printf '%s\n%s' 'volume create restored-apple' 'run --rm -i -v restored-apple:/volume alpine:3.21 tar -C /volume -xf -')"
if ctx volume import alpha exists </dev/null >/dev/null 2>&1; then
  printf 'existing volume was accepted for import\n' >&2
  exit 1
fi
if ctx volume copy docker:alpha podman:red data exists </dev/null >/dev/null 2>&1; then
  printf 'existing cross-engine volume was accepted for import\n' >&2
  exit 1
fi

cd "$TEST_ROOT/project"
ctx clear docker >/dev/null
test "$(podman ps)" = '--connection red ps'
test "$(docker ps)" = ps
ctx clear >/dev/null
test ! -e .ctx

printf '\n[projects."%s"]\ndocker = "beta"\npodman = "blue"\n' "$TEST_ROOT/mapped" >> "$CTX_HOME/config.toml"
cd "$TEST_ROOT/mapped"
test "$(docker ps)" = '--context beta ps'
test "$(podman ps)" = '--connection blue ps'

cd "$HOME"
test "$(docker ps)" = ps
test "$(podman ps)" = ps
test "$(nerdctl ps)" = ps
ctx set docker alpha --global >/dev/null
ctx set podman red --global >/dev/null
ctx set nerdctl default --global >/dev/null
test "$(docker ps)" = '--context alpha ps'
test "$(podman ps)" = '--connection red ps'
test "$(nerdctl ps)" = '--namespace default ps'
if ctx set docker missing >/dev/null 2>&1; then
  printf 'invalid Docker context was accepted\n' >&2
  exit 1
fi
if ctx set podman missing >/dev/null 2>&1; then
  printf 'invalid Podman connection was accepted\n' >&2
  exit 1
fi
if ctx set nerdctl missing >/dev/null 2>&1; then
  printf 'invalid nerdctl namespace was accepted\n' >&2
  exit 1
fi

printf 'ctx tests passed\n'
