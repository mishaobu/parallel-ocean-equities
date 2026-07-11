package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	PreviewTicker(context.Context, string) (analysis.TickerPreview, error)
	DeleteTicker(string) error
	Queue(string) bool
	RefreshAll() int
}

type Config struct {
	BasePath          string
	StaticDir         string
	MonetaryPath      string
	MonetaryStaticDir string
	MacroPath         string
	MacroStaticDir    string
	RefreshToken      string
	Logger            *slog.Logger
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
	config.MonetaryPath = "/" + strings.Trim(strings.TrimSpace(config.MonetaryPath), "/")
	if config.MonetaryPath == "/" {
		config.MonetaryPath = "/monetary"
	}
	config.MacroPath = "/" + strings.Trim(strings.TrimSpace(config.MacroPath), "/")
	if config.MacroPath == "/" {
		config.MacroPath = "/macro"
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
	s.mux.HandleFunc("GET "+base+"/api/tickers/{ticker}/preview", s.handlePreviewTicker)
	s.mux.HandleFunc("GET "+base+"/api/tickers/{ticker}", s.handleTicker)
	s.mux.HandleFunc("DELETE "+base+"/api/tickers/{ticker}", s.handleDeleteTicker)
	s.mux.HandleFunc("POST "+base+"/api/tickers/{ticker}/refresh", s.handleRefreshTicker)
	s.mux.HandleFunc("GET "+base, s.handleBaseRedirect)
	s.mux.HandleFunc("GET "+base+"/", func(w http.ResponseWriter, r *http.Request) {
		s.serveStatic(w, r, base, s.config.StaticDir)
	})
	if s.config.MonetaryStaticDir != "" && s.config.MonetaryPath != base {
		monetary := s.config.MonetaryPath
		s.mux.HandleFunc("GET "+monetary+"/api/state", s.handleMonetaryState)
		s.mux.HandleFunc("GET "+monetary, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, monetary+"/", http.StatusPermanentRedirect)
		})
		s.mux.HandleFunc("GET "+monetary+"/", func(w http.ResponseWriter, r *http.Request) {
			s.serveStatic(w, r, monetary, s.config.MonetaryStaticDir)
		})
	}
	if s.config.MacroStaticDir != "" && s.config.MacroPath != base && s.config.MacroPath != s.config.MonetaryPath {
		macro := s.config.MacroPath
		s.mux.HandleFunc("GET "+macro+"/api/state", s.handleMacroState)
		s.mux.HandleFunc("GET "+macro, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, macro+"/", http.StatusPermanentRedirect)
		})
		s.mux.HandleFunc("GET "+macro+"/", func(w http.ResponseWriter, r *http.Request) {
			s.serveStatic(w, r, macro, s.config.MacroStaticDir)
		})
	}
	s.mux.HandleFunc("GET /", s.handleRoot)
}

