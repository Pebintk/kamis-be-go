package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/services/project/internal/dto"
	"github.com/karina/kamis-be-go/services/project/internal/model"
)

// plan is everything a create or edit works out before touching anything: the
// rows to write, the money, and the external effects still to apply.
//
// Separating the two halves is the point. Java interleaved them — it deducted
// resource stock inside its validation loop, so a resource that failed
// validation left the stock of every earlier one already gone, and it reserved
// vehicles only after the insert, so a refused booking left a project holding
// vehicles it had never reserved.
type plan struct {
	assets    []model.ProjectAssetUsage
	resources []model.ProjectResourceUsage

	totalPemasukkan  *int64
	totalPengeluaran *int64

	// platNomors is what to reserve once the project has an id.
	platNomors []string
}

// AddProject records a new project and applies its external effects.
func (s *ProjectService) AddProject(ctx context.Context, req dto.AddProjectRequest) (*dto.ProjectResponseWrapper, error) {
	start, end := req.ProjectStartDate.Time, req.ProjectEndDate.Time
	if end.Before(start) {
		return nil, apierr.Invalidf("Tanggal akhir proyek tidak boleh sebelum tanggal mulai proyek")
	}

	client, err := s.client(ctx, req.ProjectClientID)
	if err != nil || client == nil {
		return nil, apierr.Invalidf("Pastikan ID Klien sudah terdaftar dalam sistem")
	}

	built, err := s.planUsage(ctx, planInput{
		projectType:      *req.ProjectType,
		assets:           req.ProjectUseAsset,
		resources:        req.ProjectUseResource,
		phlCount:         req.ProjectPHLCount,
		phlPay:           req.ProjectPHLPay,
		totalPemasukkan:  req.ProjectTotalPemasukkan,
		start:            start,
		end:              end,
		excludeProjectID: "",
	})
	if err != nil {
		return nil, err
	}

	planned := model.PaymentBelumLunas
	project := model.Project{
		ProjectType:            *req.ProjectType,
		ProjectStatus:          model.StatusDirencanakan,
		ProjectPaymentStatus:   &planned,
		ProjectName:            req.ProjectName,
		ProjectDescription:     req.ProjectDescription,
		ProjectClientID:        req.ProjectClientID,
		ProjectClientName:      client.NameClient,
		ProjectDeliveryAddress: req.ProjectDeliveryAddress,
		ProjectStartDate:       &start,
		ProjectEndDate:         &end,
		ProjectTotalPemasukkan: built.totalPemasukkan,

		ProjectPickupAddress:    req.ProjectPickupAddress,
		ProjectPHLCount:         req.ProjectPHLCount,
		ProjectPHLPay:           req.ProjectPHLPay,
		ProjectTotalPengeluaran: built.totalPengeluaran,
	}

	err = s.repo.CreateWithGeneratedID(ctx, &project, built.assets, built.resources,
		username(ctx), func(id string) string { return "Menambahkan " + id })
	if err != nil {
		return nil, err
	}

	// Everything external happens after the project exists, and unwinds if it
	// fails: the alternative is a project that owns stock or vehicles the other
	// services do not know about.
	if err := s.applyEffects(ctx, project.ID, built, start, end); err != nil {
		if delErr := s.repo.Delete(ctx, project.ID); delErr != nil {
			slog.ErrorContext(ctx, "could not roll back a project whose side effects failed",
				"project", project.ID, "error", delErr)
		}
		return nil, err
	}

	return s.detail(ctx, project)
}

// applyEffects consumes stock and books vehicles, undoing its own work if it
// cannot finish.
func (s *ProjectService) applyEffects(ctx context.Context, projectID string, built plan, start, end time.Time) error {
	var consumed []model.ProjectResourceUsage
	for _, usage := range built.resources {
		if err := s.adjustStock(ctx, usage.ResourceID, -usage.QuantityUsed); err != nil {
			for _, done := range consumed {
				s.returnStock(ctx, done.ResourceID, done.QuantityUsed)
			}
			return apierr.Invalidf("Gagal mengurangi stok resource %s: %v", usage.ResourceID, err)
		}
		consumed = append(consumed, usage)
	}

	if len(built.platNomors) > 0 {
		if err := s.reserve(ctx, built.platNomors, projectID, start, end); err != nil {
			for _, done := range consumed {
				s.returnStock(ctx, done.ResourceID, done.QuantityUsed)
			}
			return apierr.Invalidf("Gagal memesan aset: %v", err)
		}
	}
	return nil
}

