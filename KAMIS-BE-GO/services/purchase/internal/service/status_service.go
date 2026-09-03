package service

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/services/purchase/internal/dto"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
)

// purchaseActivityType is the ledger's code for a purchase, from the legacy
// AddLapkeuDTO call site.
const purchaseActivityType = 2

// acquisitionDateLayout is the format the asset service parses for
// tanggalPerolehan.
const acquisitionDateLayout = "2006-01-02"

// transition is one edge of the purchase status machine: who may take it, where
// it leads, and what to say when the caller's role is wrong.
//
// The role rules live here rather than on the route because which role may act
// depends on the purchase's *current* status — Direksi and Finance approve a
// submission, Operasional processes and completes it — so a route-level guard
// cannot express them.
type transition struct {
	to      string
	allowed []string
	refusal string
}

// advance is the /updatestatus/next machine.
var advance = map[string]transition{
	model.StatusDiajukan: {
		to:      model.StatusDisetujui,
		allowed: []string{"Direksi", "Finance"},
		refusal: "Hanya Direksi atau Finance yang dapat menyetujui pembelian.",
	},
	model.StatusDisetujui: {
		to:      model.StatusDiproses,
		allowed: []string{"Operasional", "Admin"},
		refusal: "Hanya Operasional yang dapat memproses pembelian.",
	},
	model.StatusDiproses: {
		to:      model.StatusSelesai,
		allowed: []string{"Operasional", "Admin"},
		refusal: "Hanya Operasional atau Admin yang dapat menyelesaikan pembelian.",
	},
}

// abandon is the /updatestatus/cancel machine. Note that cancelling at Diproses
// excludes Admin, while *completing* at Diproses allows it — an asymmetry
// inherited verbatim from the legacy service.
var abandon = map[string]transition{
	model.StatusDiajukan: {
		to:      model.StatusDitolak,
		allowed: []string{"Direksi", "Finance"},
		refusal: "Hanya Direksi atau Finance yang dapat menolak pembelian.",
	},
	model.StatusDisetujui: {
		to:      model.StatusDibatalkan,
		allowed: []string{"Operasional", "Admin"},
		refusal: "Hanya Operasional atau Admin yang dapat membatalkan pembelian di tahap Disetujui.",
	},
	model.StatusDiproses: {
		to:      model.StatusDibatalkan,
		allowed: []string{"Operasional"},
		refusal: "Hanya Operasional yang dapat membatalkan pembelian di tahap Diproses.",
	},
}

// role is the caller's role from the validated token.
func role(ctx context.Context) string {
	if claims, ok := auth.FromContext(ctx); ok {
		return claims.Role
	}
	return ""
}

// resolve loads the purchase, refuses a terminal status, and picks the
// transition the caller is asking for — the preamble both /next and /cancel run.
func (s *PurchaseService) resolve(ctx context.Context, purchaseID string, note *string, machine map[string]transition) (*model.Purchase, transition, error) {
	var none transition

	if note == nil {
		return nil, none, apierr.Invalidf("Catatan tidak boleh kosong")
	}

	purchase, err := s.repo.FindByID(ctx, purchaseID)
	if err != nil {
		return nil, none, notFound(err, purchaseID)
	}
	if model.Terminal(purchase.PurchaseStatus) {
		return nil, none, apierr.Invalidf("Status Pembelian sudah tidak bisa diperbarui.")
	}

	step, ok := machine[purchase.PurchaseStatus]
	if !ok {
		// Unreachable for the six known statuses, but a status written by an
		// older version must not silently no-op into a success response.
		return nil, none, apierr.Invalidf("Status Pembelian %s tidak dapat diperbarui.", purchase.PurchaseStatus)
	}
	if !slices.Contains(step.allowed, role(ctx)) {
		return nil, none, &apierr.Invalid{Message: step.refusal}
	}
	return purchase, step, nil
}

// AdvanceStatus moves a purchase to its next state, running the side effects
// that completing one entails.
func (s *PurchaseService) AdvanceStatus(ctx context.Context, purchaseID string, req dto.UpdateStatusRequest) (*dto.PurchaseResponse, error) {
	purchase, step, err := s.resolve(ctx, purchaseID, req.PurchaseNote, advance)
	if err != nil {
		return nil, err
	}

	// Completing the purchase is where it stops being paperwork: resources are
	// credited to the catalogue and an asset is registered for real. Both happen
	// before the status is written, so a failed handoff leaves the purchase in
	// Diproses to be retried rather than Selesai with nothing delivered.
	if step.to == model.StatusSelesai {
		if err := s.deliver(ctx, purchase, req.PlatNomor); err != nil {
			return nil, err
		}
	}

	return s.applyStatus(ctx, purchase, step.to, *req.PurchaseNote,
		"Mengubah Status Pembelian menjadi "+step.to)
}

