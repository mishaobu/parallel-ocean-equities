# Parallel Ocean Equities

Chart-first equity fundamentals and valuation workspace served at `/equities`. The repository is self-contained: React/Vite static UI, Go API and refresh workers, seed data, container image, tests, and deployment CI.

## Data flow

- SEC Company Facts supplies annual revenue, capex, net income, and diluted EPS.
- ThetaData v3 EOD is the preferred price-history source when `THETA_BASE_URL` is configured.
- Polygon adjusted daily aggregates are the fallback when `POLYGON_API_KEY` is configured.
- JSON state persists at `DATA_FILE`; Kubernetes mounts this file on a PVC.
- New tickers are analyzed asynchronously. Existing tickers refresh on `REFRESH_INTERVAL` and through the cluster CronJob.

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
