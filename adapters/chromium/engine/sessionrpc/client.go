// Package sessionrpc multiplexes the JSON command transport used by native
// browser sessions. It has no browser discovery, page behavior, or ownership.
package sessionrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type response struct {
	ID        uint64          `json:"id"`
	Result    json.RawMessage `json:"result"`
	Error     json.RawMessage `json:"error"`
	Message   string          `json:"message"`
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId"`
	Params    json.RawMessage `json:"params"`
}

// Event is a native protocol notification. A handler must return promptly and
// must not issue synchronous commands from the transport's reader goroutine.
type Event struct {
	Method, SessionID string
	Params            json.RawMessage
}

type Client struct {
	socket   *websocket.Conn
	mu       sync.Mutex
	writeMu  sync.Mutex
	next     uint64
	pending  map[uint64]chan response
	done     chan struct{}
	err      error
	handler  func(Event)
	finished sync.Once
}

// ProtocolError is a command rejection, distinct from loss of the transport.
type ProtocolError struct {
	Method, Kind, Message string
	Code                  int
}

type TransportError struct {
	Method string
	Cause  error
}

func (err *TransportError) Error() string {
	return fmt.Sprintf("%s: native session disconnected: %v", err.Method, err.Cause)
}
func (err *TransportError) Unwrap() error { return err.Cause }

func (err *ProtocolError) Error() string {
	return fmt.Sprintf("%s: native protocol error %s (%d): %s", err.Method, err.Kind, err.Code, err.Message)
}

func decodeReply(method string, reply response, output any) error {
	if len(reply.Error) != 0 && string(reply.Error) != "null" {
		failure := &ProtocolError{Method: method, Message: reply.Message}
		if json.Unmarshal(reply.Error, &failure.Kind) != nil {
			var detail struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(reply.Error, &detail); err != nil {
				return err
			}
			failure.Code, failure.Message = detail.Code, detail.Message
		}
		return failure
	}
	if output != nil {
		return json.Unmarshal(reply.Result, output)
	}
	return nil
}

func (client *Client) fail(err error) {
	client.finished.Do(func() {
		client.mu.Lock()
		client.err = err
		client.mu.Unlock()
		close(client.done)
		client.socket.Close()
	})
}

func Dial(ctx context.Context, endpoint string) (*Client, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second, Proxy: http.ProxyFromEnvironment}
	socket, reply, err := dialer.DialContext(ctx, endpoint, nil)
	if reply != nil && reply.Body != nil {
		reply.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	socket.SetReadLimit(16 << 20)
	if err := socket.SetReadDeadline(time.Now().Add(45 * time.Second)); err != nil {
		socket.Close()
		return nil, err
	}
	socket.SetPongHandler(func(string) error { return socket.SetReadDeadline(time.Now().Add(45 * time.Second)) })
	client := &Client{socket: socket, pending: map[uint64]chan response{}, done: make(chan struct{})}
	go client.read()
	go client.keepAlive()
	return client, nil
}

func (client *Client) keepAlive() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-client.done:
			return
		case <-ticker.C:
			if err := client.socket.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				client.fail(err)
				return
			}
		}
	}
}

func (client *Client) read() {
	for {
		var result response
		if err := client.socket.ReadJSON(&result); err != nil {
			client.fail(err)
			return
		}
		if result.ID == 0 {
			client.mu.Lock()
			handler := client.handler
			client.mu.Unlock()
			if handler != nil {
				handler(Event{Method: result.Method, SessionID: result.SessionID, Params: result.Params})
			}
			continue
		}
		client.mu.Lock()
		receiver := client.pending[result.ID]
		client.mu.Unlock()
		if receiver != nil {
			select {
			case receiver <- result:
			default:
			}
		}
	}
}

// Call sends a command, optionally scoped to a flattened CDP session.
func (client *Client) Call(ctx context.Context, session, method string, params, output any) error {
	if ctx == nil {
		return errors.New("a command context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client.mu.Lock()
	client.next++
	id := client.next
	receiver := make(chan response, 1)
	client.pending[id] = receiver
	client.mu.Unlock()
	defer func() { client.mu.Lock(); delete(client.pending, id); client.mu.Unlock() }()
	command := map[string]any{"id": id, "method": method, "params": params}
	if session != "" {
		command["sessionId"] = session
	}
	client.writeMu.Lock()
	deadline, _ := ctx.Deadline()
	err := client.socket.SetWriteDeadline(deadline)
	if err == nil {
		err = client.socket.WriteJSON(command)
	}
	client.writeMu.Unlock()
	if err != nil {
		client.fail(err)
		return &TransportError{Method: method, Cause: err}
	}
	select {
	case reply := <-receiver:
		return decodeReply(method, reply, output)
	case <-ctx.Done():
		return ctx.Err()
	case <-client.done:
		select {
		case reply := <-receiver:
			return decodeReply(method, reply, output)
		default:
		}
		client.mu.Lock()
		err := client.err
		client.mu.Unlock()
		return &TransportError{Method: method, Cause: err}
	}
}

// Close drops the transport; native browser shutdown is never implied.
func (client *Client) Close() error {
	var closeErr error
	client.finished.Do(func() {
		client.mu.Lock()
		client.err = errors.New("native transport closed")
		client.mu.Unlock()
		close(client.done)
		closeErr = client.socket.Close()
	})
	return closeErr
}

func (client *Client) OnEvent(handler func(Event)) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.handler = handler
}

func (client *Client) Done() <-chan struct{} { return client.done }
func (client *Client) Err() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.err
}
