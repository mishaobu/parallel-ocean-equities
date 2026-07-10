package model

import "time"

const StateVersion = 5

type AnnualPoint struct {
	FiscalYear     int      `json:"fiscalYear"`
	PeriodEnd      string   `json:"periodEnd,omitempty"`
	FiledAt        string   `json:"filedAt,omitempty"`
	RevenueB       *float64 `json:"revenueB,omitempty"`
	EBITB          *float64 `json:"ebitB,omitempty"`
	DAB            *float64 `json:"daB,omitempty"`
	EBITDAB        *float64 `json:"ebitdaB,omitempty"`
	OperatingCashB *float64 `json:"operatingCashB,omitempty"`
	CapexB         *float64 `json:"capexB,omitempty"`
	FCFB           *float64 `json:"fcfB,omitempty"`
	DividendsB     *float64 `json:"dividendsB,omitempty"`
	NetIncomeB     *float64 `json:"netIncomeB,omitempty"`
	DilutedEPS     *float64 `json:"dilutedEps,omitempty"`
	DilutedSharesB *float64 `json:"dilutedSharesB,omitempty"`
	CashB          *float64 `json:"cashB,omitempty"`
	InvestmentsB   *float64 `json:"investmentsB,omitempty"`
	DebtB          *float64 `json:"debtB,omitempty"`
	NetDebtB       *float64 `json:"netDebtB,omitempty"`
	PERatio        *float64 `json:"peRatio,omitempty"`
	Estimate       bool     `json:"estimate,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
}

type PricePoint struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

type QuarterlyPoint struct {
	FiscalYear     int      `json:"fiscalYear"`
	FiscalQuarter  string   `json:"fiscalQuarter"`
	PeriodEnd      string   `json:"periodEnd"`
	FiledAt        string   `json:"filedAt,omitempty"`
	Accession      string   `json:"accession,omitempty"`
	Form           string   `json:"form,omitempty"`
	FilingURL      string   `json:"filingUrl,omitempty"`
	Derived        bool     `json:"derived,omitempty"`
	RevenueB       *float64 `json:"revenueB,omitempty"`
	EBITB          *float64 `json:"ebitB,omitempty"`
	DAB            *float64 `json:"daB,omitempty"`
	EBITDAB        *float64 `json:"ebitdaB,omitempty"`
	NetIncomeB     *float64 `json:"netIncomeB,omitempty"`
	OperatingCashB *float64 `json:"operatingCashB,omitempty"`
	CapexB         *float64 `json:"capexB,omitempty"`
	FCFB           *float64 `json:"fcfB,omitempty"`
	DividendsB     *float64 `json:"dividendsB,omitempty"`
	DilutedEPS     *float64 `json:"dilutedEps,omitempty"`
	DilutedSharesB *float64 `json:"dilutedSharesB,omitempty"`
	CashB          *float64 `json:"cashB,omitempty"`
	InvestmentsB   *float64 `json:"investmentsB,omitempty"`
	DebtB          *float64 `json:"debtB,omitempty"`
	NetDebtB       *float64 `json:"netDebtB,omitempty"`
	AssetsB        *float64 `json:"assetsB,omitempty"`
	LiabilitiesB   *float64 `json:"liabilitiesB,omitempty"`
	EquityB        *float64 `json:"equityB,omitempty"`
}

type ValuationMetrics struct {
	AsOf                   string   `json:"asOf,omitempty"`
	MarketCapB             *float64 `json:"marketCapB,omitempty"`
	EnterpriseValueB       *float64 `json:"enterpriseValueB,omitempty"`
	TTMRevenueB            *float64 `json:"ttmRevenueB,omitempty"`
	TTMEBITDAB             *float64 `json:"ttmEbitdaB,omitempty"`
	TTMEBITB               *float64 `json:"ttmEbitB,omitempty"`
	TTMFCFB                *float64 `json:"ttmFcfB,omitempty"`
	TTMNetIncomeB          *float64 `json:"ttmNetIncomeB,omitempty"`
	TTMDividendsB          *float64 `json:"ttmDividendsB,omitempty"`
	NetDebtB               *float64 `json:"netDebtB,omitempty"`
	DilutedSharesB         *float64 `json:"dilutedSharesB,omitempty"`
	PE                     *float64 `json:"pe,omitempty"`
	ForwardPE              *float64 `json:"forwardPe,omitempty"`
	EVToEBITDA             *float64 `json:"evToEbitda,omitempty"`
	ForwardEVToEBITDA      *float64 `json:"forwardEvToEbitda,omitempty"`
	EVToEBIT               *float64 `json:"evToEbit,omitempty"`
	ForwardEVToEBIT        *float64 `json:"forwardEvToEbit,omitempty"`
	FCFToMarketCap         *float64 `json:"fcfToMarketCap,omitempty"`
	ForwardFCFToMarketCap  *float64 `json:"forwardFcfToMarketCap,omitempty"`
	FCFToEV                *float64 `json:"fcfToEv,omitempty"`
	ForwardFCFToEV         *float64 `json:"forwardFcfToEv,omitempty"`
	NetDebtToEBITDA        *float64 `json:"netDebtToEbitda,omitempty"`
	ForwardNetDebtToEBITDA *float64 `json:"forwardNetDebtToEbitda,omitempty"`
	DividendToFCF          *float64 `json:"dividendToFcf,omitempty"`
	ForwardDividendToFCF   *float64 `json:"forwardDividendToFcf,omitempty"`
}

type ValuationPoint struct {
	Date                   string   `json:"date"`
	PE                     *float64 `json:"pe,omitempty"`
	ForwardPE              *float64 `json:"forwardPe,omitempty"`
	EVToEBITDA             *float64 `json:"evToEbitda,omitempty"`
	ForwardEVToEBITDA      *float64 `json:"forwardEvToEbitda,omitempty"`
	EVToEBIT               *float64 `json:"evToEbit,omitempty"`
	ForwardEVToEBIT        *float64 `json:"forwardEvToEbit,omitempty"`
	FCFToMarketCap         *float64 `json:"fcfToMarketCap,omitempty"`
	ForwardFCFToMarketCap  *float64 `json:"forwardFcfToMarketCap,omitempty"`
	FCFToEV                *float64 `json:"fcfToEv,omitempty"`
	ForwardFCFToEV         *float64 `json:"forwardFcfToEv,omitempty"`
	NetDebtToEBITDA        *float64 `json:"netDebtToEbitda,omitempty"`
	ForwardNetDebtToEBITDA *float64 `json:"forwardNetDebtToEbitda,omitempty"`
	DividendToFCF          *float64 `json:"dividendToFcf,omitempty"`
	ForwardDividendToFCF   *float64 `json:"forwardDividendToFcf,omitempty"`
}

type MacroPoint struct {
	Date                string   `json:"date"`
	Inflation           *float64 `json:"inflation,omitempty"`
	FedFunds            *float64 `json:"fedFunds,omitempty"`
	Treasury2Y          *float64 `json:"treasury2Y,omitempty"`
	Treasury10Y         *float64 `json:"treasury10Y,omitempty"`
	RealPolicyRate      *float64 `json:"realPolicyRate,omitempty"`
	Real10Y             *float64 `json:"real10Y,omitempty"`
	YieldCurve          *float64 `json:"yieldCurve,omitempty"`
	Breakeven10Y        *float64 `json:"breakeven10Y,omitempty"`
	Mortgage30Y         *float64 `json:"mortgage30Y,omitempty"`
	LogM1               *float64 `json:"logM1,omitempty"`
	LogM2               *float64 `json:"logM2,omitempty"`
	LogFedAssets        *float64 `json:"logFedAssets,omitempty"`
	LogMonetaryBase     *float64 `json:"logMonetaryBase,omitempty"`
	LogBankReserves     *float64 `json:"logBankReserves,omitempty"`
	M1Growth            *float64 `json:"m1Growth,omitempty"`
	M2Growth            *float64 `json:"m2Growth,omitempty"`
	FedAssetsGrowth     *float64 `json:"fedAssetsGrowth,omitempty"`
	MonetaryBaseGrowth  *float64 `json:"monetaryBaseGrowth,omitempty"`
	ReverseRepoB        *float64 `json:"reverseRepoB,omitempty"`
	RealGDPGrowth       *float64 `json:"realGdpGrowth,omitempty"`
	IndustrialGrowth    *float64 `json:"industrialGrowth,omitempty"`
	Unemployment        *float64 `json:"unemployment,omitempty"`
	FinancialConditions *float64 `json:"financialConditions,omitempty"`
	DollarIndex         *float64 `json:"dollarIndex,omitempty"`
	VIX                 *float64 `json:"vix,omitempty"`
	CorporateSpread     *float64 `json:"corporateSpread,omitempty"`
	HighYieldSpread     *float64 `json:"highYieldSpread,omitempty"`
	Recession           *float64 `json:"recession,omitempty"`
}

type MacroSeries struct {
	UpdatedAt time.Time    `json:"updatedAt,omitempty"`
	Sources   []string     `json:"sources,omitempty"`
	Warnings  []string     `json:"warnings,omitempty"`
	Error     string       `json:"error,omitempty"`
	Points    []MacroPoint `json:"points,omitempty"`
}

type ForecastModel struct {
	Horizon           string   `json:"horizon,omitempty"`
	Method            string   `json:"method,omitempty"`
	RevenueGrowth     *float64 `json:"revenueGrowth,omitempty"`
	EBITMargin        *float64 `json:"ebitMargin,omitempty"`
	EBITDAMargin      *float64 `json:"ebitdaMargin,omitempty"`
	FCFMargin         *float64 `json:"fcfMargin,omitempty"`
	DividendGrowth    *float64 `json:"dividendGrowth,omitempty"`
	ForwardRevenueB   *float64 `json:"forwardRevenueB,omitempty"`
	ForwardEBITB      *float64 `json:"forwardEbitB,omitempty"`
	ForwardEBITDAB    *float64 `json:"forwardEbitdaB,omitempty"`
	ForwardFCFB       *float64 `json:"forwardFcfB,omitempty"`
	ForwardNetIncomeB *float64 `json:"forwardNetIncomeB,omitempty"`
	ForwardDividendsB *float64 `json:"forwardDividendsB,omitempty"`
	ForwardEPS        *float64 `json:"forwardEps,omitempty"`
}

type ValuationModels struct {
	ProjectionYears       int      `json:"projectionYears"`
	FCFGrowth             *float64 `json:"fcfGrowth,omitempty"`
	WACC                  *float64 `json:"wacc,omitempty"`
	TerminalGrowth        *float64 `json:"terminalGrowth,omitempty"`
	DCFValuePerShare      *float64 `json:"dcfValuePerShare,omitempty"`
	TargetEVToEBITDA      *float64 `json:"targetEvToEbitda,omitempty"`
	MultipleValuePerShare *float64 `json:"multipleValuePerShare,omitempty"`
	TargetPE              *float64 `json:"targetPe,omitempty"`
	EarningsValuePerShare *float64 `json:"earningsValuePerShare,omitempty"`
}

type CurrentMetrics struct {
	Price      *float64 `json:"price,omitempty"`
	TTMEPS     *float64 `json:"ttmEps,omitempty"`
	ForwardEPS *float64 `json:"forwardEps,omitempty"`
	TrailingPE *float64 `json:"trailingPE,omitempty"`
	ForwardPE  *float64 `json:"forwardPE,omitempty"`
	Return1Y   *float64 `json:"return1Y,omitempty"`
	Low52Week  *float64 `json:"low52Week,omitempty"`
	High52Week *float64 `json:"high52Week,omitempty"`
	PriceAsOf  string   `json:"priceAsOf,omitempty"`
}

type Equity struct {
	Ticker         string           `json:"ticker"`
	Company        string           `json:"company,omitempty"`
	InstrumentType string           `json:"instrumentType,omitempty"`
	CIK            string           `json:"cik,omitempty"`
	Status         string           `json:"status"`
	Error          string           `json:"error,omitempty"`
	Warnings       []string         `json:"warnings,omitempty"`
	UpdatedAt      time.Time        `json:"updatedAt,omitempty"`
	Sources        []string         `json:"sources,omitempty"`
	Annuals        []AnnualPoint    `json:"annuals"`
	Quarterlies    []QuarterlyPoint `json:"quarterlies,omitempty"`
	Prices         []PricePoint     `json:"prices,omitempty"`
	Current        CurrentMetrics   `json:"current"`
	Valuation      ValuationMetrics `json:"valuation"`
	Forecast       ForecastModel    `json:"forecast"`
	Models         ValuationModels  `json:"models"`
	Valuations     []ValuationPoint `json:"valuations,omitempty"`
}

type State struct {
	Version   int                `json:"version"`
	UpdatedAt time.Time          `json:"updatedAt"`
	Tickers   map[string]*Equity `json:"tickers"`
	Macro     MacroSeries        `json:"macro"`
}

func NewState() State {
	return State{
		Version: StateVersion,
		Tickers: make(map[string]*Equity),
	}
}