// planInput is what planUsage needs; a struct because a create and an edit pass
// the same eight things.
type planInput struct {
	projectType      bool
	assets           []dto.AssetUsage
	resources        []dto.ResourceUsage
	phlCount         *int
	phlPay           *int64
	totalPemasukkan  *int64
	start, end       time.Time
	excludeProjectID string
}

// planUsage validates the requested vehicles or catalogue items and works out
// the money. It reads from the other services but changes nothing.
func (s *ProjectService) planUsage(ctx context.Context, in planInput) (plan, error) {
	if in.projectType == model.TypePengiriman {
		return s.planDistribution(ctx, in)
	}
	return s.planSale(ctx, in)
}

// planDistribution prices a distribution: every vehicle's use and fuel, plus the
// day labour.
func (s *ProjectService) planDistribution(ctx context.Context, in planInput) (plan, error) {
	var out plan
	outgoings := int64(0)

	for _, usage := range in.assets {
		if usage.AssetUseCost == nil || usage.AssetFuelCost == nil {
			return out, apierr.Invalidf("Biaya penggunaan dan bahan bakar aset %s tidak boleh kosong", usage.PlatNomor)
		}
		if _, err := s.asset(ctx, usage.PlatNomor); err != nil {
			return out, apierr.Invalidf("Aset dengan nomor plat %s tidak terdaftar dalam sistem", usage.PlatNomor)
		}

		out.assets = append(out.assets, model.ProjectAssetUsage{
			PlatNomor:     usage.PlatNomor,
			TipeAset:      usage.TipeAset,
			AssetUseCost:  *usage.AssetUseCost,
			AssetFuelCost: *usage.AssetFuelCost,
		})
		out.platNomors = append(out.platNomors, usage.PlatNomor)
		outgoings += int64(*usage.AssetUseCost) + int64(*usage.AssetFuelCost)
	}

	// One availability call for every vehicle, where Java asked per vehicle.
	if len(out.platNomors) > 0 {
		free, err := s.availability(ctx, out.platNomors, in.start, in.end, in.excludeProjectID)
		if err != nil {
			return out, apierr.Invalidf("Gagal memeriksa ketersediaan aset: %v", err)
		}
		for _, plat := range out.platNomors {
			if !free[plat] {
				return out, apierr.Invalidf(
					"Aset dengan nomor plat %s tidak tersedia untuk periode waktu yang diminta", plat)
			}
		}
	}

	// Day labour. Java multiplied both fields unguarded, so a distribution
	// submitted without them raised a NullPointerException and a 500; an absent
	// count or rate is simply no labour cost.
	if in.phlCount != nil && in.phlPay != nil {
		if *in.phlCount < 0 || *in.phlPay < 0 {
			return out, apierr.Invalidf("Jumlah dan biaya PHL tidak boleh kurang dari 0")
		}
		outgoings += *in.phlPay * int64(*in.phlCount)
	}

	out.totalPengeluaran = &outgoings
	out.totalPemasukkan = in.totalPemasukkan
	return out, nil
}

// planSale prices a sale from the catalogue, at the price each item carries now.
func (s *ProjectService) planSale(ctx context.Context, in planInput) (plan, error) {
	var out plan
	income := int64(0)

	for _, usage := range in.resources {
		if usage.ResourceStockUsed == nil || *usage.ResourceStockUsed <= 0 {
			return out, apierr.Invalidf("Jumlah resource yang digunakan tidak boleh kurang dari 0")
		}

		item, err := s.resource(ctx, usage.ResourceID)
		if err != nil || item == nil {
			return out, apierr.Invalidf("Resource dengan ID %s tidak terdaftar dalam sistem", usage.ResourceID)
		}
		// Checked before anything is consumed, so a sale that cannot be filled
		// fails without having drawn down the items ahead of it. Java found out
		// only when the resource service refused a deduction, halfway through.
		if item.ResourceStock < *usage.ResourceStockUsed {
			return out, apierr.Invalidf("Stok %s tidak mencukupi. Tersedia: %d, diminta: %d",
				item.ResourceName, item.ResourceStock, *usage.ResourceStockUsed)
		}

		out.resources = append(out.resources, model.ProjectResourceUsage{
			ResourceID:   usage.ResourceID,
			SellPrice:    item.ResourcePrice,
			QuantityUsed: *usage.ResourceStockUsed,
		})
		income += int64(item.ResourcePrice) * int64(*usage.ResourceStockUsed)
	}

	// A sale's income is what it sold, not what the caller claimed.
	out.totalPemasukkan = &income
	return out, nil
}
