#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

export HOME="$TEST_ROOT/home"
export CTX_HOME="$HOME/.config/ctx"
export CTX_BIN_DIR="$HOME/.local/bin"
mkdir -p "$TEST_ROOT/fake-bin" "$TEST_ROOT/project/child" "$TEST_ROOT/mapped" "$TEST_ROOT/bundled" "$HOME"
cp "$ROOT/tests/fake-docker" "$TEST_ROOT/fake-bin/docker"
cp "$ROOT/tests/fake-podman" "$TEST_ROOT/fake-bin/podman"
cp "$ROOT/tests/fake-nerdctl" "$TEST_ROOT/fake-bin/nerdctl"
cp "$ROOT/tests/fake-container" "$TEST_ROOT/fake-bin/container"
cp "$ROOT/tests/fake-kubectl" "$TEST_ROOT/fake-bin/kubectl"
cp "$ROOT/tests/fake-aws" "$TEST_ROOT/fake-bin/aws"
cp "$ROOT/tests/fake-gcloud" "$TEST_ROOT/fake-bin/gcloud"
cp "$ROOT/tests/fake-open" "$TEST_ROOT/fake-bin/open"
cp "$ROOT/tests/fake-postgres" "$TEST_ROOT/fake-bin/psql"
cp "$ROOT/tests/fake-postgres" "$TEST_ROOT/fake-bin/pg_dump"
cp "$ROOT/tests/fake-mysql" "$TEST_ROOT/fake-bin/mysql"
cp "$ROOT/tests/fake-mysql" "$TEST_ROOT/fake-bin/mysqldump"
chmod +x "$TEST_ROOT/fake-bin/docker" "$TEST_ROOT/fake-bin/podman" "$TEST_ROOT/fake-bin/nerdctl" "$TEST_ROOT/fake-bin/container" "$TEST_ROOT/fake-bin/kubectl" "$TEST_ROOT/fake-bin/aws" "$TEST_ROOT/fake-bin/gcloud" "$TEST_ROOT/fake-bin/open" "$TEST_ROOT/fake-bin/psql" "$TEST_ROOT/fake-bin/pg_dump" "$TEST_ROOT/fake-bin/mysql" "$TEST_ROOT/fake-bin/mysqldump"
export PATH="$CTX_BIN_DIR:$TEST_ROOT/fake-bin:$PATH"
export CTX_PLATFORM=Darwin

"$ROOT/install.sh" >/dev/null
"$ROOT/install.sh" >/dev/null
test "$(ctx version)" = 'ctx 0.6.0'
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

cat >> "$CTX_HOME/config.toml" <<'TOML'

[profiles."client-a"]
docker = "beta"
podman = "blue"
nerdctl = "k8s.io"
kube_context = "client-a-dev"
kube_namespace = "payments"
aws_profile = "client-a"
gcloud_configuration = "client-a"
browser = "firefox:client-a"
postgres_service = "client-a-dev"
mysql_login_path = "client-a"
TOML

cd "$TEST_ROOT/bundled"
test "$(ctx profile ls)" = 'client-a'
ctx profile set client-b aws_profile default >/dev/null
ctx profile set client-b kube_context production >/dev/null
test "$(ctx profile show client-b)" = "$(printf '%s\n%s' 'aws_profile = "default"' 'kube_context = "production"')"
ctx profile unset client-b kube_context >/dev/null
test "$(ctx profile show client-b)" = 'aws_profile = "default"'
ctx profile env client-a CTX_TEST_REGION eu-west-1 >/dev/null
ctx profile set client-a shell_path /profile/bin >/dev/null
ctx profile use client-a >/dev/null
grep -Fxq 'profile = "client-a"' .ctx
test "$(docker ps)" = '--context beta ps'
test "$(podman ps)" = '--connection blue ps'
test "$(nerdctl ps)" = '--namespace k8s.io ps'
ctx set docker alpha >/dev/null
test "$(docker ps)" = '--context alpha ps'
ctx clear docker >/dev/null
test "$(docker ps)" = '--context beta ps'
test "$(ctx run kubectl get pods)" = '--context client-a-dev --namespace payments get pods'
test "$(ctx run kubectl --context production -n operations get pods)" = '--context production -n operations get pods'
test "$(ctx run aws sts get-caller-identity)" = '--profile client-a sts get-caller-identity'
test "$(AWS_PROFILE=default ctx run aws sts get-caller-identity)" = 'sts get-caller-identity'
test "$(ctx run gcloud projects list)" = '--configuration client-a projects list'
test "$(CLOUDSDK_ACTIVE_CONFIG_NAME=default ctx run gcloud projects list)" = 'projects list'
test "$(ctx open http://localhost:3000)" = '-na Firefox --args -P client-a http://localhost:3000'
test "$(ctx run psql -c 'select 1')" = 'PGSERVICE=client-a-dev -c select 1'
test "$(PGSERVICE=override ctx run pg_dump app)" = 'PGSERVICE=override app'
test "$(ctx run mysql -e 'select 1')" = '--login-path=client-a -e select 1'
test "$(ctx run mysql --login-path=override -e 'select 1')" = '--login-path=override -e select 1'
test "$(ctx run -- sh -c 'printf %s "$CTX_TEST_REGION"')" = 'eu-west-1'
test "$(CTX_TEST_REGION=override ctx run -- sh -c 'printf %s "$CTX_TEST_REGION"')" = 'override'
test "$(ctx shell -- sh -c 'printf %s "$CTX_TEST_REGION"')" = 'eu-west-1'
case "$(ctx run -- sh -c 'printf %s "$PATH"')" in /profile/bin:*) ;; *) exit 1 ;; esac
ctx env | grep -Fq 'CTX_TEST_REGION=eu-west-1'
ctx env | grep -Fq 'PATH=/profile/bin:'
CTX_TEST_REGION=override ctx env | grep -Fq 'CTX_TEST_REGION=override'
ctx profile show client-a | grep -Fq '[env]'
test "$(ctx run docker ps)" = '--context beta ps'
ctx explain | grep -Fq 'kube-context     client-a-dev'
ctx explain | grep -Fq 'aws-profile      client-a'
ctx doctor | grep -Fq 'ok   kube client-a-dev'
ctx doctor | grep -Fq 'ok   aws client-a'
ctx doctor | grep -Fq 'ok   gcloud client-a'
ctx doctor | grep -Fq 'ok   browser firefox:client-a'
ctx doctor | grep -Fq 'ok   postgres service client-a-dev'
ctx doctor | grep -Fq 'ok   mysql login path client-a'
ctx profile clear >/dev/null
test ! -e .ctx

ctx set kube production --namespace operations >/dev/null
ctx set aws default >/dev/null
ctx set gcloud default >/dev/null
ctx set browser chrome:'Profile 1' >/dev/null
ctx set postgres local-dev >/dev/null
ctx set mysql local-dev >/dev/null
test "$(ctx run kubectl get pods)" = '--context production --namespace operations get pods'
test "$(ctx run aws s3 ls)" = '--profile default s3 ls'
test "$(ctx run gcloud projects list)" = '--configuration default projects list'
test "$(ctx open https://example.test)" = '-na Google Chrome --args --profile-directory=Profile 1 https://example.test'
test "$(ctx run psql app)" = 'PGSERVICE=local-dev app'
test "$(ctx run mysqldump app)" = '--login-path=local-dev app'
ctx clear kube >/dev/null
ctx clear aws >/dev/null
ctx clear gcloud >/dev/null
ctx clear browser >/dev/null
ctx clear postgres >/dev/null
ctx clear mysql >/dev/null
test ! -e .ctx

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
