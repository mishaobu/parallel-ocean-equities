# Parallel Ocean Equities

Chart-first equity fundamentals and valuation workspace served at `/equities`, with standalone monetary-regime and global-macro workspaces at `/monetary` and `/macro`. The repository is self-contained: three React/Vite frontends, one Go API and refresh service, seed data, container image, tests, and deployment CI.

## Data flow

- SEC Company Facts supplies annual and quarterly statements. Every normalized quarter retains its accession, filing date, form, and SEC filing link; Q4 flow values are derived from the 10-K less Q1-Q3.
- Yahoo Finance monthly closes provide split-adjusted long-history coverage.
- ThetaData v3 EOD is retained as a market-data fallback when `THETA_BASE_URL` is configured.
- Polygon resolves ticker CIKs when the SEC ticker map is unavailable and supplies adjusted daily bars when configured.
- FRED supplies the US macro archive plus normalized monetary histories for the United States, euro area, United Kingdom, Japan, and China. Country metrics retain independent observation dates because publication lags and policy definitions differ.
- The market-provider chain supplies monthly histories for regional equities, duration, credit, gold, and the dollar used by the global macro workspace.
- JSON state persists at `DATA_FILE`; Kubernetes mounts this file on a PVC.
- New tickers are analyzed asynchronously. Existing tickers refresh on `REFRESH_INTERVAL` and through the cluster CronJob.

The landing view charts indexed market performance and all seven valuation measures on a shared timeline with synchronized macro panels. The comparison response omits quarterly filing arrays and reduces monthly prices to quarter-end snapshots. `GET /equities/api/tickers/{ticker}` returns the persisted filing archive and full monthly market history. Calculation definitions and forward assumptions are documented in [docs/valuation-methodology.md](docs/valuation-methodology.md).

## Local run

```bash
make run
```

Open `http://localhost:8080/equities/`.

The monetary workspace is available at `http://localhost:8080/monetary/`. It uses the same persisted FRED and equity state through the equities API while keeping its own frontend bundle and route. Its views provide dated regime pillars, synchronized/pinnable chart inspection, historical episode comparison, native/change/z-score/percentile transforms, net-liquidity accounting, and release-lagged equity-regime outcomes. Historical FRED observations are latest-revised values rather than ALFRED vintages; the UI states this explicitly.

The macro workspace is available at `http://localhost:8080/macro/`. It combines sortable country regime comparisons, regional policy divergence, indexed cross-asset histories, return boards, and a bounded directional scenario workbench. Scenario outputs are sensitivity scores, not forecast returns.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATA_FILE` | `/data/state.json` | Persistent analysis state |
| `SEED_FILE` | `/app/data/seed.json` | First-run seed state |
| `SEC_USER_AGENT` | app URL | SEC API identification |
| `FRED_USER_AGENT` | product/version URL | FRED CSV identification |
| `THETA_BASE_URL` | empty | ThetaTerminal URL, for example `http://theta-service:25503` |
| `POLYGON_API_KEY` | empty | Market-data fallback |
| `REFRESH_INTERVAL` | `24h` | In-process refresh cadence |
| `REFRESH_TOKEN` | empty | Bearer token for `/internal/refresh` |
| `MAX_TICKERS` | `30` | Watchlist limit |

## Verification

```bash
make test
docker build -t parallel-ocean-equities:local .
```

The production image listens on port `8080`; readiness is available at `/healthz` and Prometheus metrics at `/metrics`.