// CancelStatus rejects or cancels a purchase, depending on how far it had got.
func (s *PurchaseService) CancelStatus(ctx context.Context, purchaseID string, req dto.UpdateStatusRequest) (*dto.PurchaseResponse, error) {
	purchase, step, err := s.resolve(ctx, purchaseID, req.PurchaseNote, abandon)
	if err != nil {
		return nil, err
	}
	return s.applyStatus(ctx, purchase, step.to, *req.PurchaseNote,
		"Mengubah Status Pembelian menjadi "+step.to)
}

// applyStatus writes the new status, note and audit entry.
func (s *PurchaseService) applyStatus(ctx context.Context, purchase *model.Purchase, status, note, action string) (*dto.PurchaseResponse, error) {
	purchase.PurchaseStatus = status
	purchase.PurchaseNote = note

	if err := s.repo.Save(ctx, purchase); err != nil {
		return nil, err
	}
	if err := s.repo.AppendLog(ctx, &model.LogPurchase{
		PurchaseID: purchase.ID,
		Username:   username(ctx),
		Action:     action,
	}); err != nil {
		return nil, err
	}
	return s.GetPurchase(ctx, purchase.ID)
}

// deliver hands a completed purchase to the service that owns what was bought.
func (s *PurchaseService) deliver(ctx context.Context, purchase *model.Purchase, platNomor string) error {
	if purchase.PurchaseType == model.TypeResource {
		return s.creditResources(ctx, purchase.ID)
	}
	return s.registerAsset(ctx, purchase, platNomor)
}

// creditResources adds each line item's quantity to the resource catalogue's
// stock.
func (s *PurchaseService) creditResources(ctx context.Context, purchaseID string) error {
	lines, err := s.repo.FindLinesFor(ctx, []string{purchaseID})
	if err != nil {
		return err
	}
	for _, line := range lines[purchaseID] {
		path := "/resource/addToDb/" + itoa(line.ResourceID) + "/" + itoa(int64(line.ResourceTotal))
		if err := s.Resource.Put(ctx, path, nil); err != nil {
			return apierr.Invalidf("Gagal menambahkan stok untuk %s: %v", line.ResourceName, err)
		}
	}
	return nil
}

// registerAsset asks the asset service to turn the staged asset into a real one,
// forwarding its photo as a multipart file part — the same shape the legacy
// service sent.
func (s *PurchaseService) registerAsset(ctx context.Context, purchase *model.Purchase, platNomor string) error {
	if platNomor == "" {
		return apierr.Invalidf("Plat Nomor tidak boleh kosong")
	}
	if purchase.PurchaseAsset == nil {
		return &apierr.Invalid{Message: "Aset tidak ditemukan dalam database."}
	}

	staged, err := s.repo.FindAssetTempByID(ctx, *purchase.PurchaseAsset)
	if err != nil {
		return assetNotFound(err)
	}

	fields := map[string]string{
		"platNomor":        platNomor,
		"assetName":        staged.AssetName,
		"assetDescription": staged.AssetDescription,
		"assetType":        staged.AssetType,
		"assetPrice":       itoa(int64(staged.AssetPrice)),
		"tanggalPerolehan": time.Now().Format(acquisitionDateLayout),
		"supplierId":       purchase.PurchaseSupplier,
		"status":           "Tersedia",
	}

	// The photo is streamed from our blob store straight into the outgoing
	// request. Java passed the *filename* and let the asset service look it up
	// on a disk they happened to share — which stopped being true the moment
	// either service moved.
	var part *httpx.FilePart
	if staged.FotoKey != "" && s.Photos != nil {
		object, openErr := s.Photos.Get(ctx, staged.FotoKey)
		if openErr != nil {
			slog.WarnContext(ctx, "staged asset photo is missing; registering without it",
				"asset", staged.ID, "key", staged.FotoKey, "error", openErr)
		} else {
			defer func() { _ = object.Body.Close() }()
			part = &httpx.FilePart{
				Field:       "foto",
				Filename:    staged.FotoKey,
				ContentType: object.ContentType,
				Body:        object.Body,
			}
		}
	}

	if err := s.Asset.PostForm(ctx, "/asset/addAsset", fields, part); err != nil {
		return apierr.Invalidf("Gagal mendaftarkan aset: %v", err)
	}
	return nil
}

