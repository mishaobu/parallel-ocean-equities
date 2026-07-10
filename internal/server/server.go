package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/analysis"
	"github.com/mishaobu/parallel-ocean-equities/internal/model"
	"github.com/mishaobu/parallel-ocean-equities/internal/store"
)

type EquityService interface {
	Snapshot() model.State
	Stats() analysis.Stats
	AddTicker(string) error
	DeleteTicker(string) error
	Queue(string) bool
	RefreshAll() int
}

type Config struct {
	BasePath     string
	StaticDir    string
	RefreshToken string
	Logger       *slog.Logger
}

type Server struct {
	service EquityService
	config  Config
	mux     *http.ServeMux
	limiter *rateLimiter
}

func New(service EquityService, config Config) *Server {
	config.BasePath = "/" + strings.Trim(strings.TrimSpace(config.BasePath), "/")
	if config.BasePath == "/" {
		config.BasePath = "/equities"
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	s := &Server{
		service: service,
		config:  config,
		mux:     http.NewServeMux(),
		limiter: newRateLimiter(10, time.Hour),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.accessLog(s.mux))
}

func (s *Server) routes() {
	base := s.config.BasePath
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET "+base+"/healthz", s.handleHealth)
	s.mux.HandleFunc("GET /metrics", s.handleMetrics)
	s.mux.HandleFunc("POST /internal/refresh", s.handleInternalRefresh)
	s.mux.HandleFunc("GET "+base+"/api/state", s.handleState)
	s.mux.HandleFunc("POST "+base+"/api/tickers", s.handleAddTicker)
	s.mux.HandleFunc("GET "+base+"/api/tickers/{ticker}", s.handleTicker)
	s.mux.HandleFunc("DELETE "+base+"/api/tickers/{ticker}", s.handleDeleteTicker)
	s.mux.HandleFunc("POST "+base+"/api/tickers/{ticker}/refresh", s.handleRefreshTicker)
	s.mux.HandleFunc("GET "+base, s.handleBaseRedirect)
	s.mux.HandleFunc("GET "+base+"/", s.handleStatic)
	s.mux.HandleFunc("GET /", s.handleRoot)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	state := s.service.Snapshot()
	ready := len(state.Tickers) > 0
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"healthy":   ready,
		"tickers":   len(state.Tickers),
		"updatedAt": state.UpdatedAt,
	})
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	state := s.service.Snapshot()
	for _, equity := range state.Tickers {
		equity.Quarterlies = nil
		equity.Prices = nil
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state":   state,
		"runtime": s.service.Stats(),
	})
}

func (s *Server) handleTicker(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(r.PathValue("ticker"))
	equity, exists := s.service.Snapshot().Tickers[ticker]
	if !exists {
		writeError(w, http.StatusNotFound, "ticker not found")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, equity)
}

func (s *Server) handleAddTicker(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.Allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "ticker-add rate limit exceeded")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var input struct {
		Ticker string `json:"ticker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.service.AddTicker(input.Ticker); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrLimit) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"ticker": strings.ToUpper(input.Ticker), "status": "queued"})
}

func (s *Server) handleDeleteTicker(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(r.PathValue("ticker"))
	if err := s.service.DeleteTicker(ticker); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefreshTicker(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(r.PathValue("ticker"))
	if _, exists := s.service.Snapshot().Tickers[ticker]; !exists {
		writeError(w, http.StatusNotFound, "ticker not found")
		return
	}
	queued := s.service.Queue(ticker)
	writeJSON(w, http.StatusAccepted, map[string]any{"ticker": ticker, "queued": queued})
}

func (s *Server) handleInternalRefresh(w http.ResponseWriter, r *http.Request) {
	if s.config.RefreshToken != "" && r.Header.Get("Authorization") != "Bearer "+s.config.RefreshToken {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"queued": s.service.RefreshAll()})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	state := s.service.Snapshot()
	stats := s.service.Stats()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP equities_watchlist_size Number of tracked tickers.\n")
	fmt.Fprintf(w, "# TYPE equities_watchlist_size gauge\n")
	fmt.Fprintf(w, "equities_watchlist_size %d\n", len(state.Tickers))
	fmt.Fprintf(w, "# HELP equities_refresh_total Completed refresh attempts.\n")
	fmt.Fprintf(w, "# TYPE equities_refresh_total counter\n")
	fmt.Fprintf(w, "equities_refresh_total %d\n", stats.RefreshTotal)
	fmt.Fprintf(w, "# HELP equities_refresh_failures_total Failed refresh attempts.\n")
	fmt.Fprintf(w, "# TYPE equities_refresh_failures_total counter\n")
	fmt.Fprintf(w, "equities_refresh_failures_total %d\n", stats.RefreshFailures)
	fmt.Fprintf(w, "# HELP equities_refresh_inflight Active or queued refresh jobs.\n")
	fmt.Fprintf(w, "# TYPE equities_refresh_inflight gauge\n")
	fmt.Fprintf(w, "equities_refresh_inflight %d\n", stats.InFlight+stats.QueueDepth)
	fmt.Fprintf(w, "# HELP equities_macro_refreshing Whether a macro refresh is active or queued.\n")
	fmt.Fprintf(w, "# TYPE equities_macro_refreshing gauge\n")
	fmt.Fprintf(w, "equities_macro_refreshing %d\n", boolGauge(stats.MacroRefreshing))
	fmt.Fprintf(w, "# HELP equities_macro_refresh_failures_total Failed macro refresh attempts.\n")
	fmt.Fprintf(w, "# TYPE equities_macro_refresh_failures_total counter\n")
	fmt.Fprintf(w, "equities_macro_refresh_failures_total %d\n", stats.MacroFailures)
}

func boolGauge(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Server) handleBaseRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.config.BasePath+"/", http.StatusPermanentRedirect)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.config.BasePath+"/", http.StatusTemporaryRedirect)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	relative := strings.TrimPrefix(r.URL.Path, s.config.BasePath)
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" {
		relative = "index.html"
	}
	clean := filepath.Clean(relative)
	if clean == "." || strings.HasPrefix(clean, "..") {
		http.NotFound(w, r)
		return
	}
	target := filepath.Join(s.config.StaticDir, clean)
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		if strings.Contains(filepath.Base(target), ".") && filepath.Base(target) != "index.html" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFile(w, r, target)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(s.config.StaticDir, "index.html"))
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.config.Logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, hits: make(map[string][]time.Time)}
}

func (l *rateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	rows := l.hits[key][:0]
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			rows = append(rows, hit)
		}
	}
	if len(rows) >= l.limit {
		l.hits[key] = rows
		return false
	}
	l.hits[key] = append(rows, now)
	return true
}

func intEnv(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
