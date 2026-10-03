package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Backend is a consumer-selected transport binding. Implementations must honor
// cancellation, permit concurrent Invoke calls, and make Close interrupt I/O.
// Backends never decide product authorization. A CLI binding may preserve raw
// streams outside this JSON invocation API and use ValidateRequest directly.
type Backend interface {
	Handshake(context.Context) (Descriptor, error)
	Invoke(context.Context, Request) (Response, error)
	Close() error
}

type Options struct {
	// Verify must check the reviewed package selection before Connect may run.
	// A digest proves content equality, not publisher trust or permission.
	Verify  func(context.Context, Descriptor) error
	Connect func(context.Context, Descriptor) (Backend, error)
	// Authorize is required, called on every invocation, and receives copies.
	Authorize func(context.Context, Request) error
	// Timeout bounds handshake and calls without an earlier caller deadline.
	Timeout time.Duration
}

type State string

const (
	StateReady    State = "ready"
	StateDraining State = "draining"
	StateClosed   State = "closed"
	StateFailed   State = "failed"
)

// Session owns invocation admission and draining. Process start/stop/restart,
// leases, and resource controls remain in the supervisor package. A process
// restart requires a fresh Session and handshake.
type Session struct {
	descriptor Descriptor
	backend    Backend
	authorize  func(context.Context, Request) error
	timeout    time.Duration
	mu         sync.Mutex
	state      State
	next       uint64
	active     int
	idle       chan struct{}
	closeOnce  sync.Once
	closeErr   error
}

// Open verifies selection before opening a backend, then checks its exact
// handshake. Nil policy callbacks fail closed. No global registry is used.
func Open(ctx context.Context, selected Descriptor, opts Options) (*Session, error) {
	selected = selected.Clone()
	if err := selected.Validate(); err != nil {
		return nil, err
	}
	if opts.Verify == nil || opts.Authorize == nil {
		return nil, ErrDenied
	}
	if opts.Connect == nil || opts.Timeout < 0 {
		return nil, ErrInvalid
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := opts.Verify(ctx, selected.Clone()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDenied, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	backend, err := opts.Connect(ctx, selected.Clone())
	if err != nil {
		if backend != nil {
			_ = backend.Close()
		}
		return nil, err
	}
	if backend == nil {
		return nil, ErrInvalid
	}
	actual, err := backend.Handshake(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = MatchHandshake(selected, actual)
	}
	if err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	idle := make(chan struct{})
	close(idle)
	return &Session{descriptor: selected, backend: backend, authorize: opts.Authorize, timeout: opts.Timeout, state: StateReady, idle: idle}, nil
}

func (s *Session) Descriptor() Descriptor { return s.descriptor.Clone() }
func (s *Session) State() State           { s.mu.Lock(); defer s.mu.Unlock(); return s.state }

// Call derives identity, surface, deadline, and a fresh correlation ID from
// host state. A payload cannot expand the selected descriptor. Request IDs do
// not supply idempotency, authorization, or automatic retries.
func (s *Session) Call(ctx context.Context, contract ContractRef, operation string, payload json.RawMessage) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	op, err := s.descriptor.Lookup(contract, operation)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.state != StateReady {
		err := ErrClosed
		if s.state == StateDraining {
			err = ErrDraining
		}
		s.mu.Unlock()
		return nil, err
	}
	if s.active == 0 {
		s.idle = make(chan struct{})
	}
	s.active++
	s.next++
	id := strconv.FormatUint(s.next, 10)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.active--
		if s.active == 0 {
			close(s.idle)
		}
		s.mu.Unlock()
	}()
	deadline, _ := ctx.Deadline()
	request := Request{APIVersion: APIVersion, ID: id, Plugin: s.descriptor.Identity, Contract: contract, Operation: operation, Surface: op.Surface, Deadline: deadline, Payload: append(json.RawMessage(nil), payload...)}
	if err := ValidateRequest(s.descriptor, request); err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, request.Clone()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDenied, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	response, err := s.backend.Invoke(ctx, request)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = response.Validate(id)
	}
	if err != nil {
		s.fail()
		return nil, err
	}
	if response.Error != nil {
		e := *response.Error
		return nil, &e
	}
	return append(json.RawMessage(nil), response.Payload...), nil
}

// Close first rejects new invocations, then drains admitted work. A deadline
// while draining reopens admission without closing the backend. Concurrent
// Close attempts receive ErrDraining. An already closed session is idempotent.
func (s *Session) Close(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	switch s.state {
	case StateClosed, StateFailed:
		s.mu.Unlock()
		return s.closeBackend()
	case StateDraining:
		s.mu.Unlock()
		return ErrDraining
	}
	s.state = StateDraining
	idle := s.idle
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		s.mu.Lock()
		if s.state == StateDraining {
			s.state = StateReady
		}
		s.mu.Unlock()
		return ctx.Err()
	case <-idle:
		s.mu.Lock()
		if s.state == StateDraining {
			s.state = StateClosed
		}
		s.mu.Unlock()
		return s.closeBackend()
	}
}

// Abort interrupts the transport immediately. The owning host must separately
// stop any process through its supervisor. It is safe alongside Call or Close.
func (s *Session) Abort() error {
	s.mu.Lock()
	s.state = StateClosed
	s.mu.Unlock()
	return s.closeBackend()
}
func (s *Session) fail() {
	s.mu.Lock()
	if s.state != StateClosed {
		s.state = StateFailed
	}
	s.mu.Unlock()
	_ = s.closeBackend()
}
func (s *Session) closeBackend() error {
	s.closeOnce.Do(func() { s.closeErr = s.backend.Close() })
	return s.closeErr
}