// ConfirmPayment records that a purchase has been paid and posts the spend to
// the ledger. Finance alone may do this.
func (s *PurchaseService) ConfirmPayment(ctx context.Context, purchaseID string, req dto.UpdateStatusRequest) (*dto.PurchaseResponse, error) {
	if req.PurchaseNote == nil {
		return nil, apierr.Invalidf("Catatan tidak boleh kosong")
	}
	if role(ctx) != "Finance" {
		return nil, apierr.Invalidf("Hanya Finance yang dapat mengupdate status pembayaran.")
	}

	purchase, err := s.repo.FindByID(ctx, purchaseID)
	if err != nil {
		return nil, notFound(err, purchaseID)
	}
	if purchase.PurchasePaymentDate != nil {
		return nil, apierr.Invalidf("Pembayaran sudah dilakukan pada tanggal %s.",
			purchase.PurchasePaymentDate.Format(acquisitionDateLayout))
	}

	// Payment is only meaningful once the goods are in motion. Each rejected
	// status has its own message, which the frontend shows verbatim.
	switch purchase.PurchaseStatus {
	case model.StatusDibatalkan:
		return nil, apierr.Invalidf("Status Pembayaran tidak bisa diperbarui karena pembelian telah dibatalkan.")
	case model.StatusDiajukan:
		return nil, apierr.Invalidf("Status Pembayaran tidak bisa diperbarui karena pembelian belum disetujui.")
	case model.StatusDisetujui:
		return nil, apierr.Invalidf("Status Pembayaran tidak bisa diperbarui karena pembelian belum diproses.")
	case model.StatusDitolak:
		return nil, apierr.Invalidf("Status Pembayaran tidak bisa diperbarui karena pembelian telah ditolak.")
	}

	now := time.Now()
	purchase.PurchaseNote = *req.PurchaseNote
	purchase.PurchasePaymentDate = &now

	if err := s.repo.Save(ctx, purchase); err != nil {
		return nil, err
	}
	s.reportToLedger(ctx, *purchase)

	if err := s.repo.AppendLog(ctx, &model.LogPurchase{
		PurchaseID: purchase.ID,
		Username:   username(ctx),
		Action:     "Mengkonfirmasi status pembayaran telah selesai",
	}); err != nil {
		return nil, err
	}
	return s.GetPurchase(ctx, purchase.ID)
}

// reportToLedger posts the spend to finance. A failure is logged and swallowed,
// as in Java: the payment is recorded here and finance can be reconciled, so an
// outage there must not fail the confirmation.
func (s *PurchaseService) reportToLedger(ctx context.Context, purchase model.Purchase) {
	body := dto.AddLapkeuRequest{
		ID:           purchase.ID,
		ActivityType: purchaseActivityType,
		Pemasukan:    0,
		Pengeluaran:  int64(purchase.PurchasePrice),
		Description:  "Pembelian - " + s.ledgerItemName(ctx, purchase),
	}
	if purchase.PurchasePaymentDate != nil {
		body.PaymentDate = jsontime.Jakarta(*purchase.PurchasePaymentDate)
	}

	if err := s.Finance.Post(ctx, "/lapkeu/add", body); err != nil {
		slog.WarnContext(ctx, "could not record purchase in the ledger",
			"purchase", purchase.ID, "error", err)
	}
}

// ledgerItemName is what the ledger entry is called: the asset's name, or the
// first line item's for a resource purchase. Naming only the first of several
// lines is the legacy behaviour, kept so existing ledger entries stay
// consistent with new ones.
func (s *PurchaseService) ledgerItemName(ctx context.Context, purchase model.Purchase) string {
	if purchase.PurchaseType == model.TypeResource {
		lines, err := s.repo.FindLinesFor(ctx, []string{purchase.ID})
		if err != nil || len(lines[purchase.ID]) == 0 {
			return ""
		}
		return lines[purchase.ID][0].ResourceName
	}
	if purchase.PurchaseAsset == nil {
		return ""
	}
	staged, err := s.repo.FindAssetTempByID(ctx, *purchase.PurchaseAsset)
	if err != nil {
		return ""
	}
	return staged.AssetName
}