func (s *Server) handlePreviewTicker(w http.ResponseWriter, r *http.Request) {
	preview, err := s.service.PreviewTicker(r.Context(), r.PathValue("ticker"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, http.StatusOK, preview)
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

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state := s.service.Snapshot()
	for _, equity := range state.Tickers {
		equity.Quarterlies = nil
		equity.Prices = compactPrices(equity.Prices)
	}
	state.Macro = compactMacro(state.Macro)
	writeCachedJSON(w, r, map[string]any{
		"state":   state,
		"runtime": s.service.Stats(),
	})
}

func (s *Server) handleMonetaryState(w http.ResponseWriter, r *http.Request) {
	type monetaryEquity struct {
		Ticker     string                 `json:"ticker"`
		Company    string                 `json:"company,omitempty"`
		Prices     []model.PricePoint     `json:"prices,omitempty"`
		Valuations []model.ValuationPoint `json:"valuations,omitempty"`
	}
	state := s.service.Snapshot()
	equities := make(map[string]monetaryEquity, len(state.Tickers))
	for ticker, equity := range state.Tickers {
		equities[ticker] = monetaryEquity{Ticker: equity.Ticker, Company: equity.Company, Prices: compactPrices(equity.Prices), Valuations: equity.Valuations}
	}
	macro := state.Macro
	macro.Assets = nil
	writeCachedJSON(w, r, map[string]any{
		"state": map[string]any{
			"version":   state.Version,
			"updatedAt": state.UpdatedAt,
			"tickers":   equities,
			"macro":     macro,
		},
		"runtime": s.service.Stats(),
	})
}

func (s *Server) handleMacroState(w http.ResponseWriter, r *http.Request) {
	state := s.service.Snapshot()
	state.Macro.Options.Events = optionEvents(state.Tickers)
	macro := state.Macro
	switch r.URL.Query().Get("view") {
	case "countries", "relative":
		macro.Points = nil
		macro.Assets = nil
		macro.Vintages = model.VintageSeries{}
		macro.Options = model.OptionsSeries{}
	case "assets", "overview":
		macro.Points = nil
		macro.Vintages = model.VintageSeries{}
		macro.Options = model.OptionsSeries{}
	case "options":
		macro.Points = nil
		macro.Assets = nil
		macro.Vintages = model.VintageSeries{}
	case "outcomes":
		macro.Points = nil
		macro.Options = model.OptionsSeries{}
	case "scenarios":
		macro.Vintages = model.VintageSeries{}
		macro.Options = model.OptionsSeries{}
	}
	writeCachedJSON(w, r, map[string]any{
		"state": map[string]any{
			"version":   state.Version,
			"updatedAt": state.UpdatedAt,
			"macro":     macro,
		},
		"runtime": s.service.Stats(),
	})
}

func optionEvents(equities map[string]*model.Equity) []model.OptionEvent {
	rows := make([]model.OptionEvent, 0)
	for ticker, equity := range equities {
		seen := make(map[string]bool)
		for _, quarter := range equity.Quarterlies {
			if quarter.FiledAt == "" || seen[quarter.FiledAt] {
				continue
			}
			seen[quarter.FiledAt] = true
			label := quarter.Form
			if label == "" {
				label = "SEC filing"
			}
			rows = append(rows, model.OptionEvent{Ticker: ticker, Date: quarter.FiledAt, Label: label})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Date < rows[j].Date || rows[i].Date == rows[j].Date && rows[i].Ticker < rows[j].Ticker
	})
	return rows
}

func compactPrices(rows []model.PricePoint) []model.PricePoint {
	byQuarter := make(map[string]model.PricePoint)
	for _, row := range rows {
		date, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			continue
		}
		quarter := fmt.Sprintf("%04d-Q%d", date.Year(), (int(date.Month())-1)/3+1)
		if current, exists := byQuarter[quarter]; !exists || row.Date > current.Date {
			byQuarter[quarter] = row
		}
	}
	keys := make([]string, 0, len(byQuarter))
	for key := range byQuarter {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]model.PricePoint, 0, len(keys))
	for _, key := range keys {
		out = append(out, byQuarter[key])
	}
	return out
}

func compactMacro(series model.MacroSeries) model.MacroSeries {
	points := make([]model.MacroPoint, 0, len(series.Points))
	for _, row := range series.Points {
		points = append(points, model.MacroPoint{
			Date:            row.Date,
			Inflation:       row.Inflation,
			FedFunds:        row.FedFunds,
			Treasury2Y:      row.Treasury2Y,
			Treasury10Y:     row.Treasury10Y,
			RealPolicyRate:  row.RealPolicyRate,
			YieldCurve:      row.YieldCurve,
			LogM1:           row.LogM1,
			LogM2:           row.LogM2,
			LogFedAssets:    row.LogFedAssets,
			M1Growth:        row.M1Growth,
			M2Growth:        row.M2Growth,
			CorporateSpread: row.CorporateSpread,
		})
	}
	series.Points = points
	series.Countries = nil
	series.Assets = nil
	return series
}

func (s *Server) handleTicker(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(r.PathValue("ticker"))
	equity, exists := s.service.Snapshot().Tickers[ticker]
	if !exists {
		writeError(w, http.StatusNotFound, "ticker not found")
		return
	}
	writeCachedJSON(w, r, equity)
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

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request, basePath, staticDir string) {
	relative := strings.TrimPrefix(r.URL.Path, basePath)
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" {
		relative = "index.html"
	}
	clean := filepath.Clean(relative)
	if clean == "." || strings.HasPrefix(clean, "..") {
		http.NotFound(w, r)
		return
	}
	target := filepath.Join(staticDir, clean)
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
	http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
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

func writeCachedJSON(w http.ResponseWriter, r *http.Request, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode response")
		return
	}
	etag := fmt.Sprintf("\"%x\"", sha256.Sum256(body))
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(body, '\n'))
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
