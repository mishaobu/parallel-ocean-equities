package model

import "time"

const StateVersion = 10

type AnnualPoint struct {
	FiscalYear     int      `json:"fiscalYear"`
	PeriodEnd      string   `json:"periodEnd,omitempty"`
	FiledAt        string   `json:"filedAt,omitempty"`
	RevenueB       *float64 `json:"revenueB,omitempty"`
	GrossProfitB   *float64 `json:"grossProfitB,omitempty"`
	EBITB          *float64 `json:"ebitB,omitempty"`
	DAB            *float64 `json:"daB,omitempty"`
	EBITDAB        *float64 `json:"ebitdaB,omitempty"`
	OperatingCashB *float64 `json:"operatingCashB,omitempty"`
	CapexB         *float64 `json:"capexB,omitempty"`
	FCFB           *float64 `json:"fcfB,omitempty"`
	DividendsB     *float64 `json:"dividendsB,omitempty"`
	NetIncomeB     *float64 `json:"netIncomeB,omitempty"`
	PretaxIncomeB  *float64 `json:"pretaxIncomeB,omitempty"`
	IncomeTaxB     *float64 `json:"incomeTaxB,omitempty"`
	StockCompB     *float64 `json:"stockCompB,omitempty"`
	DilutedEPS     *float64 `json:"dilutedEps,omitempty"`
	DilutedSharesB *float64 `json:"dilutedSharesB,omitempty"`
	CashB          *float64 `json:"cashB,omitempty"`
	InvestmentsB   *float64 `json:"investmentsB,omitempty"`
	DebtB          *float64 `json:"debtB,omitempty"`
	NetDebtB       *float64 `json:"netDebtB,omitempty"`
	InventoryB     *float64 `json:"inventoryB,omitempty"`
	ReceivablesB   *float64 `json:"receivablesB,omitempty"`
	PayablesB      *float64 `json:"payablesB,omitempty"`
	AssetsB        *float64 `json:"assetsB,omitempty"`
	LiabilitiesB   *float64 `json:"liabilitiesB,omitempty"`
	EquityB        *float64 `json:"equityB,omitempty"`
	PERatio        *float64 `json:"peRatio,omitempty"`
	Estimate       bool     `json:"estimate,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
}

type PricePoint struct {
	Date             string   `json:"date"`
	Close            float64  `json:"close"`
	TotalReturnClose *float64 `json:"totalReturnClose,omitempty"`
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
	GrossProfitB   *float64 `json:"grossProfitB,omitempty"`
	EBITB          *float64 `json:"ebitB,omitempty"`
	DAB            *float64 `json:"daB,omitempty"`
	EBITDAB        *float64 `json:"ebitdaB,omitempty"`
	NetIncomeB     *float64 `json:"netIncomeB,omitempty"`
	PretaxIncomeB  *float64 `json:"pretaxIncomeB,omitempty"`
	IncomeTaxB     *float64 `json:"incomeTaxB,omitempty"`
	StockCompB     *float64 `json:"stockCompB,omitempty"`
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
	InventoryB     *float64 `json:"inventoryB,omitempty"`
	ReceivablesB   *float64 `json:"receivablesB,omitempty"`
	PayablesB      *float64 `json:"payablesB,omitempty"`
	AssetsB        *float64 `json:"assetsB,omitempty"`
	LiabilitiesB   *float64 `json:"liabilitiesB,omitempty"`
	EquityB        *float64 `json:"equityB,omitempty"`
}

type ValuationMetrics struct {
	AsOf                            string   `json:"asOf,omitempty"`
	MarketCapB                      *float64 `json:"marketCapB,omitempty"`
	EnterpriseValueB                *float64 `json:"enterpriseValueB,omitempty"`
	TTMRevenueB                     *float64 `json:"ttmRevenueB,omitempty"`
	TTMEBITDAB                      *float64 `json:"ttmEbitdaB,omitempty"`
	TTMEBITB                        *float64 `json:"ttmEbitB,omitempty"`
	TTMOperatingCashB               *float64 `json:"ttmOperatingCashB,omitempty"`
	TTMFCFB                         *float64 `json:"ttmFcfB,omitempty"`
	TTMNetIncomeB                   *float64 `json:"ttmNetIncomeB,omitempty"`
	TTMDividendsB                   *float64 `json:"ttmDividendsB,omitempty"`
	NetDebtB                        *float64 `json:"netDebtB,omitempty"`
	DilutedSharesB                  *float64 `json:"dilutedSharesB,omitempty"`
	PE                              *float64 `json:"pe,omitempty"`
	ForwardPE                       *float64 `json:"forwardPe,omitempty"`
	EVToEBITDA                      *float64 `json:"evToEbitda,omitempty"`
	ForwardEVToEBITDA               *float64 `json:"forwardEvToEbitda,omitempty"`
	EVToEBIT                        *float64 `json:"evToEbit,omitempty"`
	ForwardEVToEBIT                 *float64 `json:"forwardEvToEbit,omitempty"`
	OperatingCashToMarketCap        *float64 `json:"operatingCashToMarketCap,omitempty"`
	ForwardOperatingCashToMarketCap *float64 `json:"forwardOperatingCashToMarketCap,omitempty"`
	FCFToMarketCap                  *float64 `json:"fcfToMarketCap,omitempty"`
	ForwardFCFToMarketCap           *float64 `json:"forwardFcfToMarketCap,omitempty"`
	FCFToEV                         *float64 `json:"fcfToEv,omitempty"`
	ForwardFCFToEV                  *float64 `json:"forwardFcfToEv,omitempty"`
	NetDebtToEBITDA                 *float64 `json:"netDebtToEbitda,omitempty"`
	ForwardNetDebtToEBITDA          *float64 `json:"forwardNetDebtToEbitda,omitempty"`
	DividendToFCF                   *float64 `json:"dividendToFcf,omitempty"`
	ForwardDividendToFCF            *float64 `json:"forwardDividendToFcf,omitempty"`
}

type ValuationPoint struct {
	Date                            string   `json:"date"`
	PE                              *float64 `json:"pe,omitempty"`
	ForwardPE                       *float64 `json:"forwardPe,omitempty"`
	EVToEBITDA                      *float64 `json:"evToEbitda,omitempty"`
	ForwardEVToEBITDA               *float64 `json:"forwardEvToEbitda,omitempty"`
	EVToEBIT                        *float64 `json:"evToEbit,omitempty"`
	ForwardEVToEBIT                 *float64 `json:"forwardEvToEbit,omitempty"`
	OperatingCashToMarketCap        *float64 `json:"operatingCashToMarketCap,omitempty"`
	ForwardOperatingCashToMarketCap *float64 `json:"forwardOperatingCashToMarketCap,omitempty"`
	FCFToMarketCap                  *float64 `json:"fcfToMarketCap,omitempty"`
	ForwardFCFToMarketCap           *float64 `json:"forwardFcfToMarketCap,omitempty"`
	FCFToEV                         *float64 `json:"fcfToEv,omitempty"`
	ForwardFCFToEV                  *float64 `json:"forwardFcfToEv,omitempty"`
	NetDebtToEBITDA                 *float64 `json:"netDebtToEbitda,omitempty"`
	ForwardNetDebtToEBITDA          *float64 `json:"forwardNetDebtToEbitda,omitempty"`
	DividendToFCF                   *float64 `json:"dividendToFcf,omitempty"`
	ForwardDividendToFCF            *float64 `json:"forwardDividendToFcf,omitempty"`
}

type QualityMetrics struct {
	AsOf                    string   `json:"asOf,omitempty"`
	CashConversion          *float64 `json:"cashConversion,omitempty"`
	GrossMargin             *float64 `json:"grossMargin,omitempty"`
	OperatingMargin         *float64 `json:"operatingMargin,omitempty"`
	OperatingCashMargin     *float64 `json:"operatingCashMargin,omitempty"`
	FCFMargin               *float64 `json:"fcfMargin,omitempty"`
	InventoryDays           *float64 `json:"inventoryDays,omitempty"`
	ReceivableDays          *float64 `json:"receivableDays,omitempty"`
	PayableDays             *float64 `json:"payableDays,omitempty"`
	CashConversionCycleDays *float64 `json:"cashConversionCycleDays,omitempty"`
	ROIC                    *float64 `json:"roic,omitempty"`
	IncrementalROIC         *float64 `json:"incrementalRoic,omitempty"`
	StockCompToRevenue      *float64 `json:"stockCompToRevenue,omitempty"`
	DilutedShareGrowth      *float64 `json:"dilutedShareGrowth,omitempty"`
}

type QualityPoint struct {
	Date                    string   `json:"date"`
	CashConversion          *float64 `json:"cashConversion,omitempty"`
	GrossMargin             *float64 `json:"grossMargin,omitempty"`
	OperatingMargin         *float64 `json:"operatingMargin,omitempty"`
	OperatingCashMargin     *float64 `json:"operatingCashMargin,omitempty"`
	FCFMargin               *float64 `json:"fcfMargin,omitempty"`
	InventoryDays           *float64 `json:"inventoryDays,omitempty"`
	ReceivableDays          *float64 `json:"receivableDays,omitempty"`
	PayableDays             *float64 `json:"payableDays,omitempty"`
	CashConversionCycleDays *float64 `json:"cashConversionCycleDays,omitempty"`
	ROIC                    *float64 `json:"roic,omitempty"`
	IncrementalROIC         *float64 `json:"incrementalRoic,omitempty"`
	StockCompToRevenue      *float64 `json:"stockCompToRevenue,omitempty"`
	DilutedShareGrowth      *float64 `json:"dilutedShareGrowth,omitempty"`
}

type MacroPoint struct {
	Date                string   `json:"date"`
	Inflation           *float64 `json:"inflation,omitempty"`
	CoreInflation       *float64 `json:"coreInflation,omitempty"`
	CorePCEInflation    *float64 `json:"corePceInflation,omitempty"`
	ShelterInflation    *float64 `json:"shelterInflation,omitempty"`
	WageGrowth          *float64 `json:"wageGrowth,omitempty"`
	FedFunds            *float64 `json:"fedFunds,omitempty"`
	Treasury3M          *float64 `json:"treasury3M,omitempty"`
	Treasury2Y          *float64 `json:"treasury2Y,omitempty"`
	Treasury5Y          *float64 `json:"treasury5Y,omitempty"`
	Treasury10Y         *float64 `json:"treasury10Y,omitempty"`
	Treasury30Y         *float64 `json:"treasury30Y,omitempty"`
	RealPolicyRate      *float64 `json:"realPolicyRate,omitempty"`
	Real5Y              *float64 `json:"real5Y,omitempty"`
	Real10Y             *float64 `json:"real10Y,omitempty"`
	YieldCurve          *float64 `json:"yieldCurve,omitempty"`
	YieldCurve3M        *float64 `json:"yieldCurve3M,omitempty"`
	Breakeven5Y         *float64 `json:"breakeven5Y,omitempty"`
	Breakeven10Y        *float64 `json:"breakeven10Y,omitempty"`
	ForwardInflation5Y  *float64 `json:"forwardInflation5Y,omitempty"`
	TermPremium10Y      *float64 `json:"termPremium10Y,omitempty"`
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
	TgaB                *float64 `json:"tgaB,omitempty"`
	ReverseRepoB        *float64 `json:"reverseRepoB,omitempty"`
	NetLiquidityB       *float64 `json:"netLiquidityB,omitempty"`
	NetLiquidityGrowth  *float64 `json:"netLiquidityGrowth,omitempty"`
	BankCreditGrowth    *float64 `json:"bankCreditGrowth,omitempty"`
	BusinessLoanGrowth  *float64 `json:"businessLoanGrowth,omitempty"`
	RealGDPGrowth       *float64 `json:"realGdpGrowth,omitempty"`
	IndustrialGrowth    *float64 `json:"industrialGrowth,omitempty"`
	PayrollGrowth       *float64 `json:"payrollGrowth,omitempty"`
	InitialClaimsK      *float64 `json:"initialClaimsK,omitempty"`
	Unemployment        *float64 `json:"unemployment,omitempty"`
	SahmRule            *float64 `json:"sahmRule,omitempty"`
	FinancialConditions *float64 `json:"financialConditions,omitempty"`
	LendingStandards    *float64 `json:"lendingStandards,omitempty"`
	DollarIndex         *float64 `json:"dollarIndex,omitempty"`
	VIX                 *float64 `json:"vix,omitempty"`
	CorporateSpread     *float64 `json:"corporateSpread,omitempty"`
	HighYieldSpread     *float64 `json:"highYieldSpread,omitempty"`
	OilPrice            *float64 `json:"oilPrice,omitempty"`
	CopperPrice         *float64 `json:"copperPrice,omitempty"`
	FederalDebtToGDP    *float64 `json:"federalDebtToGdp,omitempty"`
	Recession           *float64 `json:"recession,omitempty"`
}

type MacroSeries struct {
	UpdatedAt time.Time       `json:"updatedAt,omitempty"`
	Sources   []string        `json:"sources,omitempty"`
	Warnings  []string        `json:"warnings,omitempty"`
	Error     string          `json:"error,omitempty"`
	Basis     string          `json:"basis,omitempty"`
	Points    []MacroPoint    `json:"points,omitempty"`
	Countries []CountrySeries `json:"countries,omitempty"`
	Assets    []AssetSeries   `json:"assets,omitempty"`
	Vintages  VintageSeries   `json:"vintages,omitempty"`
	Options   OptionsSeries   `json:"options,omitempty"`
}

type VintagePoint struct {
	Date                      string   `json:"date"`
	VintageDate               string   `json:"vintageDate"`
	Inflation                 *float64 `json:"inflation,omitempty"`
	InflationObservationDate  string   `json:"inflationObservationDate,omitempty"`
	IndustrialGrowth          *float64 `json:"industrialGrowth,omitempty"`
	IndustrialObservationDate string   `json:"industrialObservationDate,omitempty"`
}

type VintageSeries struct {
	UpdatedAt time.Time      `json:"updatedAt,omitempty"`
	Source    string         `json:"source,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	Points    []VintagePoint `json:"points,omitempty"`
}

