# Parallel Ocean Equities

Chart-first equity fundamentals and valuation workspace served at `/equities`. The repository is self-contained: React/Vite static UI, Go API and refresh workers, seed data, container image, tests, and deployment CI.

## Data flow

- SEC Company Facts supplies annual and quarterly statements. Every normalized quarter retains its accession, filing date, form, and SEC filing link; Q4 flow values are derived from the 10-K less Q1-Q3.
- Yahoo Finance monthly closes provide split-adjusted long-history coverage.
- ThetaData v3 EOD is retained as a market-data fallback when `THETA_BASE_URL` is configured.
- Polygon resolves ticker CIKs when the SEC ticker map is unavailable and supplies adjusted daily bars when configured.
- FRED supplies monthly monetary history: CPI, policy and Treasury rates, M1, M2, Federal Reserve assets, and corporate option-adjusted spread.
- JSON state persists at `DATA_FILE`; Kubernetes mounts this file on a PVC.
- New tickers are analyzed asynchronously. Existing tickers refresh on `REFRESH_INTERVAL` and through the cluster CronJob.

The landing view charts all seven valuation measures on a shared 2000-present timeline with synchronized macro panels. The comparison response omits raw quarterly and price arrays to keep polling small. `GET /equities/api/tickers/{ticker}` returns the persisted filing archive and detailed statement history. Calculation definitions and forward assumptions are documented in [docs/valuation-methodology.md](docs/valuation-methodology.md).

## Local run

```bash
make run
```

Open `http://localhost:8080/equities/`.

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
