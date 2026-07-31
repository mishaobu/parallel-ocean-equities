# Parallel Ocean Equities

Chart-first equity fundamentals and valuation workspace served at `/equities`, with standalone monetary-regime and global-macro workspaces at `/monetary` and `/macro`. The repository is self-contained: three React/Vite frontends, one Go API and refresh service, seed data, container image, tests, and deployment CI.

## Data flow

- SEC Company Facts supplies annual and quarterly statements. Every normalized quarter retains its accession, filing date, form, and SEC filing link; Q4 flow values are derived from the 10-K less Q1-Q3.
- Yahoo Finance monthly closes provide split-adjusted long-history coverage.
- The ticker statistics workspace maps all 60 Yahoo key-statistics rows into a searchable catalog. Current trading fields come from a timestamped Yahoo chart snapshot; issuer fundamentals and the actual basic-share basis come from SEC Company Facts. Exact shares prefer the SEC cover-page `dei:EntityCommonStockSharesOutstanding` instant and use the non-dimensional `us-gaap:CommonStockSharesOutstanding` instant only as a guarded fallback. Company Facts omits class dimensions, so issuers that report only separate class-level cover-page facts remain explicitly unavailable until a filing-level dimensional feed is configured. Market cap is calculated as snapshot price × latest disclosed shares, with both timestamps and the exact SEC concept shown, and enterprise value is explicitly labeled as a market/filing hybrid.
- Quote-derived values seed one completed-session snapshot per historical month from the existing ten-year chart payload, then retain bounded daily live snapshots. Moving-average, volume, dividend, price, split, and rolling 60-month SPY beta histories are immediately available; exact current market-value quote snapshots accumulate only from live observations because their SEC share basis is not reconstructed in the quote archive. Filing-derived monthly series are point-in-time: a new filing is not used before its filed date.
- A traffic-independent quote scheduler samples every persisted ticker every 15 minutes in regular trading, confirms the close after 20 minutes, and polls every two hours otherwise. Requests are staggered, single-concurrency by default, and deferred while a full data refresh is busy. The latest observation replaces the ticker's earlier snapshot for the same UTC day, preserving the bounded one-snapshot-per-day history contract.
- ThetaData v3 EOD is retained as a market-data fallback when `THETA_BASE_URL` is configured. The macro options view also uses bounded, sequential ThetaData IV-history requests for selected US underlyings outside a conservative US market-hours window.
- Polygon resolves ticker CIKs when the SEC ticker map is unavailable and supplies adjusted daily bars when configured.
- FRED supplies the US macro archive plus normalized monetary histories for the United States, euro area, United Kingdom, Japan, and China. Eurostat and the ECB overlay current euro-area inflation, production, unemployment, and M3; ONS overlays current UK CPI, production, and unemployment. Country metrics retain independent observation dates, and stale or unavailable fields are explicitly warned.
- ALFRED supplies a persisted quarterly point-in-time CPI and industrial-production archive from 1994. The initial backfill is incremental: completed vintage rows are reused on later refreshes.
- The market-provider chain supplies monthly histories for regional equities, duration, credit, gold, and the dollar used by the global macro workspace.
- JSON state persists at `DATA_FILE`; Kubernetes mounts this file on a PVC.
- New tickers are analyzed asynchronously. Existing tickers refresh on `REFRESH_INTERVAL` and through the cluster CronJob.

The landing view charts indexed market performance, all eight valuation measures, and thirteen operating-quality measures on filing-date timelines with synchronized macro panels. Each ticker opens a same-page statistics explorer with search, pinned metrics, current provenance, month/quarter/year and range controls, chart/table/split views, event timelines, and URL-addressable selections. Metrics that require an unconfigured ownership, short-interest, estimates, or vendor-methodology feed remain visible with the exact source gap instead of a fabricated value. Every equities time-series chart supports drag selection, period reset, legend filtering where multiple series are present, and selected-window y-axis fitting. Isolated extreme points are clipped from the fitted axis only when adjacent observations return to the normal range; the raw point remains available in the series. The comparison response omits quarterly filing arrays, quote history, and reduces monthly prices to quarter-end snapshots. `GET /equities/api/tickers/{ticker}` returns the persisted filing archive and full monthly market history; `GET /equities/api/tickers/{ticker}/quote` returns the short-cached current snapshot and recorded quote-stat history. Calculation definitions and forward assumptions are documented in [docs/valuation-methodology.md](docs/valuation-methodology.md).

## Local run

```bash
make run
```

Open `http://localhost:8080/equities/`.

