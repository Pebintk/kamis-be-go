// Package dto holds the request and response bodies of the purchase API. The
// JSON field names are the contract the Vue frontend parses, so they match the
// Java restdto classes exactly.
package dto

import (
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/pkg/page"
)

// PageOf is the shared Spring-shaped page, re-exported so call sites can take a
// parameter named `page` without shadowing the package.
type PageOf[T any] = page.Of[T]

// NewPage assembles the Spring-shaped page metadata around one page of content.
func NewPage[T any](content []T, number, size int, total int64) PageOf[T] {
	return page.New(content, number, size, total)
}

// Timestamp renders dates in Asia/Jakarta. Unlike the asset service, every date
// field in the purchase DTOs carries @JsonFormat(timezone="Asia/Jakarta"), so
// they are all WIB.
type Timestamp = jsontime.Time

// ResourceLineRequest is one line item of a resource purchase.
type ResourceLineRequest struct {
	ResourceID    *int64 `json:"resourceId" binding:"required"`
	ResourceName  string `json:"resourceName" binding:"required"`
	ResourceTotal *int   `json:"resourceTotal" binding:"required"`
	ResourcePrice *int   `json:"resourcePrice" binding:"required"`
}

// AddPurchaseRequest is the body of POST /api/purchase/add.
//
// Exactly one of PurchaseAsset and PurchaseResource applies, chosen by
// PurchaseType: false means an asset purchase and requires PurchaseAsset, true
// means resources and requires a non-empty PurchaseResource. Supplying the
// wrong one is rejected.
type AddPurchaseRequest struct {
	PurchaseSupplier string `json:"purchaseSupplier" binding:"required"`
	// PurchaseType has @NotNull on a primitive boolean in the legacy DTO, which
	// can never fail, so it is not required here either: an omitted value means
	// false (an asset purchase), exactly as in Java.
	PurchaseType     bool                  `json:"purchaseType"`
	PurchaseAsset    *int64                `json:"purchaseAsset"`
	PurchaseResource []ResourceLineRequest `json:"purchaseResource"`
	PurchaseNote     string                `json:"purchaseNote"`
}

// UpdatePurchaseRequest is the body of PUT /api/purchase/update/{purchaseId}.
// It carries no purchaseType: the type is fixed at creation.
type UpdatePurchaseRequest struct {
	PurchaseSupplier string                `json:"purchaseSupplier" binding:"required"`
	PurchaseResource []ResourceLineRequest `json:"purchaseResource"`
	PurchaseNote     string                `json:"purchaseNote"`
}

// ResourceLineResponse is one line item as returned.
type ResourceLineResponse struct {
	ResourceID    int64  `json:"resourceId"`
	ResourceName  string `json:"resourceName"`
	ResourceTotal int    `json:"resourceTotal"`
	ResourcePrice int    `json:"resourcePrice"`
}

// AssetTempResponse is the staged asset of an asset purchase. The misspelled
// `assetNameString` field is the legacy name and the frontend reads it.
type AssetTempResponse struct {
	ID               int64   `json:"id"`
	AssetNameString  string  `json:"assetNameString"`
	AssetDescription string  `json:"assetDescription"`
	AssetType        string  `json:"assetType"`
	AssetPrice       int     `json:"assetPrice"`
	FotoContentType  *string `json:"fotoContentType"`
	FotoURL          *string `json:"fotoUrl"`
}

// LogResponse is one audit-trail entry.
type LogResponse struct {
	ID         string    `json:"id"`
	User       string    `json:"user"`
	Action     string    `json:"action"`
	ActionDate Timestamp `json:"actionDate"`
}

// PurchaseResponse is the detail representation. PurchaseResource and
// PurchaseAsset are mutually exclusive: whichever does not apply to this
// purchase's type is null, as in Java.
type PurchaseResponse struct {
	PurchaseID             string                 `json:"purchaseId"`
	PurchaseSubmissionDate Timestamp              `json:"purchaseSubmissionDate"`
	PurchaseUpdateDate     Timestamp              `json:"purchaseUpdateDate"`
	PurchaseSupplier       string                 `json:"purchaseSupplier"`
	PurchaseType           string                 `json:"purchaseType"`
	PurchaseStatus         string                 `json:"purchaseStatus"`
	PurchaseResource       []ResourceLineResponse `json:"purchaseResource"`
	PurchaseAsset          *AssetTempResponse     `json:"purchaseAsset"`
	PurchasePrice          int                    `json:"purchasePrice"`
	PurchaseNote           string                 `json:"purchaseNote"`
	PurchasePaymentDate    *Timestamp             `json:"purchasePaymentDate"`
	PurchaseLogs           []LogResponse          `json:"purchaseLogs"`
}

// PurchaseListResponse is the row shape for the list and paginated endpoints.
// PurchaseSupplier here is the supplier's *name*, resolved from the profile
// service, where the detail response carries the id.
type PurchaseListResponse struct {
	PurchaseID             string     `json:"purchaseId"`
	PurchaseSubmissionDate Timestamp  `json:"purchaseSubmissionDate"`
	PurchaseUpdateDate     Timestamp  `json:"purchaseUpdateDate"`
	PurchaseSupplier       string     `json:"purchaseSupplier"`
	PurchaseType           string     `json:"purchaseType"`
	PurchaseStatus         string     `json:"purchaseStatus"`
	PurchasePrice          int        `json:"purchasePrice"`
	PurchasePaymentDate    *Timestamp `json:"purchasePaymentDate"`
}

// AddPurchaseIDRequest is the body purchase *sends* to the profile service to
// attach a new purchase to its supplier.
type AddPurchaseIDRequest struct {
	PurchaseID string `json:"purchaseId"`
	SupplierID string `json:"supplierId"`
}

// ResourceResponse is what the resource service returns for one catalogue item.
// Only the name is checked here, against the name the caller submitted.
type ResourceResponse struct {
	ID                  int64  `json:"id"`
	ResourceName        string `json:"resourceName"`
	ResourceDescription string `json:"resourceDescription"`
	ResourceStock       int    `json:"resourceStock"`
	ResourcePrice       int    `json:"resourcePrice"`
}
