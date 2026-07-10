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

## Valuation models

- DCF: five-year forward FCF projection, 9% default WACC, and 3% terminal growth.
- EV / EBITDA: forward EBITDA at an adjustable 15x default target, less net debt.
- P/E: forward diluted EPS at an adjustable 20x default target.

The model workspace keeps assumptions local to the browser and shows low, base, and high scenarios. Server-generated defaults are persisted with each ticker analysis so the initial state is reproducible.
