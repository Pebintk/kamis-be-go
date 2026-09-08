package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/pkg/jsontime"
	"github.com/pebintk/kamis-be-go/services/asset/internal/dto"
	"github.com/pebintk/kamis-be-go/services/asset/internal/model"
	"github.com/pebintk/kamis-be-go/services/asset/internal/repository"
)

// maintenanceActivityType is the ledger's code for this kind of entry, copied
// from the legacy AddLapkeuDTO call site (commented there as PURCHASE).
const maintenanceActivityType = 3

// errorDateLayout is how dates are rendered inside the Indonesian conflict
// messages the frontend displays.
const errorDateLayout = "02/01/2006"

// minimumGapBeforeReservation is how far a maintenance must start ahead of a
// planned reservation.
const minimumGapBeforeReservation = 24 * time.Hour

type MaintenanceService struct {
	repo    *repository.AssetRepository
	finance *httpx.Client
}

func NewMaintenanceService(repo *repository.AssetRepository, finance *httpx.Client) *MaintenanceService {
	return &MaintenanceService{repo: repo, finance: finance}
}

// Create records a maintenance job and takes the vehicle out of service.
func (s *MaintenanceService) Create(ctx context.Context, req dto.MaintenanceRequest) (*dto.MaintenanceResponse, error) {
	if *req.Biaya <= 0 {
		return nil, apierr.Invalidf("Biaya harus bernilai positif")
	}
	start := req.TanggalMulaiMaintenance.Time

	asset, err := s.repo.FindByPlatNomor(ctx, req.PlatNomor)
	if err != nil {
		return nil, notFound(err, req.PlatNomor)
	}

	switch asset.Status {
	case model.AssetSedangMaintenance:
		return nil, apierr.Invalidf("Asset dengan plat nomor %s sedang dalam maintenance", req.PlatNomor)
	case model.AssetDalamAktivitas:
		return nil, apierr.Invalidf("Asset dengan plat nomor %s sedang digunakan dalam aktivitas", req.PlatNomor)
	}

	reservations, err := s.repo.FindReservationsByAsset(ctx, req.PlatNomor)
	if err != nil {
		return nil, err
	}
	if err := checkReservationConflict(reservations, start, req.PlatNomor); err != nil {
		return nil, err
	}

	asset.Status = model.AssetSedangMaintenance
	if err := s.repo.Save(ctx, asset); err != nil {
		return nil, err
	}

	maintenance := model.Maintenance{
		TanggalMulaiMaintenance: start,
		DeskripsiPekerjaan:      req.DeskripsiPekerjaan,
		Biaya:                   *req.Biaya,
		Status:                  model.AssetSedangMaintenance,
		AssetPlatNomor:          asset.PlatNomor,
	}
	if req.TanggalSelesaiMaintenance != nil {
		finish := req.TanggalSelesaiMaintenance.Time
		maintenance.TanggalSelesaiMaintenance = &finish
	}
	if err := s.repo.CreateMaintenance(ctx, &maintenance); err != nil {
		return nil, err
	}

	s.reportToFinance(ctx, maintenance, *asset)

	out := maintenanceResponse(maintenance, *asset)
	return &out, nil
}

// reportToFinance posts the cost to the ledger. A failure is logged and
// swallowed, as in Java — the maintenance record is the source of truth and the
// finance service may not even be running.
func (s *MaintenanceService) reportToFinance(ctx context.Context, m model.Maintenance, asset model.Asset) {
	body := dto.AddLapkeuRequest{
		ID:           strconv.FormatInt(m.ID, 10),
		ActivityType: maintenanceActivityType,
		Pemasukan:    0,
		Pengeluaran:  int64(m.Biaya),
		Description:  "Maintenance - " + asset.PlatNomor,
		PaymentDate:  jsontime.UTC(m.TanggalMulaiMaintenance),
	}
	if err := s.finance.Post(ctx, "/lapkeu/add", body); err != nil {
		slog.WarnContext(ctx, "could not record maintenance in the ledger",
			"maintenance", m.ID, "error", err)
	}
}

