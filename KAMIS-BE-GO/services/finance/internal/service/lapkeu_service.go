// Package service holds the finance business logic (the Spring @Service layer).
package service

import (
	"context"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/services/finance/internal/dto"
	"github.com/karina/kamis-be-go/services/finance/internal/model"
	"github.com/karina/kamis-be-go/services/finance/internal/repository"
)

// Deps are the services the operational report reads from. The ledger itself
// needs nothing external — every other service pushes to it.
type Deps struct {
	// Project and Purchase serve the activity charts the operational report
	// combines.
	Project  *httpx.Client
	Purchase *httpx.Client
}

type LapkeuService struct {
	repo *repository.LapkeuRepository
	Deps
}

func NewLapkeuService(repo *repository.LapkeuRepository, deps Deps) *LapkeuService {
	return &LapkeuService{repo: repo, Deps: deps}
}

func response(l model.Lapkeu) dto.LapkeuResponse {
	out := dto.LapkeuResponse{
		ID:           l.ID,
		ActivityType: l.ActivityType,
		Pemasukan:    l.Pemasukan,
		Pengeluaran:  l.Pengeluaran,
		Description:  l.Description,
	}
	if l.PaymentDate != nil {
		at := jsontime.Jakarta(*l.PaymentDate)
		out.PaymentDate = &at
	}
	return out
}

func responses(rows []model.Lapkeu) []dto.LapkeuResponse {
	out := make([]dto.LapkeuResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, response(l))
	}
	return out
}

// List returns the matching ledger lines.
func (s *LapkeuService) List(ctx context.Context, f repository.Filter) ([]dto.LapkeuResponse, error) {
	rows, err := s.repo.FindFiltered(ctx, f)
	if err != nil {
		return nil, err
	}
	return responses(rows), nil
}

// Summarise totals the matching ledger lines.
func (s *LapkeuService) Summarise(ctx context.Context, f repository.Filter) (*dto.LapkeuSummaryResponse, error) {
	totals, err := s.repo.SummariseFiltered(ctx, f)
	if err != nil {
		return nil, err
	}
	return &dto.LapkeuSummaryResponse{
		TotalTransaksi:   int(totals.Count),
		TotalPemasukan:   totals.Pemasukan,
		TotalPengeluaran: totals.Pengeluaran,
		TotalProfit:      totals.Pemasukan - totals.Pengeluaran,
	}, nil
}

// Page returns the ledger screen: its totals and its rows together, so the page
// costs one request.
func (s *LapkeuService) Page(ctx context.Context, f repository.Filter) (*dto.LapkeuPageResponse, error) {
	summary, err := s.Summarise(ctx, f)
	if err != nil {
		return nil, err
	}
	list, err := s.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return &dto.LapkeuPageResponse{Summary: *summary, List: list}, nil
}

// Record writes a ledger line. It is called by the project, purchase and asset
// services when money moves, and replaces any entry already under that id so a
// retried confirmation does not double-count.
func (s *LapkeuService) Record(ctx context.Context, req dto.LapkeuRequest) (*dto.LapkeuResponse, error) {
	if _, known := model.ActivityName[*req.ActivityType]; !known {
		return nil, apierr.Invalidf("Jenis aktivitas tidak valid: %d", *req.ActivityType)
	}

	entry := model.Lapkeu{
		ID:           req.ID,
		ActivityType: *req.ActivityType,
		Pemasukan:    req.Pemasukan,
		Pengeluaran:  req.Pengeluaran,
		Description:  req.Description,
	}
	if req.PaymentDate != nil {
		at := req.PaymentDate.Time
		entry.PaymentDate = &at
	}

	if err := s.repo.Upsert(ctx, &entry); err != nil {
		return nil, err
	}
	out := response(entry)
	return &out, nil
}

// Remove deletes a ledger line, which is how a refunded project's entry is
// undone.
func (s *LapkeuService) Remove(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return notFound(err, id)
	}
	return s.repo.Delete(ctx, id)
}
