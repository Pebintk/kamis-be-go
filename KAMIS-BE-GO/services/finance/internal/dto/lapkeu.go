// Package dto holds the request and response bodies of the finance API. The
// JSON field names are the contract the Vue dashboards and the other services
// parse, so they match the Java dto classes exactly.
package dto

import "github.com/karina/kamis-be-go/pkg/jsontime"

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
