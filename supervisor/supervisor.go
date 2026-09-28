// Package supervisor runs generic local component processes and projects their
// observed lifecycle into a graph. It does not decide why a process is trusted
// or whether a consumer is authorized to start it.
package supervisor

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/webong/ctx/graph"
)

const RuntimeNamespace = "ctx.runtime"

type Artifact struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Checksum string `json:"checksum"`
}
type Endpoint struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Transport string `json:"transport"`
	Direction string `json:"direction,omitempty"`
}
type Connection struct {
	ID             string            `json:"id"`
	SourceEndpoint string            `json:"source_endpoint"`
	TargetEndpoint string            `json:"target_endpoint"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}
type SecretReference struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
}
type Spec struct {
	Artifact         Artifact
	Command          string
	Args             []string
	Dir              string
	Environment      map[string]string
	SecretReferences map[string]SecretReference
	Endpoints        []Endpoint
	Connections      []Connection
	Dependencies     []string
	Restart          RestartPolicy
	HealthCheck      func(context.Context) error
}
type RestartPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}
type State string

const (
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateStopping   State = "stopping"
	StateStopped    State = "stopped"
	StateCrashed    State = "crashed"
	StateRestarting State = "restarting"
)

type Instance struct {
	ID           string       `json:"id"`
	Artifact     Artifact     `json:"artifact"`
	State        State        `json:"state"`
	PID          int          `json:"pid,omitempty"`
	StartedAt    time.Time    `json:"started_at,omitempty"`
	UpdatedAt    time.Time    `json:"updated_at"`
	ExitCode     *int         `json:"exit_code,omitempty"`
	CrashCount   int          `json:"crash_count"`
	Health       string       `json:"health,omitempty"`
	LastError    string       `json:"last_error,omitempty"`
	Endpoints    []Endpoint   `json:"endpoints,omitempty"`
	Connections  []Connection `json:"connections,omitempty"`
	Dependencies []string     `json:"dependencies,omitempty"`
}
type Options struct {
	Graph          graph.Store
	SecretResolver func(context.Context, SecretReference) (string, error)
	LogLimit       int
	RuntimeID      string
}
type Supervisor struct {
	mu        sync.RWMutex
	graph     graph.Store
	resolver  func(context.Context, SecretReference) (string, error)
	logLimit  int
	runtimeID string
	instances map[string]*managed
	closed    bool
}
type managed struct {
	mu       sync.Mutex
	spec     Spec
	instance Instance
	cancel   context.CancelFunc
	done     chan struct{}
	cmd      *exec.Cmd
	logs     *boundedBuffer
}

func New(opts Options) (*Supervisor, error) {
	if opts.Graph == nil {
		return nil, errors.New("graph store is required")
	}
	if opts.LogLimit <= 0 {
		opts.LogLimit = 64 * 1024
	}
	if opts.RuntimeID == "" {
		opts.RuntimeID = "local"
	}
	if err := opts.Graph.Register(graph.Schema{Namespace: RuntimeNamespace, Version: "1"}); err != nil {
		return nil, err
	}
	return &Supervisor{graph: opts.Graph, resolver: opts.SecretResolver, logLimit: opts.LogLimit, runtimeID: opts.RuntimeID, instances: map[string]*managed{}}, nil
}

func (s *Supervisor) Start(ctx context.Context, spec Spec) (Instance, error) {
	if spec.Command == "" || spec.Artifact.ID == "" || spec.Artifact.Revision == "" {
		return Instance{}, errors.New("artifact ID, artifact revision, and command are required")
	}
	for name, ref := range spec.SecretReferences {
		if name == "" || ref.ID == "" || ref.Scope == "" {
			return Instance{}, errors.New("secret environment entries require an opaque scoped reference")
		}
	}
	endpointIDs := map[string]bool{}
	for _, endpoint := range spec.Endpoints {
		if endpoint.ID == "" || endpoint.Address == "" || endpoint.Transport == "" || endpointIDs[endpoint.ID] {
			return Instance{}, errors.New("IPC endpoints require unique IDs, addresses, and transports")
		}
		endpointIDs[endpoint.ID] = true
	}
	connectionIDs := map[string]bool{}
	for _, connection := range spec.Connections {
		if connection.ID == "" || connectionIDs[connection.ID] || !endpointIDs[connection.SourceEndpoint] || !endpointIDs[connection.TargetEndpoint] {
			return Instance{}, errors.New("IPC connections require unique IDs and declared endpoint references")
		}
		connectionIDs[connection.ID] = true
	}
	if len(spec.SecretReferences) > 0 && s.resolver == nil {
		return Instance{}, errors.New("secret references require a secret resolver")
	}
	id, err := newID()
	if err != nil {
		return Instance{}, err
	}
	inst := Instance{ID: id, Artifact: spec.Artifact, State: StateStarting, UpdatedAt: time.Now().UTC(), Endpoints: append([]Endpoint(nil), spec.Endpoints...), Connections: cloneConnections(spec.Connections), Dependencies: append([]string(nil), spec.Dependencies...)}
	mctx, cancel := context.WithCancel(context.Background())
	m := &managed{spec: cloneSpec(spec), instance: inst, cancel: cancel, done: make(chan struct{}), logs: newBoundedBuffer(s.logLimit)}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		cancel()
		return Instance{}, errors.New("supervisor is closed")
	}
	s.instances[id] = m
	s.mu.Unlock()
	if err := s.project(ctx, m); err != nil {
		s.mu.Lock()
		delete(s.instances, id)
		s.mu.Unlock()
		cancel()
		return Instance{}, err
	}
	go s.run(mctx, m)
	return cloneInstance(inst), nil
}

func (s *Supervisor) run(ctx context.Context, m *managed) {
	defer close(m.done)
	attempt := 0
	for {
		m.mu.Lock()
		m.instance.State = StateStarting
		m.instance.UpdatedAt = time.Now().UTC()
		m.instance.LastError = ""
		m.mu.Unlock()
		_ = s.project(context.Background(), m)
		cmd := exec.Command(m.spec.Command, m.spec.Args...)
		cmd.Dir = m.spec.Dir
		env := append([]string(nil), os.Environ()...)
		for k, v := range m.spec.Environment {
			env = append(env, k+"="+v)
		}
		for k, ref := range m.spec.SecretReferences {
			secret, err := s.resolver(ctx, ref)
			if err != nil {
				m.mu.Lock()
				m.instance.State = StateCrashed
				m.instance.LastError = "secret resolution failed"
				m.instance.UpdatedAt = time.Now().UTC()
				m.mu.Unlock()
				_ = s.project(context.Background(), m)
				return
			}
			env = append(env, k+"="+secret)
		}
		cmd.Env = env
		cmd.Stdout = m.logs
		cmd.Stderr = m.logs
		if err := cmd.Start(); err != nil {
			m.mu.Lock()
			m.instance.State = StateCrashed
			m.instance.LastError = err.Error()
			m.instance.CrashCount++
			m.mu.Unlock()
			_ = s.project(context.Background(), m)
			attempt++
			if !shouldRestart(m.spec.Restart, attempt) {
				return
			}
			if !sleepContext(ctx, backoff(m.spec.Restart, attempt)) {
				return
			}
			continue
		}
		m.mu.Lock()
		m.cmd = cmd
		m.instance.PID = cmd.Process.Pid
		m.instance.State = StateRunning
		m.instance.UpdatedAt = time.Now().UTC()
		if m.instance.StartedAt.IsZero() {
			m.instance.StartedAt = m.instance.UpdatedAt
		}
		m.mu.Unlock()
		_ = s.project(context.Background(), m)
		waitErr := waitCommand(ctx, cmd)
		code := 0
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		m.mu.Lock()
		m.cmd = nil
		m.instance.PID = 0
		m.instance.ExitCode = &code
		m.instance.UpdatedAt = time.Now().UTC()
		if ctx.Err() != nil {
			m.instance.State = StateStopped
			m.instance.LastError = ""
		} else if waitErr != nil {
			m.instance.State = StateCrashed
			m.instance.CrashCount++
			m.instance.LastError = waitErr.Error()
		} else {
			m.instance.State = StateStopped
		}
		state := m.instance.State
		m.mu.Unlock()
		_ = s.project(context.Background(), m)
		if state != StateCrashed {
			return
		}
		attempt++
		if !shouldRestart(m.spec.Restart, attempt) {
			return
		}
		m.mu.Lock()
		m.instance.State = StateRestarting
		m.instance.UpdatedAt = time.Now().UTC()
		m.mu.Unlock()
		_ = s.project(context.Background(), m)
		if !sleepContext(ctx, backoff(m.spec.Restart, attempt)) {
			m.mu.Lock()
			m.instance.State = StateStopped
			m.instance.UpdatedAt = time.Now().UTC()
			m.mu.Unlock()
			_ = s.project(context.Background(), m)
			return
		}
	}
}

func waitCommand(ctx context.Context, cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return <-done
	}
}
func shouldRestart(p RestartPolicy, attempt int) bool {
	return p.MaxAttempts > 0 && attempt <= p.MaxAttempts
}
func backoff(p RestartPolicy, attempt int) time.Duration {
	d := p.InitialDelay
	if d <= 0 {
		d = 100 * time.Millisecond
	}
	for i := 1; i < attempt && d < 30*time.Second; i++ {
		d *= 2
	}
	max := p.MaxDelay
	if max <= 0 {
		max = 30 * time.Second
	}
	if d > max {
		return max
	}
	return d
}
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Supervisor) Stop(ctx context.Context, id string) error {
	m, err := s.lookup(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.instance.State == StateStopped || m.instance.State == StateCrashed {
		m.mu.Unlock()
		return nil
	}
	m.instance.State = StateStopping
	m.instance.UpdatedAt = time.Now().UTC()
	m.cancel()
	m.mu.Unlock()
	_ = s.project(context.Background(), m)
	select {
	case <-m.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Supervisor) Health(ctx context.Context, id string) (bool, error) {
	m, err := s.lookup(id)
	if err != nil {
		return false, err
	}
	if m.spec.HealthCheck == nil {
		return true, nil
	}
	err = m.spec.HealthCheck(ctx)
	m.mu.Lock()
	if err != nil {
		m.instance.Health = "unhealthy"
	} else {
		m.instance.Health = "healthy"
	}
	m.instance.UpdatedAt = time.Now().UTC()
	m.mu.Unlock()
	if projectErr := s.project(context.Background(), m); projectErr != nil {
		return err == nil, projectErr
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
func (s *Supervisor) Instance(id string) (Instance, error) {
	m, err := s.lookup(id)
	if err != nil {
		return Instance{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneInstance(m.instance), nil
}
func (s *Supervisor) Logs(id string) (string, error) {
	m, err := s.lookup(id)
	if err != nil {
		return "", err
	}
	return m.logs.String(), nil
}
func (s *Supervisor) Instances() []Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Instance{}
	for _, m := range s.instances {
		m.mu.Lock()
		out = append(out, cloneInstance(m.instance))
		m.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Supervisor) lookup(id string) (*managed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.instances[id]
	if m == nil {
		return nil, fmt.Errorf("process instance %s not found", id)
	}
	return m, nil
}
func (s *Supervisor) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	ids := []string{}
	for id := range s.instances {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	var first error
	for _, id := range ids {
		if err := s.Stop(ctx, id); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Supervisor) project(ctx context.Context, m *managed) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	inst := cloneInstance(m.instance)
	spec := cloneSpec(m.spec)
	m.mu.Unlock()
	attrs := map[string]any{"artifact_id": inst.Artifact.ID, "artifact_revision": inst.Artifact.Revision, "state": string(inst.State), "runtime_id": s.runtimeID, "crash_count": inst.CrashCount}
	if inst.Health != "" {
		attrs["health"] = inst.Health
	}
	if inst.Artifact.Checksum != "" {
		attrs["artifact_checksum"] = inst.Artifact.Checksum
	}
	if inst.PID > 0 {
		attrs["pid"] = inst.PID
	}
	if inst.LastError != "" {
		attrs["last_error"] = inst.LastError
	}
	if !inst.StartedAt.IsZero() {
		attrs["started_at"] = inst.StartedAt.Format(time.RFC3339Nano)
	}
	vertices := []graph.Vertex{{ID: "artifact/" + inst.Artifact.ID, Kind: RuntimeNamespace + "/artifact", Attributes: map[string]any{"revision": inst.Artifact.Revision, "checksum": inst.Artifact.Checksum}, Provenance: graph.Provenance{Source: "ctx.supervisor", Operation: "artifact-observed"}}, {ID: "process/" + inst.ID, Kind: RuntimeNamespace + "/process-instance", Attributes: attrs, Provenance: graph.Provenance{Source: "ctx.supervisor", Operation: "lifecycle-observed"}}, {ID: "runtime/" + s.runtimeID, Kind: RuntimeNamespace + "/runtime", Attributes: map[string]any{"runtime_id": s.runtimeID}, Provenance: graph.Provenance{Source: "ctx.supervisor", Operation: "runtime-observed"}}}
	edges := []graph.Edge{{ID: "spawned-by/" + inst.ID, From: "process/" + inst.ID, To: "artifact/" + inst.Artifact.ID, Type: RuntimeNamespace + "/spawned-by"}, {ID: "supervises/" + inst.ID, From: "runtime/" + s.runtimeID, To: "process/" + inst.ID, Type: RuntimeNamespace + "/supervises"}}
	for _, dep := range spec.Dependencies {
		vertices = append(vertices, graph.Vertex{ID: "dependency/" + dep, Kind: RuntimeNamespace + "/component", Attributes: map[string]any{"component_id": dep}, Provenance: graph.Provenance{Source: "ctx.supervisor", Operation: "dependency-declared"}})
		edges = append(edges, graph.Edge{ID: "depends-on/" + inst.ID + "/" + dep, From: "process/" + inst.ID, To: "dependency/" + dep, Type: RuntimeNamespace + "/depends-on"})
	}
	for _, endpoint := range spec.Endpoints {
		vertices = append(vertices, graph.Vertex{ID: "endpoint/" + endpoint.ID, Kind: RuntimeNamespace + "/endpoint", Attributes: map[string]any{"address": endpoint.Address, "transport": endpoint.Transport, "direction": endpoint.Direction}, Provenance: graph.Provenance{Source: "ctx.supervisor", Operation: "endpoint-observed"}})
		edges = append(edges, graph.Edge{ID: "exposes-endpoint/" + inst.ID + "/" + endpoint.ID, From: "process/" + inst.ID, To: "endpoint/" + endpoint.ID, Type: RuntimeNamespace + "/exposes-endpoint"})
	}
	for _, connection := range spec.Connections {
		attrs := map[string]any{}
		for k, v := range connection.Metadata {
			attrs[k] = v
		}
		edges = append(edges, graph.Edge{ID: "connected-to/" + connection.ID, From: "endpoint/" + connection.SourceEndpoint, To: "endpoint/" + connection.TargetEndpoint, Type: RuntimeNamespace + "/connected-to", Attributes: attrs})
	}
	_, err := s.graph.Apply(ctx, graph.Transaction{Namespace: RuntimeNamespace, Vertices: vertices, Edges: edges})
	return err
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func cloneInstance(i Instance) Instance {
	i.Endpoints = append([]Endpoint(nil), i.Endpoints...)
	i.Connections = cloneConnections(i.Connections)
	i.Dependencies = append([]string(nil), i.Dependencies...)
	if i.ExitCode != nil {
		x := *i.ExitCode
		i.ExitCode = &x
	}
	return i
}
func cloneSpec(s Spec) Spec {
	s.Args = append([]string(nil), s.Args...)
	s.Dependencies = append([]string(nil), s.Dependencies...)
	s.Endpoints = append([]Endpoint(nil), s.Endpoints...)
	s.Connections = cloneConnections(s.Connections)
	env := map[string]string{}
	for k, v := range s.Environment {
		env[k] = v
	}
	s.Environment = env
	refs := map[string]SecretReference{}
	for k, v := range s.SecretReferences {
		refs[k] = v
	}
	s.SecretReferences = refs
	return s
}

func cloneConnections(connections []Connection) []Connection {
	out := make([]Connection, len(connections))
	for i, connection := range connections {
		out[i] = connection
		if connection.Metadata != nil {
			out[i].Metadata = map[string]string{}
			for k, v := range connection.Metadata {
				out[i].Metadata[k] = v
			}
		}
	}
	return out
}

type boundedBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func newBoundedBuffer(limit int) *boundedBuffer { return &boundedBuffer{limit: limit} }
func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if n >= b.limit {
		b.data = append(b.data[:0], p[n-b.limit:]...)
		return n, nil
	}
	overflow := len(b.data) + n - b.limit
	if overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
	}
	b.data = append(b.data, p...)
	return n, nil
}
func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.Clone(b.data))
}

var _ io.Writer = (*boundedBuffer)(nil)
