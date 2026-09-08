package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/pkg/jsontime"
	"github.com/pebintk/kamis-be-go/services/project/internal/dto"
)

// Deps are the services a project reaches out to. Bundled so the constructor
// does not grow a parameter per downstream service.
type Deps struct {
	// Profile resolves the client a project belongs to.
	Profile *httpx.Client
	// Resource prices catalogue items and holds their stock.
	Resource *httpx.Client
	// Asset validates vehicles and owns their reservations.
	Asset *httpx.Client
	// Finance receives the income when a project is paid.
	Finance *httpx.Client
}

// client fetches one client's detail from the profile service.
func (s *ProjectService) client(ctx context.Context, clientID string) (*dto.ClientDetail, error) {
	return httpx.GetData[*dto.ClientDetail](ctx, s.Profile, "/client/"+clientID)
}

// resource fetches one catalogue item from the resource service.
func (s *ProjectService) resource(ctx context.Context, resourceID string) (*dto.ResourceDetail, error) {
	return httpx.GetData[*dto.ResourceDetail](ctx, s.Resource, "/resource/find/"+resourceID)
}

// asset fetches one vehicle from the asset service.
func (s *ProjectService) asset(ctx context.Context, platNomor string) (*dto.AssetDetail, error) {
	return httpx.GetData[*dto.AssetDetail](ctx, s.Asset, "/asset/"+platNomor)
}

// availability asks the asset service which of these vehicles are free for the
// window. excludeProjectID lets a project's own bookings not count against it
// when its dates or vehicles are edited.
func (s *ProjectService) availability(ctx context.Context, platNomors []string, start, end time.Time, excludeProjectID string) (map[string]bool, error) {
	body := dto.AssetAvailabilityRequest{
		PlatNomors:       platNomors,
		StartDate:        jsontime.Jakarta(start),
		EndDate:          jsontime.Jakarta(end),
		ExcludeProjectID: excludeProjectID,
	}
	return httpx.PostData[map[string]bool](ctx, s.Asset, "/asset/reservations/check-availability", body)
}

// reserve books the vehicles to the project for the window.
func (s *ProjectService) reserve(ctx context.Context, platNomors []string, projectID string, start, end time.Time) error {
	return s.Asset.Post(ctx, "/asset/reservations/reserve", dto.AssetReservationRequest{
		PlatNomors: platNomors,
		ProjectID:  projectID,
		StartDate:  jsontime.Jakarta(start),
		EndDate:    jsontime.Jakarta(end),
	})
}

// setReservationStatus moves every booking a project holds, which is how a
// project finishing or being cancelled releases its vehicles.
func (s *ProjectService) setReservationStatus(ctx context.Context, projectID, status string) error {
	return s.Asset.Put(ctx, "/asset/reservations/project/"+projectID+"/status?status="+status, nil)
}

// adjustStock moves a catalogue item's stock. delta is positive to return stock
// and negative to consume it.
func (s *ProjectService) adjustStock(ctx context.Context, resourceID string, delta int) error {
	if delta == 0 {
		return nil
	}
	verb, quantity := "add-stock", delta
	if delta < 0 {
		verb, quantity = "deduct-stock", -delta
	}
	return s.Resource.Put(ctx, "/resource/"+resourceID+"/"+verb,
		map[string]int{"quantity": quantity})
}

// returnStock puts consumed stock back, logging rather than failing: it runs on
// paths where the caller's own work is already done or already failing.
func (s *ProjectService) returnStock(ctx context.Context, resourceID string, quantity int) {
	if err := s.adjustStock(ctx, resourceID, quantity); err != nil {
		slog.WarnContext(ctx, "could not return resource stock",
			"resource", resourceID, "quantity", quantity, "error", err)
	}
}
