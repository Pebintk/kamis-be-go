package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/services/asset/internal/dto"
	"github.com/karina/kamis-be-go/services/asset/internal/model"
	"github.com/karina/kamis-be-go/services/asset/internal/repository"
)

type ReservationService struct {
	repo *repository.AssetRepository
}

func NewReservationService(repo *repository.AssetRepository) *ReservationService {
	return &ReservationService{repo: repo}
}

// validStatus reports whether s is one the service recognises. Java accepted any
// string from the query parameter and wrote it straight to the column, so
// ?status=garbage silently corrupted a booking — and since availability keys off
// exact status values, that booking then blocked the vehicle forever.
func validStatus(s string) bool {
	switch s {
	case model.ReservationDirencanakan, model.ReservationDilaksanakan,
		model.ReservationSelesai, model.ReservationBatal:
		return true
	}
	return false
}

// CheckAvailability reports, per vehicle, whether it can be booked for the
// window. A plate that does not exist answers false rather than failing the
// request, as in Java.
func (s *ReservationService) CheckAvailability(ctx context.Context, req dto.AssetAvailabilityRequest) (map[string]bool, error) {
	start, end := req.StartDate.Time, req.EndDate.Time
	if start.After(end) {
		return nil, apierr.Invalidf("Tanggal mulai harus sebelum tanggal selesai")
	}

	available := make(map[string]bool, len(req.PlatNomors))
	for _, plat := range req.PlatNomors {
		available[plat] = false
	}

	// One lookup for every plate, then one overlap query for all of them. Java
	// ran an asset lookup plus an overlap query per plate.
	assets, err := s.repo.FindAssetsByPlatNomors(ctx, req.PlatNomors)
	if err != nil {
		return nil, err
	}
	overlapping, err := s.repo.FindOverlappingReservations(ctx, req.PlatNomors, start, end, req.ExcludeProjectID)
	if err != nil {
		return nil, err
	}
	booked := make(map[string]bool, len(overlapping))
	for _, r := range overlapping {
		booked[r.PlatNomor] = true
	}

	for _, plat := range req.PlatNomors {
		asset, known := assets[plat]
		if !known || asset.DeletedAt.Valid {
			continue
		}
		// A vehicle in the workshop cannot be booked whatever its calendar says.
		if asset.Status == model.AssetSedangMaintenance {
			continue
		}
		available[plat] = !booked[plat]
	}
	return available, nil
}

// Reserve books every listed vehicle to a project for the window, or none of
// them if any is unavailable.
func (s *ReservationService) Reserve(ctx context.Context, req dto.AssetReservationRequest) ([]dto.AssetReservationResponse, error) {
	if len(req.PlatNomors) == 0 {
		return nil, apierr.Invalidf("Daftar asset tidak boleh kosong")
	}
	start, end := req.StartDate.Time, req.EndDate.Time
	if start.After(end) {
		return nil, apierr.Invalidf("Tanggal mulai harus sebelum tanggal selesai")
	}

	available, err := s.CheckAvailability(ctx, dto.AssetAvailabilityRequest{
		PlatNomors: req.PlatNomors,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
	})
	if err != nil {
		return nil, err
	}

	var unavailable []string
	for plat, ok := range available {
		if !ok {
			unavailable = append(unavailable, plat)
		}
	}
	if len(unavailable) > 0 {
		// Sorted so the message is stable; ranging a map is not ordered.
		sort.Strings(unavailable)
		return nil, apierr.Invalidf("Asset berikut tidak tersedia: %s", strings.Join(unavailable, ", "))
	}

	reservations := make([]model.AssetReservation, 0, len(req.PlatNomors))
	for _, plat := range req.PlatNomors {
		reservations = append(reservations, model.AssetReservation{
			PlatNomor:         plat,
			ProjectID:         req.ProjectID,
			StartDate:         start,
			EndDate:           end,
			ReservationStatus: model.ReservationDirencanakan,
		})
	}
	if err := s.repo.CreateReservations(ctx, reservations); err != nil {
		return nil, err
	}

	return s.withAssets(ctx, reservations)
}

