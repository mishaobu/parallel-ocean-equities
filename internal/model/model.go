package model

import "time"

const StateVersion = 1

type AnnualPoint struct {
	FiscalYear int      `json:"fiscalYear"`
	PeriodEnd  string   `json:"periodEnd,omitempty"`
	RevenueB   *float64 `json:"revenueB,omitempty"`
	CapexB     *float64 `json:"capexB,omitempty"`
	NetIncomeB *float64 `json:"netIncomeB,omitempty"`
	DilutedEPS *float64 `json:"dilutedEps,omitempty"`
	PERatio    *float64 `json:"peRatio,omitempty"`
	Estimate   bool     `json:"estimate,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
}

type PricePoint struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
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
	Ticker    string         `json:"ticker"`
	Company   string         `json:"company,omitempty"`
	CIK       string         `json:"cik,omitempty"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt,omitempty"`
	Sources   []string       `json:"sources,omitempty"`
	Annuals   []AnnualPoint  `json:"annuals"`
	Prices    []PricePoint   `json:"prices,omitempty"`
	Current   CurrentMetrics `json:"current"`
}

type State struct {
	Version   int                `json:"version"`
	UpdatedAt time.Time          `json:"updatedAt"`
	Tickers   map[string]*Equity `json:"tickers"`
}

func NewState() State {
	return State{
		Version: StateVersion,
		Tickers: make(map[string]*Equity),
	}
}