type OptionTermPoint struct {
	Expiration       string   `json:"expiration"`
	DaysToExpiration int      `json:"daysToExpiration"`
	Spot             *float64 `json:"spot,omitempty"`
	ATMIV            *float64 `json:"atmIv,omitempty"`
	PutWingIV        *float64 `json:"putWingIv,omitempty"`
	CallWingIV       *float64 `json:"callWingIv,omitempty"`
	Skew             *float64 `json:"skew,omitempty"`
	ExpectedMove     *float64 `json:"expectedMove,omitempty"`
	StraddleMove     *float64 `json:"straddleMove,omitempty"`
}

type OptionSnapshot struct {
	Ticker                string            `json:"ticker"`
	AsOf                  string            `json:"asOf,omitempty"`
	Spot                  *float64          `json:"spot,omitempty"`
	RealizedVolatility20D *float64          `json:"realizedVolatility20D,omitempty"`
	ATMIV30D              *float64          `json:"atmIv30D,omitempty"`
	Skew30D               *float64          `json:"skew30D,omitempty"`
	ExpectedMove30D       *float64          `json:"expectedMove30D,omitempty"`
	ImpliedRealizedSpread *float64          `json:"impliedRealizedSpread,omitempty"`
	Terms                 []OptionTermPoint `json:"terms,omitempty"`
}