// UpdateStatus moves one booking to a new status and adjusts the vehicle's own
// status to match.
func (s *ReservationService) UpdateStatus(ctx context.Context, reservationID, status string) (*dto.AssetReservationResponse, error) {
	if !validStatus(status) {
		return nil, apierr.Invalidf("Status reservasi tidak valid: %s", status)
	}
	if !isUUID(reservationID) {
		return nil, apierr.Invalidf("Format ID reservasi tidak valid")
	}

	reservation, err := s.repo.FindReservationByID(ctx, reservationID)
	if err != nil {
		return nil, reservationNotFound(err, reservationID)
	}

	reservation.ReservationStatus = status
	if err := s.repo.SaveReservation(ctx, reservation); err != nil {
		return nil, err
	}
	s.syncAssetStatus(ctx, reservation.PlatNomor, status)

	rows, err := s.withAssets(ctx, []model.AssetReservation{*reservation})
	if err != nil {
		return nil, err
	}
	return &rows[0], nil
}

// UpdateProjectStatus moves every booking a project holds. An unknown project
// yields an empty list rather than an error, as in Java — the project service
// calls this on every status change, including for projects that reserved
// nothing.
func (s *ReservationService) UpdateProjectStatus(ctx context.Context, projectID, status string) ([]dto.AssetReservationResponse, error) {
	if !validStatus(status) {
		return nil, apierr.Invalidf("Status reservasi tidak valid: %s", status)
	}

	if err := s.repo.SetReservationStatusForProject(ctx, projectID, status); err != nil {
		return nil, err
	}

	reservations, err := s.repo.FindReservationsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(reservations))
	for _, r := range reservations {
		if !seen[r.PlatNomor] {
			seen[r.PlatNomor] = true
			s.syncAssetStatus(ctx, r.PlatNomor, status)
		}
	}
	return s.withAssets(ctx, reservations)
}

// ByAsset returns every booking held against one vehicle.
func (s *ReservationService) ByAsset(ctx context.Context, platNomor string) ([]dto.AssetReservationResponse, error) {
	reservations, err := s.repo.FindReservationsByAsset(ctx, platNomor)
	if err != nil {
		return nil, err
	}
	return s.withAssets(ctx, reservations)
}

// ByProject returns every booking a project holds.
func (s *ReservationService) ByProject(ctx context.Context, projectID string) ([]dto.AssetReservationResponse, error) {
	reservations, err := s.repo.FindReservationsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return s.withAssets(ctx, reservations)
}

// syncAssetStatus keeps the vehicle's own status in step with its bookings: it
// goes out on a job when one starts, and returns to the pool once the last live
// booking is finished or cancelled.
//
// A failure here is logged rather than returned: the booking change is already
// committed, and refusing the request would report a failure for work that was
// done.
func (s *ReservationService) syncAssetStatus(ctx context.Context, platNomor, reservationStatus string) {
	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		slog.WarnContext(ctx, "could not sync asset status", "asset", platNomor, "error", err)
		return
	}

	switch reservationStatus {
	case model.ReservationDilaksanakan:
		asset.Status = model.AssetDalamAktivitas
	case model.ReservationSelesai, model.ReservationBatal:
		active, countErr := s.repo.CountActiveReservations(ctx, platNomor)
		if countErr != nil {
			slog.WarnContext(ctx, "could not count active reservations", "asset", platNomor, "error", countErr)
			return
		}
		if active > 0 {
			return // still booked elsewhere
		}
		// A vehicle in the workshop stays there; its bookings ending does not
		// make it available.
		if asset.Status == model.AssetSedangMaintenance {
			return
		}
		asset.Status = model.AssetTersedia
	default:
		return // "Direncanakan" does not change the vehicle's status
	}

	if err := s.repo.Save(ctx, asset); err != nil {
		slog.WarnContext(ctx, "could not save asset status", "asset", platNomor, "error", err)
	}
}

// withAssets attaches each booking's vehicle name and type. Java looked the
// asset up once per reservation while mapping.
func (s *ReservationService) withAssets(ctx context.Context, reservations []model.AssetReservation) ([]dto.AssetReservationResponse, error) {
	platNomors := make([]string, 0, len(reservations))
	for _, r := range reservations {
		platNomors = append(platNomors, r.PlatNomor)
	}
	assets, err := s.repo.FindAssetsByPlatNomors(ctx, platNomors)
	if err != nil {
		return nil, err
	}

	out := make([]dto.AssetReservationResponse, 0, len(reservations))
	for _, r := range reservations {
		asset := assets[r.PlatNomor]
		out = append(out, dto.AssetReservationResponse{
			ID:                r.ID,
			PlatNomor:         r.PlatNomor,
			ProjectID:         r.ProjectID,
			StartDate:         jsontime.UTC(r.StartDate),
			EndDate:           jsontime.UTC(r.EndDate),
			ReservationStatus: r.ReservationStatus,
			AssetName:         asset.Nama,
			AssetType:         asset.JenisAset,
		})
	}
	return out, nil
}
