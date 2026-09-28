package supervisor

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/webong/ctx/graph"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("CTX_SUPERVISOR_HELPER") != "1" {
		return
	}
	if os.Getenv("CTX_SUPERVISOR_HOLD") == "1" {
		time.Sleep(30 * time.Second)
	}
	if count, _ := strconv.Atoi(os.Getenv("CTX_SUPERVISOR_OUTPUT_BYTES")); count > 0 {
		_, _ = os.Stdout.WriteString(strings.Repeat("x", count))
	}
	if code, _ := strconv.Atoi(os.Getenv("CTX_SUPERVISOR_EXIT_CODE")); code != 0 {
		os.Exit(code)
	}
	_, _ = os.Stdout.WriteString("runtime-output\n")
	os.Exit(0)
}

func TestCrashRestartAndBoundedLogs(t *testing.T) {
	store := graph.NewMemory()
	defer store.Close()
	sup, err := New(Options{Graph: store, LogLimit: 128})
	if err != nil {
		t.Fatal(err)
	}
	defer sup.Close(context.Background())
	instance, err := sup.Start(context.Background(), Spec{
		Artifact: Artifact{ID: "crashing", Revision: "1"}, Command: os.Args[0], Args: []string{"-test.run=TestHelperProcess"},
		Environment: map[string]string{"CTX_SUPERVISOR_HELPER": "1", "CTX_SUPERVISOR_EXIT_CODE": "7", "CTX_SUPERVISOR_OUTPUT_BYTES": "4096"},
		Restart:     RestartPolicy{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		i, e := sup.Instance(instance.ID)
		return e == nil && i.State == StateCrashed && i.CrashCount == 2
	})
	logs, err := sup.Logs(instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) > 128 {
		t.Fatalf("log tail exceeded limit: %d", len(logs))
	}
	process, err := store.GetVertex(context.Background(), RuntimeNamespace, "process/"+instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if process.Attributes["crash_count"] != float64(2) {
		t.Fatalf("crash fact missing: %#v", process.Attributes)
	}
}

func TestLifecycleFactsAndSecretReferences(t *testing.T) {
	ctx := context.Background()
	store := graph.NewMemory()
	defer store.Close()
	secret := "never-project-this-value"
	sup, err := New(Options{Graph: store, SecretResolver: func(context.Context, SecretReference) (string, error) { return secret, nil }, LogLimit: 128})
	if err != nil {
		t.Fatal(err)
	}
	defer sup.Close(context.Background())
	instance, err := sup.Start(ctx, Spec{Artifact: Artifact{ID: "artifact-1", Revision: "rev-3", Checksum: "sha256:abc"}, Command: os.Args[0], Args: []string{"-test.run=TestHelperProcess"}, Environment: map[string]string{"CTX_SUPERVISOR_HELPER": "1"}, SecretReferences: map[string]SecretReference{"EXAMPLE_SECRET": {Scope: "project:demo", ID: "db-password"}}, Endpoints: []Endpoint{{ID: "ipc-1", Address: "unix:///tmp/example.sock", Transport: "unix"}, {ID: "ipc-2", Address: "tcp://127.0.0.1:7000", Transport: "tcp"}}, Connections: []Connection{{ID: "link-1", SourceEndpoint: "ipc-1", TargetEndpoint: "ipc-2", Metadata: map[string]string{"channel": "control"}}}})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { i, e := sup.Instance(instance.ID); return e == nil && i.State == StateStopped })
	logs, err := sup.Logs(instance.ID)
	if err != nil || !strings.Contains(logs, "runtime-output") {
		t.Fatalf("bounded process logs: %q, %v", logs, err)
	}
	process, err := store.GetVertex(ctx, RuntimeNamespace, "process/"+instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if process.Attributes["state"] != string(StateStopped) {
		t.Fatalf("graph did not receive stopped fact: %#v", process.Attributes)
	}
	if _, ok := process.Attributes["authorized"]; ok {
		t.Fatal("runtime fact implies domain authority")
	}
	snapshot, err := store.Snapshot(ctx, RuntimeNamespace)
	if err != nil {
		t.Fatal(err)
	}
	serialized := ""
	for _, v := range snapshot.Vertices {
		serialized += v.ID + " "
		for k, val := range v.Attributes {
			serialized += k + " "
			if x, ok := val.(string); ok {
				serialized += x + " "
			}
		}
	}
	if strings.Contains(serialized, secret) {
		t.Fatal("secret value was projected into graph")
	}
	if _, err = store.GetEdge(ctx, RuntimeNamespace, "spawned-by/"+instance.ID); err != nil {
		t.Fatalf("missing generic spawned-by relationship: %v", err)
	}
	if _, err = store.GetEdge(ctx, RuntimeNamespace, "connected-to/link-1"); err != nil {
		t.Fatalf("missing generic IPC connection fact: %v", err)
	}
}

func TestStopAndHealthProjection(t *testing.T) {
	store := graph.NewMemory()
	defer store.Close()
	sup, err := New(Options{Graph: store})
	if err != nil {
		t.Fatal(err)
	}
	defer sup.Close(context.Background())
	instance, err := sup.Start(context.Background(), Spec{Artifact: Artifact{ID: "long", Revision: "1"}, Command: os.Args[0], Args: []string{"-test.run=TestHelperProcess"}, Environment: map[string]string{"CTX_SUPERVISOR_HELPER": "1", "CTX_SUPERVISOR_HOLD": "1"}, HealthCheck: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { i, e := sup.Instance(instance.ID); return e == nil && i.State == StateRunning })
	healthy, err := sup.Health(context.Background(), instance.ID)
	if err != nil || !healthy {
		t.Fatalf("health=%v err=%v", healthy, err)
	}
	if err = sup.Stop(context.Background(), instance.ID); err != nil {
		t.Fatal(err)
	}
	i, err := sup.Instance(instance.ID)
	if err != nil || i.State != StateStopped {
		t.Fatalf("stop lifecycle: %+v %v", i, err)
	}
	process, err := store.GetVertex(context.Background(), RuntimeNamespace, "process/"+instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if process.Attributes["health"] != "healthy" {
		t.Fatalf("health fact missing: %#v", process.Attributes)
	}
}

func waitFor(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}
