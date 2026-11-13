package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, upstreamURL string, ttl int) *Server {
	t.Helper()
	return NewServer(Config{
		ListenAddr: ":0",
		Token:      "internal",
		Upstreams: []Upstream{{
			Name:            "test",
			BaseURL:         upstreamURL,
			APIKey:          "upstream-key",
			CacheTTLSeconds: ttl,
			MaxCacheEntries: 10,
		}},
	})
}

func doRequest(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/u/test/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer internal")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestPassthroughCachesIdenticalRequests(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer upstream-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	}))
	defer upstream.Close()

	s := newTestServer(t, upstream.URL, 60)
	body := `{"model":"m","messages":[{"role":"user","content":"hi"}]}`

	first := doRequest(t, s, body)
	require.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "miss", first.Header().Get("X-Optimizer-Cache"))
	assert.Contains(t, first.Body.String(), `"prompt_tokens":10`)

	second := doRequest(t, s, body)
	require.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, "hit", second.Header().Get("X-Optimizer-Cache"))
	assert.Equal(t, first.Body.String(), second.Body.String())
	assert.Equal(t, int64(1), calls.Load(), "cache hit must not reach upstream")
}

func TestDifferentBodyMissesCache(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"usage":{}}`)
	}))
	defer upstream.Close()

	s := newTestServer(t, upstream.URL, 60)
	doRequest(t, s, `{"model":"m","messages":[]}`)
	doRequest(t, s, `{"model":"m","messages":[{"role":"user","content":"x"}]}`)
	assert.Equal(t, int64(2), calls.Load())
}

func TestStreamingIsNeverCached(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, chunk := range []string{"data: {\"a\":1}\n\n", "data: [DONE]\n\n"} {
			_, _ = io.WriteString(w, chunk)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer upstream.Close()

	s := newTestServer(t, upstream.URL, 60)
	body := `{"model":"m","stream":true}`

	for range 2 {
		rec := doRequest(t, s, body)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "bypass", rec.Header().Get("X-Optimizer-Cache"))
		assert.Equal(t, "data: {\"a\":1}\n\ndata: [DONE]\n\n", rec.Body.String())
	}
	assert.Equal(t, int64(2), calls.Load())
}

func TestUpstreamErrorIsNotCached(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer upstream.Close()

	s := newTestServer(t, upstream.URL, 60)
	body := `{"model":"m","messages":[]}`

	first := doRequest(t, s, body)
	require.Equal(t, http.StatusTooManyRequests, first.Code)
	assert.Contains(t, first.Body.String(), "rate limited")

	doRequest(t, s, body)
	assert.Equal(t, int64(2), calls.Load(), "non-200 responses must not be cached")
}

func TestRejectsWrongInternalToken(t *testing.T) {
	s := newTestServer(t, "http://127.0.0.1:1", 0)
	req := httptest.NewRequest(http.MethodPost, "/u/test/v1/chat/completions", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
