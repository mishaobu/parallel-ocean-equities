# Valuation methodology

## Reported inputs

Quarterly flow values come from SEC CompanyFacts duration facts. Direct quarterly facts are preferred. Cumulative 10-Q values are converted to discrete quarters, and Q4 is derived as the 10-K annual value less Q1-Q3. Balance-sheet values are matched by period end. The archive retains every unique normalized quarter available from CompanyFacts.

Core definitions:

- `EBIT = OperatingIncomeLoss`
- `EBITDA = EBIT + depreciation and amortization`
- `FCF = operating cash flow - capital expenditure`
- `Net debt = current debt + non-current debt - cash - current marketable securities`
- `Market cap = latest adjusted close x latest diluted weighted-average shares`
- `Enterprise value = market cap + net debt`

Trailing values sum the latest four quarters. Negative earnings and operating denominators are displayed as not meaningful where a valuation multiple would otherwise be misleading. Negative net debt remains visible as net cash.

## Forward values

Forward revenue uses trailing revenue multiplied by year-over-year trailing revenue growth, bounded from -20% to 40%. Forward EBIT, EBITDA, and FCF hold the corresponding trailing margin constant. Forward dividends grow with revenue, bounded from 0% to 20%. Configured annual estimates override modeled net income and diluted EPS.

All forward ratios use current market cap, enterprise value, and net debt with the forward denominator. The UI labels these values `Forward`; they are model outputs, not reported SEC facts.

## Historical valuation series

Historical ratios are calculated at each reported quarter end from the close on or before that date, diluted shares, net debt, and the trailing four normalized quarters. Historical P/E uses market cap divided by trailing net income so price, earnings, and shares remain on one split-adjusted basis. Clearly inconsistent SEC per-share facts are repaired from net income and the nearest plausible diluted-share observation; multiples above 200x are treated as not meaningful. The price provider is asked for history from January 2000; if the configured plan rejects that range, the service retains a nine-year fallback and records a warning.

Historical `Forward` points use the next four subsequently reported quarters as a realized forward proxy. The latest point uses the current forward model described above. This keeps old points reproducible without presenting modern analyst estimates as if they were available historically.

## Monetary context

Monthly macro history comes from FRED and uses `CPIAUCSL`, `FEDFUNDS`, `GS2`, `GS10`, `M1SL`, `M2SL`, `WALCL`, and `BAMLC0A0CM`. The UI charts CPI year-over-year inflation, nominal and real policy rates, 2-year and 10-year Treasury rates, the 10Y-2Y curve, M1 and M2 year-over-year growth, log10 M1 and M2, log10 Federal Reserve assets, and the US corporate option-adjusted spread on the same date domain as valuation history.

## Valuation models

- DCF: five-year forward FCF projection, 9% default WACC, and 3% terminal growth.
- EV / EBITDA: forward EBITDA at an adjustable 15x default target, less net debt.
- P/E: forward diluted EPS at an adjustable 20x default target.

The model workspace keeps assumptions local to the browser and shows low, base, and high scenarios. Server-generated defaults are persisted with each ticker analysis so the initial state is reproducible.
