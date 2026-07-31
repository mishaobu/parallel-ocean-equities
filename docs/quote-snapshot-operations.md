# Quote snapshot operations

The in-process scheduler captures every persisted ticker independently of page traffic. Its defaults are deliberately conservative: one request at a time, initial staggering, 15-minute regular-session polls, a delayed close confirmation, two-hour closed-session polls, and five-minute retries. Full ticker and macro refreshes take priority. Failed ten-year history loads back off for 30 minutes whether or not stale data is available, while the lightweight current quote remains available. Persistence retains the latest observation per ticker and UTC day, so frequent polling improves current accuracy without unbounded history growth.

## Prometheus contract

The existing `ServiceMonitor` scrapes `/metrics` every 30 seconds. Scheduler metrics use bounded ticker, market-state, cache-status, and failure-reason labels.

| Signal | Meaning |
| --- | --- |
| `equities_scheduled_snapshot_scheduler_running` | Scheduler lifecycle gauge; production must report `1` |
| `equities_scheduled_snapshot_attempts_total`, `successes_total`, `no_new_session_total` | Poll outcomes; an unchanged provider session is healthy, not a failure |
| `equities_scheduled_snapshot_failures_total{reason}` | Failed current-quote requests: `throttled`, `timeout`, `upstream`, `persist`, or `other` |
| `equities_scheduled_snapshot_last_observation_timestamp_seconds{ticker}` | Timestamp supplied by the quote provider |
| `equities_scheduled_snapshot_last_healthy_check_timestamp_seconds{ticker}` | Latest successful poll, including an unchanged closed session |
| `equities_scheduled_snapshot_stale{ticker,market_state}` | Backend-computed staleness: 20 minutes in regular trading, 150 minutes otherwise |
| `equities_scheduled_snapshot_quote_field_coverage_ratio{ticker}` | Present core quote fields divided by the backend-defined expected count |
| `equities_scheduled_snapshot_history_cache_status{ticker,status}` | One-hot `fresh`, `stale`, or `unavailable` long-history cache state |
| `equities_scheduled_snapshot_history_cache_age_seconds{ticker}` | Age of the long-history payload used by the latest quote |
| `equities_scheduled_snapshot_history_refresh_failures_total{reason}` | Actual failed history refresh attempts; stale-cache reuse during backoff does not repeatedly increment it |

The [PrometheusRule](../deploy/monitoring/quote-snapshot-prometheus-rule.yaml) covers scheduler absence, regular- and off-session polling staleness, throttling, repeated history-refresh failures, stale history cache, low field coverage, and missing or seven-day-old observations. Regular-session staleness is explicitly filtered to `market_state="regular"`. The separate pre/post/closed/unknown alert uses the backend's 150-minute healthy-poll clock, so weekends and exchange holidays do not alert merely because no new session exists. A never-observed ticker receives the same 30-minute alert grace as a very old observation.

## Deployment boundary

The rule is namespaced to `equities` and carries `release: kube-prometheus-stack`, which is required by the cluster's Prometheus rule selector. Keep the 30-second rule interval aligned with the existing scrape interval.

The application deployment workflow does not apply this file. Have the platform owner adopt it into the monitoring source of truth before applying it to the cluster. Applying the rule creates alert evaluations but does not by itself deliver notifications. Alertmanager routing and its PagerDuty receiver are managed in the separate `parallel-ocean-terraform` infrastructure repository. That route must explicitly match `service="parallel-ocean-equities"` before these alerts can page; until then, unmatched alerts follow the cluster's default null receiver.

Operational response is intentionally simple:

1. Check `up`, scheduler-running, and recent failure reasons before restarting anything.
2. Treat `no_new_session_total` as healthy outside an exchange session.
3. For throttling, keep concurrency at one and extend cadence/backoff before increasing provider load.
4. For low coverage, inspect the quote payload and SEC share-basis timestamp; do not substitute vendor-estimated shares.
