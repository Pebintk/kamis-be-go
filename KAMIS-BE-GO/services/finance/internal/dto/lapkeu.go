// Package dto holds the request and response bodies of the finance API. The
// JSON field names are the contract the Vue dashboards and the other services
// parse, so they match the Java dto classes exactly.
package dto

import "github.com/pebintk/kamis-be-go/pkg/jsontime"

// Timestamp renders dates in Asia/Jakarta, as the envelope's @JsonFormat does.
type Timestamp = jsontime.Time

// LapkeuRequest is the body of POST /api/lapkeu/add, sent by the project,
// purchase and asset services when money moves.
//
// It is the same shape as LapkeuResponse in the legacy code, which used one DTO
// for both directions.
type LapkeuRequest struct {
	ID           string     `json:"id" binding:"required"`
	ActivityType *int       `json:"activityType" binding:"required"`
	Pemasukan    *int64     `json:"pemasukan"`
	Pengeluaran  *int64     `json:"pengeluaran"`
	Description  string     `json:"description"`
	PaymentDate  *Timestamp `json:"paymentDate"`
}

// LapkeuResponse is one ledger line.
type LapkeuResponse struct {
	ID           string     `json:"id"`
	ActivityType int        `json:"activityType"`
	Pemasukan    *int64     `json:"pemasukan"`
	Pengeluaran  *int64     `json:"pengeluaran"`
	Description  string     `json:"description"`
	PaymentDate  *Timestamp `json:"paymentDate"`
}

// LapkeuSummaryResponse totals a slice of the ledger.
type LapkeuSummaryResponse struct {
	TotalTransaksi   int   `json:"totalTransaksi"`
	TotalPemasukan   int64 `json:"totalPemasukan"`
	TotalPengeluaran int64 `json:"totalPengeluaran"`
	TotalProfit      int64 `json:"totalProfit"`
}

// LapkeuPageResponse is the ledger screen: its totals and its rows in one
// response, so the page needs a single request.
type LapkeuPageResponse struct {
	Summary LapkeuSummaryResponse `json:"summary"`
	List    []LapkeuResponse      `json:"list"`
}

// ChartPengeluaranResponse is one slice of the expense breakdown.
type ChartPengeluaranResponse struct {
	ActivityType     string `json:"activityType"`
	TotalPengeluaran int64  `json:"totalPengeluaran"`
}

// IncomeExpenseResponse is one point on the income-versus-outgoings chart. The
// line and bar charts return the same shape, as they did in Java.
type IncomeExpenseResponse struct {
	Period           string `json:"period"`
	TotalPemasukan   int64  `json:"totalPemasukan"`
	TotalPengeluaran int64  `json:"totalPengeluaran"`
}

// FinancialSummaryResponse is the finance dashboard's headline panel.
type FinancialSummaryResponse struct {
	TotalIncome                 int64   `json:"totalIncome"`
	TotalIncomeFromDistribusi   int64   `json:"totalIncomeFromDistribusi"`
	TotalIncomeFromPenjualan    int64   `json:"totalIncomeFromPenjualan"`
	TotalPurchase               int64   `json:"totalPurchase"`
	TotalMaintenanceExpense     int64   `json:"totalMaintenanceExpense"`
	TotalProjectExpense         int64   `json:"totalProjectExpense"`
	TotalProfit                 int64   `json:"totalProfit"`
	TotalTransactions           int     `json:"totalTransactions"`
	TransactionPercentageChange float64 `json:"transactionPercentageChange"`
	ProfitPercentageChange      float64 `json:"profitPercentageChange"`
}

// ActivityLine is one point of another service's activity chart, as this
// service reads it back.
type ActivityLine struct {
	Period string `json:"period"`
	Count  int64  `json:"count"`
}

// ActivityComparisonResponse is one period of the operational chart, with the
// three activity streams side by side.
type ActivityComparisonResponse struct {
	Period          string `json:"period"`
	PembelianCount  int64  `json:"pembelianCount"`
	PenjualanCount  int64  `json:"penjualanCount"`
	DistribusiCount int64  `json:"distribusiCount"`
}
