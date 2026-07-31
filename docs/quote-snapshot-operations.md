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
| `equities_scheduled_snapshot_benchmark_history_cache_status{benchmark,status}` | One-hot state for the single shared beta-benchmark history cache |
| `equities_scheduled_snapshot_benchmark_history_cache_timestamp_seconds{benchmark}` | Source timestamp of the shared beta-benchmark cache |
| `equities_scheduled_snapshot_benchmark_history_cache_age_seconds{benchmark}` | Age of the shared beta-benchmark cache |
| `equities_scheduled_snapshot_benchmark_history_refresh_failures_total{benchmark,reason}` | Failed shared benchmark refresh attempts by bounded reason |
| `equities_macro_last_success_timestamp_seconds`, `equities_macro_last_attempt_timestamp_seconds` | Last committed baseline and latest attempt, kept separate so failed attempts cannot make cached data appear fresh |
| `equities_macro_degraded`, `equities_macro_stale` | Latest attempt failed, and successful baseline absent or beyond the backend freshness limit, respectively |

The [PrometheusRule](../deploy/monitoring/quote-snapshot-prometheus-rule.yaml) covers scheduler absence, macro degradation with a still-usable baseline, missing or stale macro baselines, regular- and off-session polling staleness, throttling, repeated history-refresh failures, stale or unavailable history caches, low field coverage, and missing or seven-day-old observations. Regular-session staleness is explicitly filtered to `market_state="regular"`. The separate pre/post/closed/unknown alert uses the backend's 150-minute healthy-poll clock, so weekends and exchange holidays do not alert merely because no new session exists. A never-observed ticker receives a 30-minute startup grace. A seven-day-old observation becomes critical only when healthy polling has also been absent for three hours, avoiding false pages during extended but successfully polled exchange closures. An unavailable target cache warns per ticker only after current polling has succeeded; stale target caches also alert per ticker. The shared SPY cache has separate bounded metrics and one benchmark-level cache alert, preventing a single shared failure from paging once for every dependent ticker. Target and benchmark refresh failures both feed the provider-throttling and repeated-refresh-failure alerts.

## Deployment boundary

The rule is namespaced to `equities` and carries `release: kube-prometheus-stack`, which is required by the cluster's Prometheus rule selector. Keep the 30-second rule interval aligned with the existing scrape interval.

The application deployment workflow does not apply this file. The production adoption and Alertmanager route are maintained in the separate `parallel-ocean-terraform` infrastructure repository. Keep its `modules/k8s-config/equities-quote-snapshot-prometheus-rule.yaml` byte-for-byte synchronized with this source, then apply that infrastructure change before relying on alerts. Applying the rule creates alert evaluations but does not by itself deliver notifications; Alertmanager must also route `service="parallel-ocean-equities"` to the configured PagerDuty receiver. Until both resources are applied and a test alert is delivered, monitoring is not operationally complete.

## Release verification and rollback

Main-branch deployments are non-cancellable once started. The workflow records the currently deployed image, rolls out the immutable commit tag, and runs `/app/verify-startup-refresh` inside the new pod. The verifier requires every persisted ticker to be `ready` with an `updatedAt` at or after the deployment cutoff, zero fundamental refresh failures, an idle refresh queue, and a running quote scheduler.

Macro refresh is a separately degraded subsystem. The verifier requires a macro attempt from the new process. Production deploys permit that attempt to fail only when a persisted successful macro baseline remains available and non-stale; the verifier emits a warning in that case. Set `STARTUP_REFRESH_ALLOW_MACRO_DEGRADED=false` for a release that must wait for a clean macro refresh. If rollout or verification fails, the workflow restores the prior image, forces a fresh pod, and runs the same verifier against the rollback before failing the job.

Administrative mutations fail closed unless the backend receives a non-empty `ADMIN_TOKEN`. Production stores the generated token in a Kubernetes Secret. Only server-side operators and the authenticated refresh CronJob may send it as a bearer token; it must never be embedded in the public frontend or workflow logs.

The current AKS cluster reports `networkProfile.networkPolicy=none`. A Kubernetes `NetworkPolicy` object would therefore be accepted but not enforced, so this release deliberately does not claim pod-level ingress isolation. Mandatory bearer authentication is the effective protection for administrative mutations. Enabling a supported AKS network-policy engine requires a separate, planned cluster migration and node-pool reimage; add and empirically test ingress allow/deny policy only after that platform change.

Operational response is intentionally simple:

1. Check `up`, scheduler-running, and recent failure reasons before restarting anything.
2. Treat `no_new_session_total` as healthy outside an exchange session.
3. For throttling, keep concurrency at one and extend cadence/backoff before increasing provider load.
4. For low coverage, inspect the quote payload and SEC share-basis timestamp; do not substitute vendor-estimated shares.
