package analysis

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const (
	SnapshotFailureThrottled = "throttled"
	SnapshotFailureTimeout   = "timeout"
	SnapshotFailureUpstream  = "upstream"
	SnapshotFailurePersist   = "persist"
	SnapshotFailureOther     = "other"

	// The expected set is the universally applicable Yahoo quote surface:
	// current/previous/change, market metadata, 52-week and moving-average
	// fields, and volume averages. Optional beta, dividend, split, and SEC-share
	// fields are excluded so non-dividend and international equities are not
	// reported as incomplete by design.
	expectedScheduledQuoteFieldCount = 16
	benchmarkFailureDedupeLimit      = 256
)

var snapshotFailureReasons = [...]string{
	SnapshotFailureThrottled,
	SnapshotFailureTimeout,
	SnapshotFailureUpstream,
	SnapshotFailurePersist,
	SnapshotFailureOther,
}

// SnapshotObservation is the last bounded, alert-ready state recorded by the
// background quote scheduler for one tracked ticker. MarketState and failure
// kinds are normalized to fixed enumerations before they reach metrics.
type SnapshotObservation struct {
	LastAttempt                        time.Time `json:"lastAttempt,omitempty"`
	LastSuccess                        time.Time `json:"lastSuccess,omitempty"`
	LastHealthyCheck                   time.Time `json:"lastHealthyCheck,omitempty"`
	LastObservation                    time.Time `json:"lastObservation,omitempty"`
	MarketState                        string    `json:"marketState,omitempty"`
	QuoteFieldsPresent                 int       `json:"quoteFieldsPresent"`
	HistoryCacheStatus                 string    `json:"historyCacheStatus,omitempty"`
	HistoryCacheAsOf                   time.Time `json:"historyCacheAsOf,omitempty"`
	HistoryRefreshFailureKind          string    `json:"historyRefreshFailureKind,omitempty"`
	BenchmarkHistoryCacheStatus        string    `json:"benchmarkHistoryCacheStatus,omitempty"`
	BenchmarkHistoryCacheAsOf          time.Time `json:"benchmarkHistoryCacheAsOf,omitempty"`
	BenchmarkHistoryRefreshFailureKind string    `json:"benchmarkHistoryRefreshFailureKind,omitempty"`
}

type scheduledSnapshotMetrics struct {
	schedulerRunning                bool
	inflight                        int
	attempts                        int64
	successes                       int64
	noNewSession                    int64
	failures                        map[string]int64
	historyRefreshFailures          map[string]int64
	benchmarkHistoryRefreshFailures map[string]int64
	observations                    map[string]SnapshotObservation
}

func newScheduledSnapshotMetrics() scheduledSnapshotMetrics {
	return scheduledSnapshotMetrics{
		failures:                        newFailureCounts(),
		historyRefreshFailures:          newFailureCounts(),
		benchmarkHistoryRefreshFailures: newFailureCounts(),
		observations:                    make(map[string]SnapshotObservation),
	}
}

func newFailureCounts() map[string]int64 {
	counts := make(map[string]int64, len(snapshotFailureReasons))
	for _, reason := range snapshotFailureReasons {
		counts[reason] = 0
	}
	return counts
}

func cloneFailureCounts(source map[string]int64) map[string]int64 {
	counts := newFailureCounts()
	for _, reason := range snapshotFailureReasons {
		counts[reason] = source[reason]
	}
	return counts
}

// SetSnapshotSchedulerRunning lets the scheduler expose its lifecycle without
// requiring the HTTP layer to infer it from logs or scrape timing.
func (s *Service) SetSnapshotSchedulerRunning(running bool) {
	s.mu.Lock()
	s.snapshotMetrics.schedulerRunning = running
	s.mu.Unlock()
}