type OptionHistoryPoint struct {
	Ticker                string   `json:"ticker"`
	Date                  string   `json:"date"`
	Spot                  *float64 `json:"spot,omitempty"`
	RealizedVolatility20D *float64 `json:"realizedVolatility20D,omitempty"`
	ATMIV30D              *float64 `json:"atmIv30D,omitempty"`
	Skew30D               *float64 `json:"skew30D,omitempty"`
	ExpectedMove30D       *float64 `json:"expectedMove30D,omitempty"`
	ImpliedRealizedSpread *float64 `json:"impliedRealizedSpread,omitempty"`
}

type OptionEvent struct {
	Ticker string `json:"ticker"`
	Date   string `json:"date"`
	Label  string `json:"label"`
}

type OptionsSeries struct {
	UpdatedAt time.Time            `json:"updatedAt,omitempty"`
	AsOf      string               `json:"asOf,omitempty"`
	Source    string               `json:"source,omitempty"`
	Warnings  []string             `json:"warnings,omitempty"`
	Snapshots []OptionSnapshot     `json:"snapshots,omitempty"`
	History   []OptionHistoryPoint `json:"history,omitempty"`
	Events    []OptionEvent        `json:"events,omitempty"`
}

type CountryPoint struct {
	Date              string   `json:"date"`
	PolicyRate        *float64 `json:"policyRate,omitempty"`
	PolicyRateDate    string   `json:"policyRateDate,omitempty"`
	Inflation         *float64 `json:"inflation,omitempty"`
	InflationDate     string   `json:"inflationDate,omitempty"`
	CoreInflation     *float64 `json:"coreInflation,omitempty"`
	CoreInflationDate string   `json:"coreInflationDate,omitempty"`
	IndustrialGrowth  *float64 `json:"industrialGrowth,omitempty"`
	IndustrialDate    string   `json:"industrialDate,omitempty"`
	Unemployment      *float64 `json:"unemployment,omitempty"`
	UnemploymentDate  string   `json:"unemploymentDate,omitempty"`
	MoneyGrowth       *float64 `json:"moneyGrowth,omitempty"`
	MoneyGrowthDate   string   `json:"moneyGrowthDate,omitempty"`
	LongRate          *float64 `json:"longRate,omitempty"`
	LongRateDate      string   `json:"longRateDate,omitempty"`
	RealRate          *float64 `json:"realRate,omitempty"`
	YieldCurve        *float64 `json:"yieldCurve,omitempty"`
	FX                *float64 `json:"fx,omitempty"`
	FXDate            string   `json:"fxDate,omitempty"`
	LeadingIndex      *float64 `json:"leadingIndex,omitempty"`
	LeadingIndexDate  string   `json:"leadingIndexDate,omitempty"`
}

