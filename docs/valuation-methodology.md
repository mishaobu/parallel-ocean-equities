# Valuation methodology

## Reported inputs

Quarterly and annual flow values come from SEC CompanyFacts duration facts. Direct quarterly facts are preferred. Cumulative 10-Q values are converted to discrete quarters, and Q4 is derived as the 10-K annual value less Q1-Q3. Balance-sheet values are matched by period end. The archive retains every unique normalized quarter and full-year period available from CompanyFacts. For historical analysis, the first filed value for a period is used so later restatements are not projected backward in time.

Core definitions:

- `EBIT = OperatingIncomeLoss`
- `EBITDA = EBIT + depreciation and amortization`
- `OCF yield = operating cash flow / market cap`
- `FCF = operating cash flow - capital expenditure`
- `Net debt = current debt + non-current debt - cash - current marketable securities`
- `Market cap = latest adjusted close x latest diluted weighted-average shares`
- `Enterprise value = market cap + net debt`

Operating-quality definitions:

- `Cash conversion = trailing operating cash flow / trailing net income`; unavailable when net income is nonpositive.
- `Gross margin = gross profit / revenue`; when gross profit is not reported directly, it is revenue less reported cost of revenue.
- `Operating, OCF, and FCF margins = the corresponding trailing value / trailing revenue`.
- `Inventory days = average inventory / trailing cost of goods sold x 365`.
- `Receivable days = average net current receivables / trailing revenue x 365`.
- `Payable days = average current payables / trailing cost of goods sold x 365`.
- `Cash conversion cycle = inventory days + receivable days - payable days`.
- `NOPAT = EBIT x (1 - cash tax rate)`; the cash tax rate is trailing tax expense / pretax income, bounded from 0% to 35%, with a 21% fallback.
- `Invested capital = debt + stockholders' equity - cash - current investments`.
- `ROIC = trailing NOPAT / average invested capital`.
- `Incremental ROIC = change in trailing NOPAT / positive change in invested capital`; unavailable when incremental capital is nonpositive.
- `Stock compensation / revenue = trailing share-based compensation expense / trailing revenue`.
- `Diluted share growth = latest diluted shares / diluted shares one year earlier - 1`.

Working-capital balances and invested capital use the average of current and one-year-prior observations when both are available. Debt is treated as zero for invested-capital calculations only when the filing provides equity and cash but no debt concept.

Trailing values sum the latest four quarters. Negative earnings and operating denominators are displayed as not meaningful where a valuation multiple would otherwise be misleading. Negative net debt remains visible as net cash.

## Model values

Model revenue uses trailing revenue multiplied by year-over-year trailing revenue growth, bounded from -20% to 40%. Model EBIT, EBITDA, operating cash flow, and FCF hold the corresponding trailing margin constant. Model dividends grow with revenue, bounded from 0% to 20%. Configured annual estimates override modeled net income and diluted EPS.

All model ratios use current market cap, enterprise value, and net debt with the modeled denominator. The current valuation table labels these values `Model`; they are internal model outputs, not analyst consensus or reported SEC facts.

## Historical valuation series

Historical ratios are dated when the corresponding filing became public and use the latest close available on or before that filing date. Quarterly history uses diluted shares, net debt, and the trailing four normalized quarters. Before four-quarter XBRL coverage begins, full-year SEC facts provide annual valuation observations. Historical P/E uses market cap divided by trailing net income so price, earnings, and shares remain on one split-adjusted basis. Clearly inconsistent SEC per-share facts are repaired from net income and the nearest plausible diluted-share observation; multiples above 200x are treated as not meaningful. The price provider is asked for history from January 1980; if the configured plan rejects that range, the service retains a nine-year fallback and records a warning.

Historical `N12M realized` points use the next four subsequently reported quarters, or the next full-year filing in the annual fallback period. They are hindsight outcomes, not estimates that were available on the historical date. Current internal model values are intentionally excluded from this series and appear only in the current valuation table and model workspace.

Operating-quality histories use the same filing-availability dates. Quarterly points use trailing four-quarter flows and current versus one-year-prior balance-sheet observations; older history falls back to annual filings. Interactive y-axes fit the selected date window. An isolated observation is excluded from the displayed axis only when it is an extreme interquartile-range outlier and both adjacent observations return inside the normal range; persistent level shifts are retained.

## Monetary context

The macro archive comes from 43 FRED series covering headline, core, PCE, shelter and wage inflation; the nominal Treasury curve; direct TIPS yields; breakevens and forward inflation; term premium; money, reserves and bank credit; Federal Reserve assets; the Treasury General Account; reverse repos; output, payrolls, claims and unemployment; lending standards; financial conditions; the dollar, volatility and credit spreads; commodities; federal debt; and NBER recessions. Required core series fail the refresh if unavailable; optional series record warnings without discarding the rest of the archive.

Net liquidity is defined as Federal Reserve assets minus the Treasury General Account minus overnight reverse repos, after normalizing all three inputs to USD billions. The dashboard presents both the stock and its year-over-year impulse. Composite regime pillars standardize multiple constituent series over the selected history instead of averaging unmatched raw levels.

The broad chart archive remains latest-revised FRED history. Separately, the service persists quarterly ALFRED snapshots of CPI and industrial production from 1994 onward. Each snapshot records the requested vintage date and the actual observation month used to calculate year-over-year inflation and production growth. Equity regime outcome tables use the latest recorded vintage available immediately before each quarterly start; if the vintage archive is unavailable they fall back to a conservative two-month lag over revised data. These remain descriptive historical comparisons rather than investable backtests.

Euro-area current readings overlay the long FRED archive with Eurostat HICP, industrial production, and unemployment plus ECB M3. UK current readings overlay ONS CPI, total production, and unemployment. Every country metric retains its own observation date. A field older than nine months is marked stale; missing international fields are reported as unavailable rather than inferred from another economy.

The options workspace uses ThetaData v3 implied-volatility history for expirations nearest 30, 60, and 90 days. Requests are sequential, limited to one completed session at one-hour intervals, and bounded around spot with `strike_range`. The service skips a conservative 13:00-22:00 UTC weekday window and preserves the prior snapshot when the terminal or a contract request is unavailable. ATM IV averages the nearest call and put mid IV; downside and upside wings use strikes nearest 95% and 105% moneyness. Model expected move is `ATM IV * sqrt(DTE / 365)`. Realized volatility is the annualized sample deviation of the latest 20 daily log returns. Options diagnostics describe market pricing, not expected returns.

## Valuation models

- DCF: five-year forward FCF projection, 9% default WACC, and 3% terminal growth.
- EV / EBITDA: forward EBITDA at an adjustable 15x default target, less net debt.
- P/E: forward diluted EPS at an adjustable 20x default target.

The model workspace keeps assumptions local to the browser and shows low, base, and high scenarios. Server-generated defaults are persisted with each ticker analysis so the initial state is reproducible.