// ScheduledQuote executes and records one background snapshot attempt. The
// scheduler should call this instead of Quote so attempts, failures, freshness,
// coverage, and degraded history-cache use remain observable as one operation.
func (s *Service) ScheduledQuote(ctx context.Context, ticker string) (model.LiveQuote, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	_, trackedErr := s.store.Get(ticker)
	tracked := trackedErr == nil
	started := time.Now().UTC()

	s.mu.Lock()
	s.snapshotMetrics.attempts++
	s.snapshotMetrics.inflight++
	if tracked {
		observation := s.snapshotMetrics.observations[ticker]
		observation.LastAttempt = started
		s.snapshotMetrics.observations[ticker] = observation
	}
	s.mu.Unlock()

	quote, err := s.Quote(ctx, ticker)
	finished := time.Now().UTC()

	s.mu.Lock()
	s.snapshotMetrics.inflight--
	if err != nil {
		s.snapshotMetrics.failures[classifyQuoteFailure(err)]++
		s.mu.Unlock()
		return quote, err
	}

	observedAt, observedAtErr := time.Parse(time.RFC3339Nano, quote.AsOf)
	noNewSession := false
	if tracked {
		observation := s.snapshotMetrics.observations[ticker]
		observation.LastHealthyCheck = finished
		if !quoteHasProviderObservation(quote) || observedAtErr != nil || !observation.LastObservation.IsZero() && !observedAt.After(observation.LastObservation) {
			noNewSession = true
		} else {
			observation.LastSuccess = observedAt.UTC()
			observation.LastObservation = observedAt.UTC()
		}
		observation.MarketState = normalizeQuoteMarketState(quote.MarketState)
		observation.QuoteFieldsPresent = scheduledQuoteFieldCoverage(quote)
		observation.HistoryCacheStatus = normalizeHistoryCacheStatus(quote.HistoryCacheStatus)
		if cachedAt, parseErr := time.Parse(time.RFC3339Nano, quote.HistoryCacheAsOf); parseErr == nil {
			observation.HistoryCacheAsOf = cachedAt.UTC()
		} else {
			observation.HistoryCacheAsOf = time.Time{}
		}
		observation.HistoryRefreshFailureKind = normalizeFailureKind(quote.HistoryRefreshFailureKind)
		if quote.HistoryRefreshFailed && observation.HistoryRefreshFailureKind != "" {
			s.snapshotMetrics.historyRefreshFailures[observation.HistoryRefreshFailureKind]++
		}
		observation.BenchmarkHistoryCacheStatus = normalizeHistoryCacheStatus(quote.BenchmarkHistoryCacheStatus)
		if cachedAt, parseErr := time.Parse(time.RFC3339Nano, quote.BenchmarkHistoryCacheAsOf); parseErr == nil {
			observation.BenchmarkHistoryCacheAsOf = cachedAt.UTC()
		} else {
			observation.BenchmarkHistoryCacheAsOf = time.Time{}
		}
		observation.BenchmarkHistoryRefreshFailureKind = normalizeFailureKind(quote.BenchmarkHistoryRefreshFailureKind)
		if quote.BenchmarkHistoryRefreshFailed && observation.BenchmarkHistoryRefreshFailureKind != "" {
			s.recordBenchmarkHistoryFailureLocked(quote.BenchmarkHistoryRefreshFailureID, observation.BenchmarkHistoryRefreshFailureKind)
		}
		s.snapshotMetrics.observations[ticker] = observation
	}
	if noNewSession {
		s.snapshotMetrics.noNewSession++
	} else {
		s.snapshotMetrics.successes++
	}
	s.mu.Unlock()
	return quote, nil
}

// recordBenchmarkHistoryFailureLocked keeps the shared SPY counter at refresh-
// attempt semantics when multiple ticker requests coalesce on the same cache
// load. Empty IDs retain compatibility with non-Yahoo analyzers. The bounded
// window prevents untrusted/provider-generated identifiers from growing state
// without limit.
func (s *Service) recordBenchmarkHistoryFailureLocked(failureID, reason string) {
	if failureID == "" {
		s.snapshotMetrics.benchmarkHistoryRefreshFailures[reason]++
		return
	}
	if _, seen := s.benchmarkFailureIDs[failureID]; seen {
		return
	}
	s.benchmarkFailureIDs[failureID] = struct{}{}
	s.benchmarkFailureOrder = append(s.benchmarkFailureOrder, failureID)
	if len(s.benchmarkFailureOrder) > benchmarkFailureDedupeLimit {
		oldest := s.benchmarkFailureOrder[0]
		delete(s.benchmarkFailureIDs, oldest)
		s.benchmarkFailureOrder = s.benchmarkFailureOrder[1:]
	}
	s.snapshotMetrics.benchmarkHistoryRefreshFailures[reason]++
}

