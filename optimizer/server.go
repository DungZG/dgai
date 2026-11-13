package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const maxRequestBodyBytes = 32 << 20
const maxCacheableResponseBytes = 16 << 20

type cacheEntry struct {
	status int
	header http.Header
	body   []byte
	expiry time.Time
}

// responseCache is a bounded in-memory exact cache. It is intentionally not
// persisted: a restart drops every entry, which keeps stale answers impossible.
type responseCache struct {
	mu    sync.Mutex
	items map[string]*cacheEntry
	max   int
}

func newResponseCache(max int) *responseCache {
	return &responseCache{items: make(map[string]*cacheEntry), max: max}
}

func (c *responseCache) get(key string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiry) {
		delete(c.items, key)
		return nil, false
	}
	return entry, true
}

func (c *responseCache) set(key string, entry *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= c.max {
		now := time.Now()
		var oldestKey string
		var oldestExpiry time.Time
		for k, v := range c.items {
			if now.After(v.expiry) {
				delete(c.items, k)
				continue
			}
			if oldestKey == "" || v.expiry.Before(oldestExpiry) {
				oldestKey, oldestExpiry = k, v.expiry
			}
		}
		if len(c.items) >= c.max && oldestKey != "" {
			delete(c.items, oldestKey)
		}
	}
	c.items[key] = entry
}

// Server routes every request either to health/metrics or to one upstream.
type Server struct {
	cfg    Config
	byName map[string]*Upstream
	client *http.Client
	caches map[string]*responseCache

	requests     atomic.Int64
	upstreamErrs atomic.Int64
	cacheHits    atomic.Int64
	cacheStores  atomic.Int64
}

func NewServer(cfg Config) *Server {
	s := &Server{
		cfg:    cfg,
		byName: make(map[string]*Upstream, len(cfg.Upstreams)),
		client: newUpstreamClient(),
		caches: make(map[string]*responseCache, len(cfg.Upstreams)),
	}
	for i := range cfg.Upstreams {
		up := &cfg.Upstreams[i]
		s.byName[up.Name] = up
		if up.CacheTTLSeconds > 0 {
			s.caches[up.Name] = newResponseCache(up.MaxCacheEntries)
		}
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health/live":
		writeText(w, http.StatusOK, "ok")
		return
	case "/health/ready":
		writeText(w, http.StatusOK, "ready")
		return
	case "/internal/metrics":
		writeJSON(w, http.StatusOK, map[string]int64{
			"requests":        s.requests.Load(),
			"upstream_errors": s.upstreamErrs.Load(),
			"cache_hits":      s.cacheHits.Load(),
			"cache_stores":    s.cacheStores.Load(),
		})
		return
	}

	rest, ok := strings.CutPrefix(r.URL.Path, "/u/")
	if !ok {
		writeText(w, http.StatusNotFound, "unknown route")
		return
	}
	name, path, _ := strings.Cut(rest, "/")
	up, ok := s.byName[name]
	if !ok {
		writeText(w, http.StatusNotFound, "unknown upstream")
		return
	}
	if s.cfg.Token != "" && r.Header.Get("Authorization") != "Bearer "+s.cfg.Token {
		writeText(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	s.requests.Add(1)
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err != nil {
		writeText(w, http.StatusBadRequest, "read body failed")
		return
	}
	s.handle(w, r, up, "/"+path, body)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request, up *Upstream, path string, body []byte) {
	store := s.caches[up.Name]
	if store == nil || r.Method != http.MethodPost || isStreaming(body) {
		s.forward(w, r, up, path, body)
		return
	}

	key := cacheKey(r.Method, path, body)
	if entry, ok := store.get(key); ok {
		s.cacheHits.Add(1)
		for name, values := range entry.header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.Header().Set("X-Optimizer-Cache", "hit")
		w.WriteHeader(entry.status)
		_, _ = w.Write(entry.body)
		return
	}

	resp, respBody, err := s.roundTrip(r, up, path, body)
	if err != nil {
		s.upstreamErrs.Add(1)
		log.Printf("upstream %s %s failed: %v", up.Name, path, err)
		writeText(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	if resp.StatusCode == http.StatusOK && len(respBody) <= maxCacheableResponseBytes {
		header := make(http.Header, len(resp.Header))
		for name, values := range resp.Header {
			if hopByHop[name] || name == "Content-Length" {
				continue
			}
			header[name] = append([]string(nil), values...)
		}
		store.set(key, &cacheEntry{
			status: resp.StatusCode,
			header: header,
			body:   respBody,
			expiry: time.Now().Add(time.Duration(up.CacheTTLSeconds) * time.Second),
		})
		s.cacheStores.Add(1)
	}

	copyHeaders(w.Header(), resp.Header)
	w.Header().Del("Content-Length")
	w.Header().Set("X-Optimizer-Cache", "miss")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// roundTrip buffers the whole upstream response; only used for cacheable,
// non-streaming requests.
func (s *Server) roundTrip(r *http.Request, up *Upstream, path string, body []byte) (*http.Response, []byte, error) {
	req, err := s.buildRequest(r, up, path, body)
	if err != nil {
		return nil, nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxCacheableResponseBytes))
	if err != nil {
		return nil, nil, err
	}
	return resp, respBody, nil
}

// forward streams the upstream response straight through without buffering.
func (s *Server) forward(w http.ResponseWriter, r *http.Request, up *Upstream, path string, body []byte) {
	req, err := s.buildRequest(r, up, path, body)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "build request failed")
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.upstreamErrs.Add(1)
		log.Printf("upstream %s %s failed: %v", up.Name, path, err)
		writeText(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.Header().Del("Content-Length")
	w.Header().Set("X-Optimizer-Cache", "bypass")
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32<<10)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

func (s *Server) buildRequest(r *http.Request, up *Upstream, path string, body []byte) (*http.Request, error) {
	target := strings.TrimSuffix(up.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, r.Header)
	if up.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+up.APIKey)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	}
	return req, nil
}

// isStreaming reports whether the request body asks for a streamed response.
// Streaming requests are never cached: replaying an SSE body would have to
// rebuild chunk framing, which is a later milestone.
func isStreaming(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var probe struct {
		Stream bool `json:"stream"`
	}
	if err := common.Unmarshal(body, &probe); err != nil {
		return false
	}
	return probe.Stream
}

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, text)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	payload, err := common.Marshal(value)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "encode failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
