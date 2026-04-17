// Copyright 2026 The go-vinu Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.

package rpc

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// captureWriter records every writeJSON call so tests can inspect the
// responses emitted onto the underlying connection.
type captureWriter struct {
	mu       sync.Mutex
	messages []*jsonrpcMessage
	remote   string
	done     chan struct{}
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{done: make(chan struct{}, 16)}
}

func (c *captureWriter) writeJSON(ctx context.Context, v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch m := v.(type) {
	case *jsonrpcMessage:
		c.messages = append(c.messages, m)
	case []*jsonrpcMessage:
		c.messages = append(c.messages, m...)
	default:
		// Re-marshal unexpected types so tests can still assert structure.
		raw, _ := json.Marshal(v)
		msg := new(jsonrpcMessage)
		_ = json.Unmarshal(raw, msg)
		c.messages = append(c.messages, msg)
	}
	select {
	case c.done <- struct{}{}:
	default:
	}
	return nil
}

func (c *captureWriter) remoteAddr() string { return c.remote }
func (c *captureWriter) close()             {}
func (c *captureWriter) closed() <-chan interface{} {
	ch := make(chan interface{})
	return ch
}

func (c *captureWriter) snapshot() []*jsonrpcMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*jsonrpcMessage, len(c.messages))
	copy(out, c.messages)
	return out
}

func (c *captureWriter) waitForMessage(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for response on stopping handler")
	}
}

// newStoppingHandler builds a handler whose serviceRegistry is already
// marked as stopping, mimicking the state after Server.Stop() has flipped
// the flag but before the connection is torn down.
func newStoppingHandler() (*handler, *captureWriter) {
	conn := newCaptureWriter()
	reg := &serviceRegistry{stopping: true}
	h := newHandler(context.Background(), conn, randomIDGenerator(), reg)
	return h, conn
}

// --- startCallProc behaviour on stopping handler --------------------------

func TestStartCallProc_ReportsNotScheduledOnStopping(t *testing.T) {
	h, _ := newStoppingHandler()

	scheduled := h.startCallProc(func(cp *callProc) {
		t.Fatal("fn must not run when handler is stopping")
	})
	if scheduled {
		t.Fatal("startCallProc must return false when reg.stopping is true")
	}
}

func TestStartCallProc_RunsFnWhenNotStopping(t *testing.T) {
	conn := newCaptureWriter()
	reg := &serviceRegistry{stopping: false}
	h := newHandler(context.Background(), conn, randomIDGenerator(), reg)

	done := make(chan struct{})
	scheduled := h.startCallProc(func(cp *callProc) { close(done) })
	if !scheduled {
		t.Fatal("startCallProc must schedule fn when not stopping")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduled fn did not run in time")
	}
}

// --- handleMsg / handleBatch must emit a shutdown error ------------------

func TestHandleMsg_WhileStopping_EmitsShutdownError(t *testing.T) {
	h, conn := newStoppingHandler()
	msg := &jsonrpcMessage{Version: vsn, ID: json.RawMessage(`7`), Method: "rpc_modules"}

	h.handleMsg(msg)

	msgs := conn.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 response, got %d", len(msgs))
	}
	resp := msgs[0]
	if resp.Error == nil {
		t.Fatal("response must be an error; stopping handler silently dropped the call")
	}
	if string(resp.ID) != "7" {
		t.Fatalf("response ID must echo request ID 7, got %s", string(resp.ID))
	}
	if resp.Error.Code != defaultErrorCode {
		t.Fatalf("expected error code %d, got %d", defaultErrorCode, resp.Error.Code)
	}
	if resp.Error.Message == "" {
		t.Fatal("error message must describe the shutdown state")
	}
}

func TestHandleBatch_WhileStopping_EmitsShutdownErrorPerMessage(t *testing.T) {
	h, conn := newStoppingHandler()
	batch := []*jsonrpcMessage{
		{Version: vsn, ID: json.RawMessage(`1`), Method: "rpc_modules"},
		{Version: vsn, ID: json.RawMessage(`2`), Method: "rpc_modules"},
		{Version: vsn, ID: json.RawMessage(`3`), Method: "rpc_modules"},
	}

	h.handleBatch(batch)

	msgs := conn.snapshot()
	if len(msgs) != len(batch) {
		t.Fatalf("expected %d responses, got %d", len(batch), len(msgs))
	}
	seen := make(map[string]bool)
	for _, m := range msgs {
		if m.Error == nil {
			t.Fatalf("batch response missing error: %+v", m)
		}
		seen[string(m.ID)] = true
	}
	for _, req := range batch {
		if !seen[string(req.ID)] {
			t.Fatalf("batch response missing for id %s", string(req.ID))
		}
	}
}

func TestHandleBatch_EmptyWhileStopping_EmitsSingleError(t *testing.T) {
	h, conn := newStoppingHandler()

	h.handleBatch(nil)

	msgs := conn.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 shutdown error for empty batch, got %d", len(msgs))
	}
	if msgs[0].Error == nil {
		t.Fatal("empty-batch shutdown response must carry an error")
	}
}

func TestHandleBatch_OversizedWhileStopping_EmitsError(t *testing.T) {
	h, conn := newStoppingHandler()

	oversized := make([]*jsonrpcMessage, maxBatchSize+1)
	for i := range oversized {
		oversized[i] = &jsonrpcMessage{Version: vsn, ID: json.RawMessage(`1`), Method: "rpc_modules"}
	}

	h.handleBatch(oversized)

	msgs := conn.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 shutdown error for oversized batch, got %d", len(msgs))
	}
	if msgs[0].Error == nil {
		t.Fatal("oversized-batch shutdown response must carry an error")
	}
}

// --- Concurrent callers during shutdown all receive responses ------------

func TestConcurrentCallsDuringShutdown_AllGetErrorResponses(t *testing.T) {
	h, conn := newStoppingHandler()

	const concurrent = 16
	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, _ := json.Marshal(i)
			h.handleMsg(&jsonrpcMessage{
				Version: vsn,
				ID:      id,
				Method:  "rpc_modules",
			})
		}(i)
	}
	wg.Wait()

	msgs := conn.snapshot()
	if len(msgs) != concurrent {
		t.Fatalf("expected %d responses, got %d", concurrent, len(msgs))
	}
	for i, m := range msgs {
		if m.Error == nil {
			t.Fatalf("response %d is not an error: %+v", i, m)
		}
	}
}