func (s *Service) seedScheduledSnapshotObservations() {
	state := s.store.Snapshot()
	now := time.Now().UTC()
	for ticker, equity := range state.Tickers {
		if equity == nil || len(equity.QuoteHistory) == 0 {
			continue
		}
		latestIndex := -1
		var observedAt time.Time
		for index := range equity.QuoteHistory {
			candidateAt, err := time.Parse(time.RFC3339Nano, equity.QuoteHistory[index].AsOf)
			if err != nil || candidateAt.After(now.Add(maximumQuoteFutureSkew)) {
				continue
			}
			if latestIndex < 0 || candidateAt.After(observedAt) {
				latestIndex = index
				observedAt = candidateAt
			}
		}
		if latestIndex < 0 {
			continue
		}
		latest := equity.QuoteHistory[latestIndex]
		observedAt = observedAt.UTC()
		s.snapshotMetrics.observations[ticker] = SnapshotObservation{
			LastSuccess:        observedAt,
			LastHealthyCheck:   observedAt,
			LastObservation:    observedAt,
			MarketState:        normalizeQuoteMarketState(latest.Text["market-state"]),
			QuoteFieldsPresent: scheduledSnapshotFieldCoverage(latest),
			HistoryCacheStatus: "unavailable",
		}
	}
}

func classifyQuoteFailure(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrQuotePersistence) {
		return SnapshotFailurePersist
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "http 429") || strings.Contains(message, "too many requests") || strings.Contains(message, "rate limit") || strings.Contains(message, "throttl") {
		return SnapshotFailureThrottled
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() || strings.Contains(message, "deadline exceeded") || strings.Contains(message, "timed out") || strings.Contains(message, "timeout") {
		return SnapshotFailureTimeout
	}
	if errors.Is(err, ErrQuoteUpstream) || errors.Is(err, ErrNoQuoteProvider) || strings.Contains(message, "yahoo finance") || strings.Contains(message, "provider") || strings.Contains(message, "upstream") {
		return SnapshotFailureUpstream
	}
	return SnapshotFailureOther
}

func normalizeFailureKind(kind string) string {
	for _, allowed := range snapshotFailureReasons {
		if kind == allowed {
			return kind
		}
	}
	if strings.TrimSpace(kind) != "" {
		return SnapshotFailureOther
	}
	return ""
}

func normalizeQuoteMarketState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "REGULAR", "OPEN":
		return "regular"
	case "PRE", "PREPRE":
		return "pre"
	case "POST", "POSTPOST":
		return "post"
	case "CLOSED":
		return "closed"
	default:
		return "unknown"
	}
}

func normalizeHistoryCacheStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "fresh", "stale", "unavailable":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func scheduledQuoteFieldCoverage(quote model.LiveQuote) int {
	present := 0
	for _, value := range []*float64{
		quote.Price,
		quote.PreviousClose,
		quote.Change,
		quote.ChangePercent,
		quote.Change52Week,
		quote.High52Week,
		quote.Low52Week,
		quote.MovingAverage50Day,
		quote.MovingAverage200Day,
		quote.AverageVolume3Month,
		quote.AverageVolume10Day,
	} {
		if value != nil {
			present++
		}
	}
	for _, value := range []string{quote.AsOf, quote.MarketState, quote.Currency, quote.Exchange, quote.Source} {
		if strings.TrimSpace(value) != "" {
			present++
		}
	}
	return present
}

func scheduledSnapshotFieldCoverage(snapshot model.StatisticSnapshot) int {
	present := 0
	for _, key := range []string{
		"price",
		"previous-close",
		"change",
		"change-percent",
		"change-52-week",
		"high-52-week",
		"low-52-week",
		"moving-average-50d",
		"moving-average-200d",
		"average-volume-3m",
		"average-volume-10d",
	} {
		if _, ok := snapshot.Numeric[key]; ok {
			present++
		}
	}
	for _, key := range []string{"market-state", "currency", "exchange"} {
		if strings.TrimSpace(snapshot.Text[key]) != "" {
			present++
		}
	}
	if strings.TrimSpace(snapshot.AsOf) != "" {
		present++
	}
	if strings.TrimSpace(snapshot.Source) != "" {
		present++
	}
	return present
}
