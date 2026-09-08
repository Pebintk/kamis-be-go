package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/services/project/internal/dto"
	"github.com/pebintk/kamis-be-go/services/project/internal/model"
)

// UpdateProject edits a project that is still open, re-pricing it and moving
// whatever it holds.
func (s *ProjectService) UpdateProject(ctx context.Context, id string, req dto.UpdateProjectRequest) (*dto.ProjectResponseWrapper, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, notFound(err, id)
	}
	if !project.Active() {
		return nil, apierr.Invalidf("Proyek yang sudah selesai atau batal tidak dapat diubah.")
	}

	start, end := project.ProjectStartDate, project.ProjectEndDate
	if req.ProjectStartDate != nil {
		at := atNoon(req.ProjectStartDate.Time)
		start = &at
	}
	if req.ProjectEndDate != nil {
		at := atNoon(req.ProjectEndDate.Time)
		end = &at
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, apierr.Invalidf("Tanggal akhir proyek tidak boleh sebelum tanggal mulai proyek")
	}

	changes := changeLog{}
	changes.text("deskripsi", project.ProjectDescription, req.ProjectDescription)
	changes.text("alamat pengiriman", project.ProjectDeliveryAddress, req.ProjectDeliveryAddress)

	held, err := s.currentUsage(ctx, *project)
	if err != nil {
		return nil, err
	}

	built, err := s.planUsage(ctx, planInput{
		projectType:     project.ProjectType,
		assets:          req.ProjectUseAsset,
		resources:       req.ProjectUseResource,
		phlCount:        orExisting(req.ProjectPHLCount, project.ProjectPHLCount),
		phlPay:          orExisting(req.ProjectPHLPay, project.ProjectPHLPay),
		totalPemasukkan: orExisting(req.ProjectTotalPemasukkan, project.ProjectTotalPemasukkan),
		start:           derefTime(start),
		end:             derefTime(end),
		// A project's own bookings must not count against it when its dates or
		// vehicles change; without this, re-saving an unchanged distribution
		// would report every one of its own vehicles as unavailable.
		excludeProjectID: project.ID,
		heldStock:        held.stock,
	})
	if err != nil {
		return nil, err
	}

	applyEdits(project, req, start, end, built)

	if err := s.repo.Save(ctx, project); err != nil {
		return nil, err
	}
	if project.ProjectType == model.TypePengiriman {
		if err := s.repo.ReplaceAssetUsage(ctx, project.ID, built.assets); err != nil {
			return nil, err
		}
	} else if err := s.repo.ReplaceResourceUsage(ctx, project.ID, built.resources); err != nil {
		return nil, err
	}

	if err := s.repo.AppendLog(ctx, &model.LogProject{
		ProjectID: project.ID,
		Username:  username(ctx),
		Action:    changes.String(),
	}); err != nil {
		return nil, err
	}

	s.moveHoldings(ctx, *project, held, built)
	return s.detail(ctx, *project)
}

// holdings is what a project already has: how much of each catalogue item it
// consumes, and which vehicles it has booked.
type holdings struct {
	stock  map[string]int
	assets []string
}

func (s *ProjectService) currentUsage(ctx context.Context, p model.Project) (holdings, error) {
	out := holdings{stock: map[string]int{}}
	ids := []string{p.ID}

	if p.ProjectType == model.TypePengiriman {
		usage, err := s.repo.FindAssetUsageFor(ctx, ids)
		if err != nil {
			return out, err
		}
		for _, u := range usage[p.ID] {
			out.assets = append(out.assets, u.PlatNomor)
		}
		return out, nil
	}

	usage, err := s.repo.FindResourceUsageFor(ctx, ids)
	if err != nil {
		return out, err
	}
	for _, u := range usage[p.ID] {
		out.stock[u.ResourceID] += u.QuantityUsed
	}
	return out, nil
}

// moveHoldings settles the difference with the other services: the net stock
// change, and a re-booking of the vehicles.
//
// Java returned every unit of stock the project held and then deducted the new
// amounts. That inflates the catalogue in between, and leaves it inflated if the
// deductions fail. The net delta touches each item once.
func (s *ProjectService) moveHoldings(ctx context.Context, p model.Project, held holdings, built plan) {
	if p.ProjectType != model.TypePengiriman {
		wanted := map[string]int{}
		for _, u := range built.resources {
			wanted[u.ResourceID] += u.QuantityUsed
		}
		for id := range held.stock {
			if _, ok := wanted[id]; !ok {
				wanted[id] = 0
			}
		}
		for id, want := range wanted {
			// Positive returns stock, negative consumes it.
			if delta := held.stock[id] - want; delta != 0 {
				if err := s.adjustStock(ctx, id, delta); err != nil {
					slog.WarnContext(ctx, "could not settle resource stock after an edit",
						"project", p.ID, "resource", id, "delta", delta, "error", err)
				}
			}
		}
		return
	}

	// Vehicles are re-booked wholesale: the old bookings are cancelled and the
	// new set reserved, which is the only operation the asset service offers.
	if err := s.setReservationStatus(ctx, p.ID, "Batal"); err != nil {
		slog.WarnContext(ctx, "could not release the project's previous reservations",
			"project", p.ID, "error", err)
	}
	if len(built.platNomors) > 0 && p.ProjectStartDate != nil && p.ProjectEndDate != nil {
		if err := s.reserve(ctx, built.platNomors, p.ID, *p.ProjectStartDate, *p.ProjectEndDate); err != nil {
			slog.WarnContext(ctx, "could not re-reserve the project's vehicles",
				"project", p.ID, "error", err)
		}
	}
}

// applyEdits writes the request's fields onto the project.
func applyEdits(p *model.Project, req dto.UpdateProjectRequest, start, end *time.Time, built plan) {
	if req.ProjectDescription != "" {
		p.ProjectDescription = req.ProjectDescription
	}
	if req.ProjectDeliveryAddress != "" {
		p.ProjectDeliveryAddress = req.ProjectDeliveryAddress
	}
	p.ProjectStartDate, p.ProjectEndDate = start, end

	if p.ProjectType == model.TypePengiriman {
		if req.ProjectPickupAddress != "" {
			p.ProjectPickupAddress = req.ProjectPickupAddress
		}
		if req.ProjectPHLCount != nil {
			p.ProjectPHLCount = req.ProjectPHLCount
		}
		if req.ProjectPHLPay != nil {
			p.ProjectPHLPay = req.ProjectPHLPay
		}
		p.ProjectTotalPengeluaran = built.totalPengeluaran
	}
	p.ProjectTotalPemasukkan = built.totalPemasukkan
}

// changeLog builds the human-readable audit entry an edit records.
type changeLog struct{ lines []string }

func (c *changeLog) text(field, before, after string) {
	if after != "" && after != before {
		c.lines = append(c.lines, fmt.Sprintf("  - Mengubah %s menjadi: %s", field, after))
	}
}

func (c changeLog) String() string {
	if len(c.lines) == 0 {
		return "Memperbarui Proyek"
	}
	return "Memperbarui Proyek:\n" + strings.Join(c.lines, "\n")
}

// orExisting prefers the submitted value and falls back to what is stored, so an
// omitted field is left alone rather than cleared.
func orExisting[T any](submitted, existing *T) *T {
	if submitted != nil {
		return submitted
	}
	return existing
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
