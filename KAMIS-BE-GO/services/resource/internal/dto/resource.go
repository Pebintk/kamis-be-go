// Package dto holds the request and response bodies of the resource API. The
// JSON field names are the contract the Vue frontend and the other services
// parse, so they match the Java restdto classes exactly.
package dto

import "github.com/karina/kamis-be-go/pkg/page"

// PageOf is the shared Spring-shaped page, re-exported so call sites can take a
// parameter named `page` without shadowing the package.
type PageOf[T any] = page.Of[T]

// NewPage assembles the Spring-shaped page metadata around one page of content.
func NewPage[T any](content []T, number, size int, total int64) PageOf[T] {
	return page.New(content, number, size, total)
}

// AddResourceRequest is the body of POST /api/resource/add.
//
// The numeric fields are pointers because Gin's `required` rejects a zero value
// on a plain int, and a stock or price of 0 is legitimate here. On a pointer,
// `required` only rejects a missing field — which is what the Java @NotNull did.
type AddResourceRequest struct {
	ResourceName        string `json:"resourceName" binding:"required"`
	ResourceDescription string `json:"resourceDescription" binding:"required"`
	ResourceStock       *int   `json:"resourceStock" binding:"required"`
	ResourcePrice       *int   `json:"resourcePrice" binding:"required"`
	ResourceSupplierID  string `json:"resourceSupplierId" binding:"required"`
}

// UpdateResourceRequest is the body of PUT /api/resource/update/{idResource}.
// The name is deliberately absent: the Java DTO has no name field either, so a
// resource cannot be renamed.
type UpdateResourceRequest struct {
	ResourceDescription string `json:"resourceDescription" binding:"required"`
	ResourcePrice       *int   `json:"resourcePrice" binding:"required"`
	ResourceStock       *int   `json:"resourceStock" binding:"required"`
}

// UpdateResourceStockRequest is the body of the add-stock and deduct-stock
// endpoints. The service rejects a non-positive quantity.
type UpdateResourceStockRequest struct {
	Quantity *int `json:"quantity" binding:"required"`
}

// AddSupplierIDRequest is the body of PUT /api/resource/add-supplier and
// /update-supplier. The profile service sends this shape when a supplier's
// resource list is created or edited.
type AddSupplierIDRequest struct {
	SupplierID string  `json:"supplierId" binding:"required"`
	ResourceID []int64 `json:"resourceId"`
}

// ResourceResponse is the single representation every resource endpoint
// returns. It deliberately omits the supplier ids: the Java DTO had that field
// commented out, and the frontend does not read it.
type ResourceResponse struct {
	ID                  int64  `json:"id"`
	ResourceName        string `json:"resourceName"`
	ResourceDescription string `json:"resourceDescription"`
	ResourceStock       int    `json:"resourceStock"`
	ResourcePrice       int    `json:"resourcePrice"`
}
