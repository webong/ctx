#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

export HOME="$TEST_ROOT/home"
export CTX_HOME="$HOME/.config/ctx"
export CTX_BIN_DIR="$TEST_ROOT/bin"
export CTX_PLATFORM=Darwin
mkdir -p "$HOME" "$CTX_BIN_DIR" "$TEST_ROOT/fake-bin" "$TEST_ROOT/project"

for tool in docker podman nerdctl container kubectl aws gcloud open shell; do
  cp "$ROOT/tests/fake-$tool" "$TEST_ROOT/fake-bin/$tool"
done
mv "$TEST_ROOT/fake-bin/shell" "$TEST_ROOT/fake-bin/custom-shell"
cp "$ROOT/tests/fake-postgres" "$TEST_ROOT/fake-bin/psql"
cp "$ROOT/tests/fake-mysql" "$TEST_ROOT/fake-bin/mysql"
chmod +x "$TEST_ROOT/fake-bin"/*
export PATH="$CTX_BIN_DIR:$TEST_ROOT/fake-bin:$PATH"

GOCACHE=${GOCACHE:-/tmp/ctx-go-build-cache} GOMODCACHE=${GOMODCACHE:-/tmp/ctx-go-mod-cache} \
  go build -o "$CTX_BIN_DIR/ctx" "$ROOT/cmd/ctx"

mkdir -p "$CTX_HOME/catalog/adapters"
for adapter in docker podman nerdctl apple firefox kube aws gcloud postgres mysql; do
  cp -R "$ROOT/adapters/$adapter" "$CTX_HOME/catalog/adapters/$adapter"
done
ctx adapter available | grep -Eq '^docker[[:space:]]+container[[:space:]]+available'
ctx setup --adapters docker,podman,nerdctl,apple,firefox,kube,aws,gcloud,postgres,mysql >/dev/null
for adapter in docker podman nerdctl apple firefox kube aws gcloud postgres mysql; do
  ctx adapter ls | grep -Eq "^${adapter}[[:space:]]+trusted"
done
for engine in docker podman nerdctl; do test -x "$CTX_BIN_DIR/$engine"; done
ctx adapter available | grep -Eq '^docker[[:space:]]+container[[:space:]]+installed'
ctx adapter remove nerdctl >/dev/null
test ! -e "$CTX_BIN_DIR/nerdctl"
ctx adapter add nerdctl >/dev/null
test -x "$CTX_BIN_DIR/nerdctl"
ctx adapter refresh >/dev/null

ctx adapter install "$ROOT/examples/adapters/echo" >/dev/null
if ctx adapter doctor echo >/dev/null 2>&1; then
  printf 'native core executed an untrusted adapter\n' >&2
  exit 1
fi
ctx adapter trust echo >/dev/null

mkdir -p "$HOME/Library/Application Support/Firefox"
printf '%s\n' '[Profile0]' 'Name=client-a' > "$HOME/Library/Application Support/Firefox/profiles.ini"

cd "$TEST_ROOT/project"
ctx set docker alpha >/dev/null
ctx set podman red >/dev/null
ctx set nerdctl k8s.io >/dev/null
ctx set apple local >/dev/null
ctx set kube production --namespace payments >/dev/null
ctx set aws client-a >/dev/null
ctx set gcloud client-a >/dev/null
ctx set browser firefox:client-a >/dev/null
ctx set postgres client-a-dev >/dev/null
ctx set mysql client-a >/dev/null
ctx set echo staging >/dev/null

test "$(ctx real docker)" = "$TEST_ROOT/fake-bin/docker"
test "$(CTX_SHELL=custom-shell ctx shell)" = 'custom shell'
test "$(ctx shell --shell custom-shell)" = 'custom shell'
ctx completion powershell | grep -Fq 'Register-ArgumentCompleter'
test "$(ctx run docker ps)" = '--context alpha ps'
test "$(docker ps)" = '--context alpha ps'
test "$(DOCKER_CONTEXT=environment ctx status docker)" = 'docker: environment (DOCKER_CONTEXT)'
test "$(ctx run podman ps)" = '--connection red ps'
test "$(podman ps)" = '--connection red ps'
test "$(ctx run nerdctl ps)" = '--namespace k8s.io ps'
test "$(nerdctl ps)" = '--namespace k8s.io ps'
test "$(ctx run container list)" = 'list'
test "$(ctx run apple list)" = 'list'
ctx clear docker >/dev/null
test "$(ctx run docker ps)" = 'ps'
test "$(docker ps)" = 'ps'
ctx set docker alpha >/dev/null
test "$(ctx ls browser)" = 'firefox:client-a'
test "$(ctx run kubectl get pods)" = '--context production --namespace payments get pods'
test "$(ctx run aws sts get-caller-identity)" = '--profile client-a sts get-caller-identity'
test "$(ctx run gcloud projects list)" = '--configuration client-a projects list'
test "$(ctx run psql app)" = 'PGSERVICE=client-a-dev app'
test "$(ctx run mysql app)" = '--login-path=client-a app'
test "$(ctx run echo hello)" = 'run[staging] hello'
test "$(ctx open https://example.test)" = '-na Firefox --args -P client-a https://example.test'
ctx adapter inspect firefox | grep -Fq 'kind:         browser'
ctx adapter inspect docker | grep -Fq 'kind:         container'
ctx adapter inspect docker | grep -Fq 'default:      true'
ctx adapter ls container | grep -Eq '^apple[[:space:]]+trusted'
ctx adapter ls container | grep -Eq '^docker[[:space:]]+trusted'
ctx ls container | grep -Fq 'docker:alpha'
ctx ls container | grep -Fq 'apple:local'
mv "$TEST_ROOT/fake-bin/container" "$TEST_ROOT/fake-bin/container-disabled"
ctx ls container | grep -Fq 'docker:alpha'
mv "$TEST_ROOT/fake-bin/container-disabled" "$TEST_ROOT/fake-bin/container"
ctx doctor | grep -Fq 'ok   adapter kube production'
ctx doctor | grep -Fq 'ok   container docker alpha'
ctx doctor | grep -Fq 'ok   browser firefox:client-a'

test "$(ctx build --cache-ref registry.example/app:buildcache -- --tag registry.example/app:dev .)" = '--context alpha buildx build --cache-from type=registry,ref=registry.example/app:buildcache --cache-to type=registry,ref=registry.example/app:buildcache,mode=max --tag registry.example/app:dev .'
ctx clear docker >/dev/null
test "$(ctx build --cache-ref registry.example/app:buildcache -- --tag registry.example/app:dev .)" = 'buildx build --cache-from type=registry,ref=registry.example/app:buildcache --cache-to type=registry,ref=registry.example/app:buildcache,mode=max --tag registry.example/app:dev .'
ctx set docker alpha >/dev/null
test "$(ctx build podman --cache-ref registry.example/app:podman-cache -- --tag registry.example/app:dev .)" = '--connection red build --layers --cache-from registry.example/app:podman-cache --cache-to registry.example/app:podman-cache --tag registry.example/app:dev .'
test "$(ctx build nerdctl --cache-ref registry.example/app:nerdctl-cache -- --tag registry.example/app:dev .)" = '--namespace k8s.io build --cache-from type=registry,ref=registry.example/app:nerdctl-cache --cache-to type=registry,ref=registry.example/app:nerdctl-cache,mode=max --tag registry.example/app:dev .'
test "$(ctx image sync alpha beta registry.example/app:dev)" = "$(printf '%s\n%s' '--context alpha image push registry.example/app:dev' '--context beta image pull registry.example/app:dev')"
ctx image sync --tar alpha beta registry.example/app:dev >/dev/null
image_copy_output=$(ctx image copy docker:alpha podman:red registry.example/app:dev)
printf '%s\n' "$image_copy_output" | grep -Eq '^--context alpha image save -o .+ registry.example/app:dev$'
printf '%s\n' "$image_copy_output" | grep -Eq '^--connection red image load -i .+$'
image_copy_output=$(ctx image copy podman:red docker:beta registry.example/app:dev)
printf '%s\n' "$image_copy_output" | grep -Eq '^--connection red image save -o .+ registry.example/app:dev$'
printf '%s\n' "$image_copy_output" | grep -Eq '^--context beta image load -i .+$'
image_copy_output=$(ctx image copy docker:alpha nerdctl:k8s.io registry.example/app:dev)
printf '%s\n' "$image_copy_output" | grep -Eq '^--context alpha image save -o .+ registry.example/app:dev$'
printf '%s\n' "$image_copy_output" | grep -Eq '^--namespace k8s.io load -i .+$'
image_copy_output=$(ctx image copy nerdctl:k8s.io apple:local registry.example/app:dev)
printf '%s\n' "$image_copy_output" | grep -Eq '^--namespace k8s.io save -o .+ registry.example/app:dev$'
printf '%s\n' "$image_copy_output" | grep -Eq '^image load --input .+$'
if CTX_TEST_APPLE_VERSION=1.2.0 ctx image copy docker:alpha apple:local registry.example/app:dev >/dev/null 2>&1; then
  printf 'native core allowed an unsafe Apple Container image import\n' >&2
  exit 1
fi
test "$(ctx volume export alpha data 2>/dev/null)" = '--context alpha run --rm -v data:/volume:ro alpine:3.21 tar -C /volume -cf - .'
test "$(ctx volume import alpha restored </dev/null)" = "$(printf '%s\n%s' '--context alpha volume create restored' '--context alpha run --rm -i -v restored:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy docker:alpha podman:red data restored-podman 2>/dev/null)" = "$(printf '%s\n%s' '--connection red volume create restored-podman' '--connection red run --rm -i -v restored-podman:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy podman:red docker:beta data restored-docker 2>/dev/null)" = "$(printf '%s\n%s' '--context beta volume create restored-docker' '--context beta run --rm -i -v restored-docker:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy docker:alpha nerdctl:k8s.io data restored-nerdctl 2>/dev/null)" = "$(printf '%s\n%s' '--namespace k8s.io volume create restored-nerdctl' '--namespace k8s.io run --rm -i -v restored-nerdctl:/volume alpine:3.21 tar -C /volume -xf -')"
test "$(ctx volume copy nerdctl:k8s.io apple:local data restored-apple 2>/dev/null)" = "$(printf '%s\n%s' 'volume create restored-apple' 'run --rm -i -v restored-apple:/volume alpine:3.21 tar -C /volume -xf -')"
if ctx volume import alpha exists </dev/null >/dev/null 2>&1; then
  printf 'native core imported into an existing volume\n' >&2
  exit 1
fi
if ctx volume copy docker:alpha podman:red data exists </dev/null >/dev/null 2>&1; then
  printf 'native core copied into an existing volume\n' >&2
  exit 1
fi

ctx profile set client-b aws_profile default >/dev/null
ctx profile set client-b shell_path /client/bin >/dev/null
ctx profile env client-b APP_ENV development >/dev/null
ctx profile env client-b CTX_SHELL custom-shell >/dev/null
ctx profile show client-b | grep -Fq 'aws_profile = "default"'
ctx profile show client-b | grep -Fq '[env]'
ctx profile use client-b >/dev/null
ctx clear aws >/dev/null
test "$(ctx shell)" = 'custom shell'
test "$(ctx run aws sts get-caller-identity)" = '--profile default sts get-caller-identity'
test "$(ctx run docker print-env)" = 'development'
test "$(ctx run -- sh -c 'printf %s "$APP_ENV"')" = 'development'
case "$(ctx env)" in *'PATH=/client/bin:'*) ;; *) exit 1 ;; esac
ctx profile env-unset client-b APP_ENV >/dev/null
ctx profile unset client-b shell_path >/dev/null
ctx profile clear >/dev/null

printf 'ctx native adapter tests passed\n'
