package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServeHTTP_ConcurrencyLimit_Rejects(t *testing.T) {
	server := newTestServer()
	defer server.Stop()
	server.SetConcurrencyLimit(1)

	// Register a slow method so the first request holds the semaphore.
	server.RegisterName("test", new(testService))

	ts := httptest.NewServer(server)
	defer ts.Close()

	// Block the single slot with a slow call (duration is nanoseconds in JSON).
	var wg sync.WaitGroup
	blocked := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		body := `{"jsonrpc":"2.0","id":1,"method":"test_sleep","params":[500000000]}`
		req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		close(blocked)
		http.DefaultClient.Do(req)
	}()

	<-blocked
	// Give the goroutine time to acquire the semaphore.
	time.Sleep(100 * time.Millisecond)

	// Second request should be rejected with 503.
	body := `{"jsonrpc":"2.0","id":2,"method":"test_echo","params":["hello",10,"0x68656c6c6f"]}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	wg.Wait()
}

func TestServeHTTP_ConcurrencyLimit_Zero_Unlimited(t *testing.T) {
	server := newTestServer()
	defer server.Stop()
	// Zero means unlimited (default).
	server.SetConcurrencyLimit(0)

	ts := httptest.NewServer(server)
	defer ts.Close()

	// Multiple concurrent requests should all succeed.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"jsonrpc":"2.0","id":1,"method":"rpc_modules"}`
			req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
}

func TestServeHTTP_ConcurrencyLimit_AllowsAfterRelease(t *testing.T) {
	server := newTestServer()
	defer server.Stop()
	server.SetConcurrencyLimit(1)

	ts := httptest.NewServer(server)
	defer ts.Close()

	// First request succeeds and releases the slot.
	body := `{"jsonrpc":"2.0","id":1,"method":"rpc_modules"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Second request should also succeed (slot was released).
	req2, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	req2.Header.Set("Content-Type", contentType)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after release, got %d", resp2.StatusCode)
	}
}
