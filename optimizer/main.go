// Command optimizer is the Token Optimizer: an upstream-compatible passthrough
// that sits behind a New API channel. New API stays the only billing authority;
// this process only forwards, optionally replays cached responses, and records
// its own counters. It never touches New API quota.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/QuantumNous/new-api/common"
)

func main() {
	configPath := flag.String("config", "config.json", "path to the optimizer config file")
	flag.Parse()

	raw, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var cfg Config
	if err := common.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	if err := cfg.validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	server := NewServer(cfg)
	log.Printf("token optimizer listening on %s with %d upstream(s)", cfg.ListenAddr, len(cfg.Upstreams))
	if err := http.ListenAndServe(cfg.ListenAddr, server); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// Config is the whole optimizer state; there is no database.
type Config struct {
	ListenAddr string     `json:"listen_addr"`
	Token      string     `json:"token"`
	Upstreams  []Upstream `json:"upstreams"`
}

// Upstream is one provider this optimizer can reach, addressed as /u/<name>.
type Upstream struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	// CacheTTLSeconds enables exact-response caching when > 0. Off by default:
	// a cache hit replays the stored usage, so it changes nothing about billing
	// but must be proven against real traffic first.
	CacheTTLSeconds int `json:"cache_ttl_seconds"`
	MaxCacheEntries int `json:"max_cache_entries"`
}

func (c *Config) validate() error {
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	seen := make(map[string]bool, len(c.Upstreams))
	for i := range c.Upstreams {
		up := &c.Upstreams[i]
		if up.Name == "" || up.BaseURL == "" {
			return fmt.Errorf("upstream %d needs name and base_url", i)
		}
		if seen[up.Name] {
			return fmt.Errorf("duplicate upstream name %q", up.Name)
		}
		seen[up.Name] = true
		if up.MaxCacheEntries <= 0 {
			up.MaxCacheEntries = 1000
		}
	}
	return nil
}

func cacheKey(method, path string, body []byte) string {
	sum := sha256.Sum256(bytes.Join([][]byte{[]byte(method), []byte(path), body}, []byte{0}))
	return hex.EncodeToString(sum[:])
}

// hopByHop headers must not be forwarded between HTTP connections.
var hopByHop = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if hopByHop[key] || key == "Authorization" || key == "Accept-Encoding" || key == "Host" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func newUpstreamClient() *http.Client {
	// No overall timeout: streaming responses can legitimately run for minutes.
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 64,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}
