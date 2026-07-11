# Analyst audit remediation

This matrix tracks the 31 P0-P2 findings from the July 2026 analyst and UX audit. A finding is complete only when implementation and regression evidence are both present.

| ID | Priority | Finding | Status | Evidence |
|---:|:---:|---|:---:|---|
| 1 | P0 | Incorrect or duplicate fiscal quarter identity | Done | Fiscal-year-end derivation and consecutive-quarter validation; Go tests |
| 2 | P0 | Nondeterministic duplicate valuation dates | Done | Deterministic filing-date collapse; Go tests |
| 3 | P0 | Price-only long-horizon returns | Done | Yahoo adjusted close stored separately and used for return analysis |
| 4 | P0 | Unequal indexed comparison start dates | Done | Latest common-start rebasing in Equities and Macro; TS tests |
| 5 | P0 | Misleading country latest-observation date | Done | Oldest comparable input shown as `Common through`; TS tests |
| 6 | P0 | Overlapping forward-return observations | Done | Horizon-spaced starts in Macro and Monetary; TS tests |
| 7 | P1 | Conflicting Macro and Monetary outcome methodologies | Done | Macro Outcomes is canonical; Monetary links directly instead of publishing a revised-data duplicate |
| 8 | P1 | Invalid confidence intervals and unadjusted winner selection | Done | Newey-West HAC intervals and family-wise Holm adjustment replace naive winner inference |
| 9 | P1 | Weak calibrated scenario shown without validation | Done | 70/30 holdout gate, block-bootstrap uncertainty, and sign-stability gate |
| 10 | P1 | Pillar signal changes with chart range | Done | Fixed trailing 25-year reference; TS test |
| 11 | P1 | Headline and core inflation ambiguity | Done | Regime explicitly says headline CPI; pillar explicitly says core CPI, shelter, and wages |
| 12 | P1 | Composite cards mix release dates | Done | Each pillar reports its primary-value date and oldest component date |
| 13 | P1 | Cross-country rankings mix incomparable definitions | Done | Common historical cutoff reconstruction and comparable-current core-field filter |
| 14 | P1 | Stale countries included in current global headline | Done | Headline and rankings count only fresh core-field systems; stale systems remain in all-observed view |
| 15 | P1 | DCF defaults to unusable negative-FCF result | Done | First viable model selected; invalid models disabled |
| 16 | P1 | Price, filing, and forecast dates not co-located | Done | Matrix and model workbench co-locate price, period, filing, horizon, and method |
| 17 | P1 | No sector or peer normalization | Done | Explicit peer taxonomy, peer medians, and own-history percentiles; unknowns stay unclassified |
| 18 | P2 | Excessive default chart and DOM volume | Done | One selected chart per section; only explicitly pinned annual charts add SVGs |
| 19 | P2 | Chart ranges and ticker filters are unsynchronized | Done | Shared page-level range and ticker filter state |
| 20 | P2 | Compact charts cannot isolate tickers | Done | Shared interactive legends across compact and primary charts |
| 21 | P2 | Monetary charts lack selection and y-axis fitting | Done | Shared drag/date range selection and selected-window fitting |
| 22 | P2 | Outlier clipping is opaque and uncontrollable | Done | Visible clipped count and reversible include-all control on every auto-clipped equity chart |
| 23 | P2 | Mobile controls, tabs, and legends clip | Done | Wrapping legends, visible nav scrollbars, centered active tabs; verified at 390px across all three apps |
| 24 | P2 | Drag selection is inaccessible on touch and keyboard | Done | Synchronized date inputs and reset controls |
| 25 | P2 | Navigation and contextual deep links are inconsistent | Done | Three-way navigation and view-aware query links |
| 26 | P2 | Misleading loading and stale-data failure states | Done | Coherent loading UI and cached-state error language |
| 27 | P2 | Large APIs are fully repolled without caching | Done | Representation ETags on all state/detail APIs and view-scoped Macro payloads |
| 28 | P2 | Options analysis has no historical context | Done | Retained daily history, percentile, 1D/1W/1M changes, and SEC filing-event markers |
| 29 | P2 | Country scatter requires hover to identify points | Done | Persistent country-code labels |
| 30 | P2 | No saved universes, exports, or shareable state | Done | Named universes, pinned charts, CSV/PNG, deep links, and saved scenario comparisons |
| 31 | P2 | Unsafe ticker add/remove workflow | Done | Server-backed exact-symbol preview, exchange-qualified validation, confirmation, and undo |