// Complete closes a maintenance job and returns the vehicle to service.
func (s *MaintenanceService) Complete(ctx context.Context, id int64) (*dto.MaintenanceResponse, error) {
	maintenance, err := s.repo.FindMaintenanceByID(ctx, id)
	if err != nil {
		return nil, maintenanceNotFound(err, id)
	}
	if maintenance.Status != model.AssetSedangMaintenance {
		return nil, apierr.Invalidf("Maintenance ini sudah selesai")
	}

	asset, err := s.repo.FindByPlatNomor(ctx, maintenance.AssetPlatNomor)
	if err != nil {
		return nil, notFound(err, maintenance.AssetPlatNomor)
	}

	now := time.Now()
	maintenance.Status = repository.StatusSelesai
	maintenance.TanggalSelesaiMaintenance = &now
	asset.Status = model.AssetTersedia

	if err := s.repo.Save(ctx, asset); err != nil {
		return nil, err
	}
	if err := s.repo.SaveMaintenance(ctx, maintenance); err != nil {
		return nil, err
	}

	out := maintenanceResponse(*maintenance, *asset)
	return &out, nil
}

// ListAll returns every maintenance record.
func (s *MaintenanceService) ListAll(ctx context.Context) ([]dto.MaintenanceResponse, error) {
	records, err := s.repo.FindAllMaintenance(ctx)
	if err != nil {
		return nil, err
	}
	return s.withAssets(ctx, records)
}

// ListInProgress returns the jobs currently open.
//
// Java raised an exception when the list was empty, which the controller turned
// into a 404 with a null body. Having no vehicle in the workshop is a normal
// state, not an error — and the frontend's own handler already falls back to an
// empty array on a 200, so it was written expecting this.
func (s *MaintenanceService) ListInProgress(ctx context.Context) ([]dto.MaintenanceResponse, error) {
	records, err := s.repo.FindMaintenanceByStatus(ctx, model.AssetSedangMaintenance)
	if err != nil {
		return nil, err
	}
	return s.withAssets(ctx, records)
}

// withAssets attaches each record's asset name. Java reached through the
// @ManyToOne association while mapping every row; this resolves them in one
// query for the whole list.
func (s *MaintenanceService) withAssets(ctx context.Context, records []model.Maintenance) ([]dto.MaintenanceResponse, error) {
	platNomors := make([]string, 0, len(records))
	for _, m := range records {
		platNomors = append(platNomors, m.AssetPlatNomor)
	}
	assets, err := s.repo.FindAssetsByPlatNomors(ctx, platNomors)
	if err != nil {
		return nil, err
	}

	out := make([]dto.MaintenanceResponse, 0, len(records))
	for _, m := range records {
		out = append(out, maintenanceResponse(m, assets[m.AssetPlatNomor]))
	}
	return out, nil
}

// checkReservationConflict refuses a maintenance that clashes with a booking.
//
// Two rules, both from the legacy service: the start date may not fall inside an
// active reservation, and it may not begin less than a day before a reservation
// that is still only planned.
func checkReservationConflict(reservations []model.AssetReservation, start time.Time, platNomor string) error {
	for _, r := range reservations {
		if !r.Active() {
			continue
		}

		if !start.Before(r.StartDate) && !start.After(r.EndDate) {
			return apierr.Invalidf(
				"Maintenance tidak dapat dijadwalkan karena asset dengan plat nomor %s sudah direservasi untuk project %s (status: %s) dari %s sampai %s",
				platNomor, r.ProjectID, r.ReservationStatus,
				r.StartDate.Format(errorDateLayout), r.EndDate.Format(errorDateLayout))
		}

		// Only planned reservations get the buffer; one already under way is
		// covered by the overlap check above.
		if r.ReservationStatus == model.ReservationDirencanakan && start.Before(r.StartDate) &&
			r.StartDate.Sub(start) < minimumGapBeforeReservation {
			return apierr.Invalidf(
				"Maintenance tidak dapat dijadwalkan karena terlalu dekat dengan reservasi yang direncanakan untuk project %s (dimulai %s). Berikan jarak minimal 1 hari.",
				r.ProjectID, r.StartDate.Format(errorDateLayout))
		}
	}
	return nil
}

func maintenanceResponse(m model.Maintenance, asset model.Asset) dto.MaintenanceResponse {
	out := dto.MaintenanceResponse{
		ID:                      m.ID,
		TanggalMulaiMaintenance: jsontime.UTC(m.TanggalMulaiMaintenance),
		DeskripsiPekerjaan:      m.DeskripsiPekerjaan,
		Biaya:                   m.Biaya,
		Status:                  m.Status,
		PlatNomor:               m.AssetPlatNomor,
		NamaAset:                asset.Nama,
	}
	if m.TanggalSelesaiMaintenance != nil {
		finished := jsontime.UTC(*m.TanggalSelesaiMaintenance)
		out.TanggalSelesaiMaintenance = &finished
	}
	return out
}
