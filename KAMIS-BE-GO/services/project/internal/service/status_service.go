package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/jsontime"
	"github.com/pebintk/kamis-be-go/services/project/internal/dto"
	"github.com/pebintk/kamis-be-go/services/project/internal/model"
)

// Ledger activity codes, from the legacy AddLapkeuDTO call site.
const (
	activityPenjualan  = 0
	activityDistribusi = 1
)

// statusText names a status for the audit log.
var statusText = map[int]string{
	model.StatusDirencanakan: "Direncanakan",
	model.StatusDilaksanakan: "Dilaksanakan",
	model.StatusSelesai:      "Selesai",
	model.StatusBatal:        "Batal",
}

// atNoon normalises a date to midday local time.
//
// Java did this to both the start and end dates, with a comment explaining why:
// a date stored at midnight or at 23:59 shifts to the previous or next day when
// it crosses a timezone, and these are calendar dates, not instants. Midday has
// twelve hours of slack in either direction.
func atNoon(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, t.Location())
}

// UpdateStatus moves a project along its lifecycle and releases what it holds.
func (s *ProjectService) UpdateStatus(ctx context.Context, id string, newStatus int) (*dto.ProjectResponseWrapper, error) {
	if _, known := statusText[newStatus]; !known {
		// Java accepted any integer here, so a stray value was written to the
		// column and logged as "Mengubah Status menjadi " with nothing after it.
		return nil, apierr.Invalidf("Status proyek tidak valid: %d", newStatus)
	}

	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, notFound(err, id)
	}

	if err := checkStatusTransition(project.ProjectStatus, newStatus); err != nil {
		return nil, err
	}

	// Starting a project dates it from today; finishing or cancelling one closes
	// it today.
	now := atNoon(time.Now())
	switch newStatus {
	case model.StatusDilaksanakan:
		project.ProjectStartDate = &now
	case model.StatusSelesai, model.StatusBatal:
		project.ProjectEndDate = &now
	}
	project.ProjectStatus = newStatus

	if err := s.repo.Save(ctx, project); err != nil {
		return nil, err
	}
	if err := s.repo.AppendLog(ctx, &model.LogProject{
		ProjectID: project.ID,
		Username:  username(ctx),
		Action:    "Mengubah Status menjadi " + statusText[newStatus],
	}); err != nil {
		return nil, err
	}

	// A project that is over stops holding its vehicles. The booking status
	// mirrors the project's, so the asset service can tell a finished job from
	// an abandoned one.
	if reservationStatus, ok := map[int]string{
		model.StatusSelesai: "Selesai",
		model.StatusBatal:   "Batal",
	}[newStatus]; ok {
		if err := s.setReservationStatus(ctx, project.ID, reservationStatus); err != nil {
			// The status change is committed; a stuck reservation is a
			// reconcilable inconsistency, not a reason to refuse the request.
			slog.WarnContext(ctx, "could not release the project's asset reservations",
				"project", project.ID, "status", reservationStatus, "error", err)
		}
	}

	return s.detail(ctx, *project)
}

// checkStatusTransition enforces the lifecycle: a project is planned, then
// carried out, then finished; it can be cancelled up to the point it finishes,
// and nothing moves once it is finished or cancelled.
func checkStatusTransition(current, next int) error {
	if current == model.StatusSelesai || current == model.StatusBatal {
		return apierr.Invalidf("Status proyek sudah selesai atau batal tidak dapat diubah.")
	}
	if current == model.StatusDilaksanakan && next == model.StatusDirencanakan {
		return apierr.Invalidf("Status proyek tidak bisa dikembalikan ke 'Direncanakan' dari 'Dilaksanakan'.")
	}
	if current == model.StatusDirencanakan && next == model.StatusSelesai {
		return apierr.Invalidf("Status proyek tidak bisa langsung menjadi 'Selesai' dari 'Direncanakan'.")
	}
	return nil
}

// UpdatePayment records that a project has been paid, or refunded.
func (s *ProjectService) UpdatePayment(ctx context.Context, id string, newStatus int) (*dto.ProjectResponseWrapper, error) {
	switch newStatus {
	case model.PaymentBelumLunas, model.PaymentTelahLunas, model.PaymentDikembalikan:
	default:
		// Java accepted any integer here too.
		return nil, apierr.Invalidf("Status pembayaran tidak valid: %d", newStatus)
	}

	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, notFound(err, id)
	}

	if err := checkPaymentTransition(*project, newStatus); err != nil {
		return nil, err
	}

	// The payment date is stamped on every change, a refund included — so a
	// refunded project reports when it was refunded, not when it was paid.
	// Inherited; the frontend renders this field as "payment date".
	now := time.Now()
	project.ProjectPaymentStatus = &newStatus
	project.ProjectPaymentDate = &now

	if err := s.repo.Save(ctx, project); err != nil {
		return nil, err
	}
	if err := s.repo.AppendLog(ctx, &model.LogProject{
		ProjectID: project.ID,
		Username:  username(ctx),
		Action:    "Mengkonfirmasi status pembayaran telah selesai",
	}); err != nil {
		return nil, err
	}

	s.syncLedger(ctx, *project, newStatus)
	return s.detail(ctx, *project)
}

// checkPaymentTransition allows a payment once, and a refund only on a
// cancelled project that was actually paid.
func checkPaymentTransition(project model.Project, next int) error {
	paid := project.ProjectPaymentStatus != nil && *project.ProjectPaymentStatus == model.PaymentTelahLunas

	if next == model.PaymentDikembalikan {
		if project.ProjectStatus != model.StatusBatal {
			return apierr.Invalidf("Pengembalian hanya bisa dilakukan jika proyek sudah dibatalkan.")
		}
		if !paid {
			return apierr.Invalidf("Pengembalian hanya bisa dilakukan jika proyek sudah dibayar.")
		}
		return nil
	}

	if paid {
		return apierr.Invalidf("Proyek sudah dibayar, tidak dapat diubah status pembayarannya kecuali ke pengembalian.")
	}
	return nil
}

// syncLedger tells finance about the money. Failures are logged and swallowed,
// as in Java: the payment is recorded here and finance can be reconciled, so an
// outage there must not fail the confirmation.
func (s *ProjectService) syncLedger(ctx context.Context, project model.Project, newStatus int) {
	switch newStatus {
	case model.PaymentTelahLunas:
		body := dto.AddLapkeuRequest{
			ID:           project.ID,
			ActivityType: activityPenjualan,
			Description:  "Penjualan - " + project.ProjectName,
		}
		if project.ProjectType == model.TypePengiriman {
			body.ActivityType = activityDistribusi
			body.Description = "Distribusi - " + project.ProjectName
		}
		if project.ProjectTotalPemasukkan != nil {
			body.Pemasukan = *project.ProjectTotalPemasukkan
		}
		if project.ProjectTotalPengeluaran != nil {
			body.Pengeluaran = *project.ProjectTotalPengeluaran
		}
		if project.ProjectPaymentDate != nil {
			body.PaymentDate = jsontime.Jakarta(*project.ProjectPaymentDate)
		}

		if err := s.Finance.Post(ctx, "/lapkeu/add", body); err != nil {
			slog.WarnContext(ctx, "could not record the project in the ledger",
				"project", project.ID, "error", err)
		}

	case model.PaymentDikembalikan:
		// A refund removes the ledger entry rather than offsetting it, which is
		// how the legacy service undid one.
		if err := s.Finance.Delete(ctx, "/lapkeu/"+project.ID); err != nil {
			slog.WarnContext(ctx, "could not remove the project from the ledger",
				"project", project.ID, "error", err)
		}
	}
}
