package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestPurgeCloudflareCache(t *testing.T) {
	t.Run("success", func(t *testing.T) {

		mux := http.NewServeMux()
		mux.HandleFunc("/client/v4/zones/test-zone/purge_cache", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("unexpected authorization header: %s", got)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("unexpected content type: %s", got)
			}

			var payload purgeRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode payload: %v", err)
			}
			if len(payload.Files) != 2 {
				t.Fatalf("expected 2 files, got %d", len(payload.Files))
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		})

		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		origBaseURL, origClient := apiBaseURL, httpClient
		apiBaseURL = server.URL
		httpClient = server.Client()
		t.Cleanup(func() {
			apiBaseURL = origBaseURL
			httpClient = origClient
		})

		err := PurgeCloudflareCache(context.Background(), "test-token", "test-zone", []string{"https://example.com/script.js", "https://example.com/style.css"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
	})

	t.Run("api error", func(t *testing.T) {

		mux := http.NewServeMux()
		mux.HandleFunc("/client/v4/zones/test-zone/purge_cache", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"Invalid zone ID"}]}`))
		})

		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		origBaseURL, origClient := apiBaseURL, httpClient
		apiBaseURL = server.URL
		httpClient = server.Client()
		t.Cleanup(func() {
			apiBaseURL = origBaseURL
			httpClient = origClient
		})

		err := PurgeCloudflareCache(context.Background(), "test-token", "test-zone", []string{"https://example.com/script.js"})
		if err == nil || !contains(err.Error(), "Invalid zone ID") {
			t.Fatalf("expected error containing 'Invalid zone ID', got: %v", err)
		}
	})

	t.Run("retry transient error", func(t *testing.T) {

		mux := http.NewServeMux()
		mux.HandleFunc("/client/v4/zones/test-zone/purge_cache", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"success":true}`))
		})

		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		origBaseURL, origClient := apiBaseURL, httpClient
		apiBaseURL = server.URL
		httpClient = &http.Client{Transport: &flakyTransport{target: server.Client().Transport}}
		t.Cleanup(func() {
			apiBaseURL = origBaseURL
			httpClient = origClient
		})

		err := PurgeCloudflareCache(context.Background(), "test-token", "test-zone", []string{"https://example.com/script.js"})
		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
	})

	t.Run("invalid json response", func(t *testing.T) {

		mux := http.NewServeMux()
		mux.HandleFunc("/client/v4/zones/test-zone/purge_cache", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not a json`))
		})

		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		origBaseURL, origClient := apiBaseURL, httpClient
		apiBaseURL = server.URL
		httpClient = server.Client()
		t.Cleanup(func() {
			apiBaseURL = origBaseURL
			httpClient = origClient
		})

		err := PurgeCloudflareCache(context.Background(), "test-token", "test-zone", []string{"https://example.com/script.js"})
		if err == nil || !contains(err.Error(), "decode") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})

	t.Run("empty files", func(t *testing.T) {

		err := PurgeCloudflareCache(context.Background(), "test-token", "test-zone", nil)
		if err == nil || !contains(err.Error(), "files") {
			t.Fatalf("expected validation error, got: %v", err)
		}
	})
}

type flakyTransport struct {
	mu       sync.Mutex
	attempts int
	target   http.RoundTripper
}

func (f *flakyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.attempts++
	attempt := f.attempts
	f.mu.Unlock()

	if attempt == 1 {
		return nil, temporaryError{msg: "temporary network failure"}
	}

	if f.target != nil {
		return f.target.RoundTrip(r)
	}
	return http.DefaultTransport.RoundTrip(r)
}

type temporaryError struct {
	msg string
}

func (e temporaryError) Error() string   { return e.msg }
func (e temporaryError) Timeout() bool   { return true }
func (e temporaryError) Temporary() bool { return true }

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