type CountrySeries struct {
	Code         string         `json:"code"`
	Name         string         `json:"name"`
	Currency     string         `json:"currency"`
	Region       string         `json:"region"`
	PolicyLabel  string         `json:"policyLabel"`
	FXLabel      string         `json:"fxLabel"`
	EquityTicker string         `json:"equityTicker,omitempty"`
	Sources      []string       `json:"sources,omitempty"`
	Warnings     []string       `json:"warnings,omitempty"`
	Points       []CountryPoint `json:"points,omitempty"`
}

type AssetSeries struct {
	Symbol string       `json:"symbol"`
	Label  string       `json:"label"`
	Group  string       `json:"group"`
	Region string       `json:"region,omitempty"`
	Source string       `json:"source,omitempty"`
	Points []PricePoint `json:"points,omitempty"`
}

type ForecastModel struct {
	Horizon               string   `json:"horizon,omitempty"`
	Method                string   `json:"method,omitempty"`
	RevenueGrowth         *float64 `json:"revenueGrowth,omitempty"`
	EBITMargin            *float64 `json:"ebitMargin,omitempty"`
	EBITDAMargin          *float64 `json:"ebitdaMargin,omitempty"`
	OperatingCashMargin   *float64 `json:"operatingCashMargin,omitempty"`
	FCFMargin             *float64 `json:"fcfMargin,omitempty"`
	DividendGrowth        *float64 `json:"dividendGrowth,omitempty"`
	ForwardRevenueB       *float64 `json:"forwardRevenueB,omitempty"`
	ForwardEBITB          *float64 `json:"forwardEbitB,omitempty"`
	ForwardEBITDAB        *float64 `json:"forwardEbitdaB,omitempty"`
	ForwardOperatingCashB *float64 `json:"forwardOperatingCashB,omitempty"`
	ForwardFCFB           *float64 `json:"forwardFcfB,omitempty"`
	ForwardNetIncomeB     *float64 `json:"forwardNetIncomeB,omitempty"`
	ForwardDividendsB     *float64 `json:"forwardDividendsB,omitempty"`
	ForwardEPS            *float64 `json:"forwardEps,omitempty"`
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
	Quality        QualityMetrics   `json:"quality"`
	Qualities      []QualityPoint   `json:"qualities,omitempty"`
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
