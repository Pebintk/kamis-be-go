// Package service holds the project business logic (the Spring @Service /
// restservice layer).
package service

import (
	"context"
	"time"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/services/project/internal/dto"
	"github.com/karina/kamis-be-go/services/project/internal/model"
	"github.com/karina/kamis-be-go/services/project/internal/repository"
)

type ProjectService struct {
	repo *repository.ProjectRepository
	Deps
}

func NewProjectService(repo *repository.ProjectRepository, deps Deps) *ProjectService {
	return &ProjectService{repo: repo, Deps: deps}
}

// username is the token subject, which is what the audit log records.
func username(ctx context.Context) string {
	if claims, ok := auth.FromContext(ctx); ok {
		return claims.Subject
	}
	return ""
}

// stamp renders an optional time for a DTO.
func stamp(t *time.Time) *dto.Timestamp {
	if t == nil {
		return nil
	}
	at := jsontime.Jakarta(*t)
	return &at
}

// ---- reads ----

// ListProjects returns every matching project.
func (s *ProjectService) ListProjects(ctx context.Context, f repository.Filter) ([]dto.ProjectListResponse, error) {
	projects, err := s.repo.FindFiltered(ctx, f)
	if err != nil {
		return nil, err
	}
	return rows(projects), nil
}

// ListProjectsPaginated returns one page of matching projects.
func (s *ProjectService) ListProjectsPaginated(ctx context.Context, f repository.Filter, number, size int) (dto.PageOf[dto.ProjectListResponse], error) {
	var empty dto.PageOf[dto.ProjectListResponse]

	projects, total, err := s.repo.FindFilteredPaginated(ctx, f, number, size)
	if err != nil {
		return empty, err
	}
	return dto.NewPage(rows(projects), number, size, total), nil
}

// rows builds the flat list representation, which is the same across both kinds
// of project.
func rows(projects []model.Project) []dto.ProjectListResponse {
	out := make([]dto.ProjectListResponse, 0, len(projects))
	for _, p := range projects {
		out = append(out, dto.ProjectListResponse{
			ID:                      p.ID,
			ProjectType:             p.ProjectType,
			ProjectStatus:           p.ProjectStatus,
			ProjectPaymentStatus:    p.ProjectPaymentStatus,
			ProjectName:             p.ProjectName,
			ProjectDescription:      p.ProjectDescription,
			ProjectClientID:         p.ProjectClientID,
			ProjectClientName:       p.ProjectClientName,
			ProjectStartDate:        stamp(p.ProjectStartDate),
			ProjectEndDate:          stamp(p.ProjectEndDate),
			ProjectTotalPemasukkan:  p.ProjectTotalPemasukkan,
			ProjectTotalPengeluaran: p.ProjectTotalPengeluaran,
			ProjectProfit:           p.Profit(),
			ProjectPaymentDate:      stamp(p.ProjectPaymentDate),
		})
	}
	return out
}

// GetProject returns one project in full, in the shape its kind calls for.
func (s *ProjectService) GetProject(ctx context.Context, id string) (*dto.ProjectResponseWrapper, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, notFound(err, id)
	}
	return s.detail(ctx, *project)
}

// detail assembles a project's full representation, loading its usages and logs.
func (s *ProjectService) detail(ctx context.Context, p model.Project) (*dto.ProjectResponseWrapper, error) {
	ids := []string{p.ID}

	logs, err := s.repo.FindLogsFor(ctx, ids)
	if err != nil {
		return nil, err
	}

	common := struct {
		logs []dto.LogResponse
	}{logs: logResponses(logs[p.ID])}

	if p.ProjectType == model.TypePengiriman {
		usage, usageErr := s.repo.FindAssetUsageFor(ctx, ids)
		if usageErr != nil {
			return nil, usageErr
		}
		wrapped := dto.WrapDistribution(dto.DistributionResponse{
			ID:                      p.ID,
			ProjectType:             p.ProjectType,
			ProjectPaymentStatus:    p.ProjectPaymentStatus,
			ProjectStatus:           p.ProjectStatus,
			ProjectName:             p.ProjectName,
			ProjectDescription:      p.ProjectDescription,
			ProjectClientID:         p.ProjectClientID,
			ProjectClientName:       p.ProjectClientName,
			ProjectUseAsset:         assetUsageResponses(usage[p.ID]),
			ProjectDeliveryAddress:  p.ProjectDeliveryAddress,
			ProjectPickupAddress:    p.ProjectPickupAddress,
			ProjectPHLCount:         p.ProjectPHLCount,
			ProjectPHLPay:           p.ProjectPHLPay,
			ProjectStartDate:        stamp(p.ProjectStartDate),
			ProjectEndDate:          stamp(p.ProjectEndDate),
			ProjectTotalPemasukkan:  p.ProjectTotalPemasukkan,
			ProjectTotalPengeluaran: p.ProjectTotalPengeluaran,
			ProjectLogs:             common.logs,
			ProjectPaymentDate:      stamp(p.ProjectPaymentDate),
		})
		return &wrapped, nil
	}

	usage, usageErr := s.repo.FindResourceUsageFor(ctx, ids)
	if usageErr != nil {
		return nil, usageErr
	}
	wrapped := dto.WrapSell(dto.SellResponse{
		ID:                     p.ID,
		ProjectType:            p.ProjectType,
		ProjectPaymentStatus:   p.ProjectPaymentStatus,
		ProjectStatus:          p.ProjectStatus,
		ProjectName:            p.ProjectName,
		ProjectDescription:     p.ProjectDescription,
		ProjectClientID:        p.ProjectClientID,
		ProjectClientName:      p.ProjectClientName,
		ProjectDeliveryAddress: p.ProjectDeliveryAddress,
		ProjectUseResource:     resourceUsageResponses(usage[p.ID]),
		ProjectStartDate:       stamp(p.ProjectStartDate),
		ProjectEndDate:         stamp(p.ProjectEndDate),
		ProjectTotalPemasukkan: p.ProjectTotalPemasukkan,
		ProjectPaymentDate:     stamp(p.ProjectPaymentDate),
		ProjectLogs:            common.logs,
	})
	return &wrapped, nil
}

func assetUsageResponses(usages []model.ProjectAssetUsage) []dto.AssetUsage {
	out := make([]dto.AssetUsage, 0, len(usages))
	for _, u := range usages {
		use, fuel := u.AssetUseCost, u.AssetFuelCost
		out = append(out, dto.AssetUsage{
			TipeAset:      u.TipeAset,
			PlatNomor:     u.PlatNomor,
			AssetUseCost:  &use,
			AssetFuelCost: &fuel,
		})
	}
	return out
}

func resourceUsageResponses(usages []model.ProjectResourceUsage) []dto.ResourceUsage {
	out := make([]dto.ResourceUsage, 0, len(usages))
	for _, u := range usages {
		price, quantity := u.SellPrice, u.QuantityUsed
		out = append(out, dto.ResourceUsage{
			ResourceID:        u.ResourceID,
			SellPrice:         &price,
			ResourceStockUsed: &quantity,
		})
	}
	return out
}

func logResponses(logs []model.LogProject) []dto.LogResponse {
	out := make([]dto.LogResponse, 0, len(logs))
	for _, l := range logs {
		out = append(out, dto.LogResponse{
			ID:         l.ID,
			User:       l.Username,
			Action:     l.Action,
			ActionDate: jsontime.Jakarta(l.ActionDate),
		})
	}
	return out
}
