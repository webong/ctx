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

for tool in docker podman nerdctl kubectl aws gcloud open; do
  cp "$ROOT/tests/fake-$tool" "$TEST_ROOT/fake-bin/$tool"
done
cp "$ROOT/tests/fake-postgres" "$TEST_ROOT/fake-bin/psql"
cp "$ROOT/tests/fake-mysql" "$TEST_ROOT/fake-bin/mysql"
chmod +x "$TEST_ROOT/fake-bin"/*
export PATH="$CTX_BIN_DIR:$TEST_ROOT/fake-bin:$PATH"

GOCACHE=${GOCACHE:-/tmp/ctx-go-build-cache} GOMODCACHE=${GOMODCACHE:-/tmp/ctx-go-mod-cache} \
  go build -o "$CTX_BIN_DIR/ctx" "$ROOT/cmd/ctx"

for adapter in firefox kube aws gcloud postgres mysql; do
  ctx adapter install "$ROOT/adapters/$adapter" >/dev/null
  ctx adapter trust "$adapter" >/dev/null
  ctx adapter ls | grep -Eq "^${adapter}[[:space:]]+trusted"
done

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
ctx set kube production --namespace payments >/dev/null
ctx set aws client-a >/dev/null
ctx set gcloud client-a >/dev/null
ctx set browser firefox:client-a >/dev/null
ctx set postgres client-a-dev >/dev/null
ctx set mysql client-a >/dev/null
ctx set echo staging >/dev/null

test "$(ctx run docker ps)" = '--context alpha ps'
test "$(ctx run podman ps)" = '--connection red ps'
test "$(ctx run nerdctl ps)" = '--namespace k8s.io ps'
test "$(ctx ls browser)" = 'firefox:client-a'
test "$(ctx run kubectl get pods)" = '--context production --namespace payments get pods'
test "$(ctx run aws sts get-caller-identity)" = '--profile client-a sts get-caller-identity'
test "$(ctx run gcloud projects list)" = '--configuration client-a projects list'
test "$(ctx run psql app)" = 'PGSERVICE=client-a-dev app'
test "$(ctx run mysql app)" = '--login-path=client-a app'
test "$(ctx run echo hello)" = 'run[staging] hello'
test "$(ctx open https://example.test)" = '-na Firefox --args -P client-a https://example.test'
ctx adapter inspect firefox | grep -Fq 'kind:         browser'
ctx doctor | grep -Fq 'ok   adapter kube production'
ctx doctor | grep -Fq 'ok   browser firefox:client-a'

ctx profile set client-b aws_profile default >/dev/null
ctx profile set client-b shell_path /client/bin >/dev/null
ctx profile env client-b APP_ENV development >/dev/null
ctx profile show client-b | grep -Fq 'aws_profile = "default"'
ctx profile show client-b | grep -Fq '[env]'
ctx profile use client-b >/dev/null
ctx clear aws >/dev/null
test "$(ctx run aws sts get-caller-identity)" = '--profile default sts get-caller-identity'
test "$(ctx run -- sh -c 'printf %s "$APP_ENV"')" = 'development'
case "$(ctx env)" in *'PATH=/client/bin:'*) ;; *) exit 1 ;; esac
ctx profile env-unset client-b APP_ENV >/dev/null
ctx profile unset client-b shell_path >/dev/null
ctx profile clear >/dev/null

printf 'ctx native adapter tests passed\n'
