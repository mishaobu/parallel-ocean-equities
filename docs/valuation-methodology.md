# Valuation methodology

## Reported inputs

Quarterly and annual flow values come from SEC CompanyFacts duration facts. Direct quarterly facts are preferred. Cumulative 10-Q values are converted to discrete quarters, and Q4 is derived as the 10-K annual value less Q1-Q3. Balance-sheet values are matched by period end. The archive retains every unique normalized quarter and full-year period available from CompanyFacts. For historical analysis, the first filed value for a period is used so later restatements are not projected backward in time.

Core definitions:

- `EBIT = OperatingIncomeLoss`
- `EBITDA = EBIT + depreciation and amortization`
- `FCF = operating cash flow - capital expenditure`
- `Net debt = current debt + non-current debt - cash - current marketable securities`
- `Market cap = latest adjusted close x latest diluted weighted-average shares`
- `Enterprise value = market cap + net debt`

Trailing values sum the latest four quarters. Negative earnings and operating denominators are displayed as not meaningful where a valuation multiple would otherwise be misleading. Negative net debt remains visible as net cash.

## Model values

Model revenue uses trailing revenue multiplied by year-over-year trailing revenue growth, bounded from -20% to 40%. Model EBIT, EBITDA, and FCF hold the corresponding trailing margin constant. Model dividends grow with revenue, bounded from 0% to 20%. Configured annual estimates override modeled net income and diluted EPS.

All model ratios use current market cap, enterprise value, and net debt with the modeled denominator. The current valuation table labels these values `Model`; they are internal model outputs, not analyst consensus or reported SEC facts.

## Historical valuation series

Historical ratios are dated when the corresponding filing became public and use the latest close available on or before that filing date. Quarterly history uses diluted shares, net debt, and the trailing four normalized quarters. Before four-quarter XBRL coverage begins, full-year SEC facts provide annual valuation observations. Historical P/E uses market cap divided by trailing net income so price, earnings, and shares remain on one split-adjusted basis. Clearly inconsistent SEC per-share facts are repaired from net income and the nearest plausible diluted-share observation; multiples above 200x are treated as not meaningful. The price provider is asked for history from January 1980; if the configured plan rejects that range, the service retains a nine-year fallback and records a warning.

Historical `N12M realized` points use the next four subsequently reported quarters, or the next full-year filing in the annual fallback period. They are hindsight outcomes, not estimates that were available on the historical date. Current internal model values are intentionally excluded from this series and appear only in the current valuation table and model workspace.

## Monetary context

Monthly macro history comes from 21 FRED series covering inflation, policy rates, Treasury and mortgage rates, inflation expectations, money and reserve aggregates, Federal Reserve assets, reverse repos, real growth, industrial production, unemployment, financial conditions, the broad dollar, volatility, credit spreads, and NBER recessions. Required core series fail the refresh if unavailable; optional series record warnings without discarding the rest of the macro archive.

## Valuation models

- DCF: five-year forward FCF projection, 9% default WACC, and 3% terminal growth.
- EV / EBITDA: forward EBITDA at an adjustable 15x default target, less net debt.
- P/E: forward diluted EPS at an adjustable 20x default target.

The model workspace keeps assumptions local to the browser and shows low, base, and high scenarios. Server-generated defaults are persisted with each ticker analysis so the initial state is reproducible.