The monetary workspace is available at `http://localhost:8080/monetary/`. It uses the same persisted FRED and equity state through the equities API while keeping its own frontend bundle and route. Its views provide dated regime pillars, synchronized/pinnable chart inspection, historical episode comparison, native/change/z-score/percentile transforms, net-liquidity accounting, and release-lagged equity-regime outcomes. Historical FRED observations are latest-revised values rather than ALFRED vintages; the UI states this explicitly.

The macro workspace is available at `http://localhost:8080/macro/`. It combines sortable country regime comparisons, regional policy divergence, indexed cross-asset histories, regime-conditioned forward outcomes, options term structure and skew, return boards, and calibrated plus structural scenario modes. Drag across any time-series chart to isolate a historical period and refit its axes to the visible observations. Outcome calculations prefer the persisted ALFRED vintage available immediately before each quarterly start; they fall back to a conservative two-month lag only when the vintage archive is unavailable.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATA_FILE` | `/data/state.json` | Persistent analysis state |
| `SEED_FILE` | `/app/data/seed.json` | First-run seed state |
| `SEC_USER_AGENT` | app URL | SEC API identification |
| `FRED_USER_AGENT` | product/version URL | FRED CSV identification |
| `THETA_BASE_URL` | empty | ThetaTerminal URL, for example `http://theta-service:25503` |
| `THETA_OPTIONS_ENABLED` | `true` | Enable options enrichment when `THETA_BASE_URL` is configured |
| `OPTIONS_TICKERS` | core US list | Comma-separated options underlyings; defaults to SPY, QQQ, AMD, NVDA, MU, SMCI, DELL, and BABA |
| `ALFRED_ENABLED` | `true` | Enable incremental point-in-time US regime vintages |
| `OFFICIAL_COUNTRY_DATA_ENABLED` | `true` | Enable Eurostat, ECB, and ONS country overlays |
| `POLYGON_API_KEY` | empty | Market-data fallback |
| `STARTUP_REFRESH` | `false` | Queue a full ticker and macro refresh shortly after process startup; production enables this so schema-dependent fields are backfilled after rollout |
| `REFRESH_INTERVAL` | `24h` | In-process refresh cadence |
| `ADMIN_TOKEN` | empty (mutations disabled) | Required bearer token for ticker add/remove/refresh and `/internal/refresh` |
| `MAX_TICKERS` | `30` | Watchlist limit |
| `QUOTE_SNAPSHOT_ENABLED` | `true` | Run the traffic-independent quote snapshot scheduler |
| `QUOTE_SNAPSHOT_REGULAR_INTERVAL` | `15m` | Poll cadence while the provider reports regular trading |
| `QUOTE_SNAPSHOT_POST_CLOSE_INTERVAL` | `20m` | Confirmation delay after the first post-close observation; must be at least 15 minutes |
| `QUOTE_SNAPSHOT_CLOSED_INTERVAL` | `2h` | Poll cadence outside regular trading |
| `QUOTE_SNAPSHOT_RETRY_INTERVAL` | `5m` | Retry delay after a failed quote request |
| `QUOTE_SNAPSHOT_BUSY_INTERVAL` | `2m` | Deferral while a fundamentals refresh is active or queued |
| `QUOTE_SNAPSHOT_DISCOVERY_INTERVAL` | `1m` | Watchlist addition/removal reconciliation cadence |
| `QUOTE_SNAPSHOT_TIMEOUT` | `30s` | Per-ticker scheduler deadline, also applied to the shared live-quote provider work |
| `QUOTE_SNAPSHOT_INITIAL_DELAY` | `2m` | Delay before the first scheduled quote request after startup |
| `QUOTE_SNAPSHOT_INITIAL_STAGGER` | `5s` | Delay between initial ticker requests |
| `QUOTE_SNAPSHOT_CONCURRENCY` | `1` | Maximum concurrent scheduled quote requests |

## Verification

```bash
make test
docker build -t parallel-ocean-equities:local .
```

The production image listens on port `8080`; liveness is available at `/healthz`, readiness at `/readyz`, and Prometheus metrics at `/metrics`. The deployment workflow enforces `STARTUP_REFRESH=true` and does not succeed until every persisted ticker has completed its startup SEC/market refresh with zero failures and the quote scheduler reports running. This refresh upgrades older PVC state with newly extracted fields, including exact SEC actual-share disclosures where the issuer reports them; unsupported disclosures remain explicitly unavailable.

Snapshot freshness, coverage, provider throttling, and history-cache health are exported as bounded Prometheus series. [The snapshot operations guide](docs/quote-snapshot-operations.md) documents their semantics and the deployable [PrometheusRule](deploy/monitoring/quote-snapshot-prometheus-rule.yaml). Alertmanager notification routing is owned by the separate infrastructure repository and must be updated there before these alerts can reach PagerDuty.
