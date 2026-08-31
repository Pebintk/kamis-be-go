package dto

// AddSupplierRequest is the body of POST /api/supplier/add.
type AddSupplierRequest struct {
	NameSupplier    string  `json:"nameSupplier" binding:"required"`
	NoTelpSupplier  string  `json:"noTelpSupplier" binding:"required"`
	EmailSupplier   string  `json:"emailSupplier" binding:"required,email"`
	CompanySupplier string  `json:"companySupplier" binding:"required"`
	AddressSupplier string  `json:"addressSupplier" binding:"required"`
	ResourceIDs     []int64 `json:"resourceIds"`
}

// UpdateSupplierRequest is the body of PUT /api/supplier/update. Note the id
// travels in the body, not the path — that is the legacy contract.
// companySupplier is deliberately absent: the legacy service never updates it.
type UpdateSupplierRequest struct {
	ID              string  `json:"id" binding:"required"`
	AddressSupplier string  `json:"addressSupplier" binding:"required"`
	NoTelpSupplier  string  `json:"noTelpSupplier" binding:"required"`
	EmailSupplier   string  `json:"emailSupplier" binding:"required,email"`
	NameSupplier    string  `json:"nameSupplier" binding:"required"`
	ResourceIDs     []int64 `json:"resourceIds"`
}

// AddPurchaseIDRequest is the body of PUT /api/supplier/add-purchase, called by
// the purchase service to link a purchase back to its supplier.
type AddPurchaseIDRequest struct {
	PurchaseID string `json:"purchaseId"`
	SupplierID string `json:"supplierId"`
}

// AddSupplierIDRequest is the body profile *sends* to the resource service to
// attach or re-attach a supplier to a set of resources.
type AddSupplierIDRequest struct {
	SupplierID string  `json:"supplierId"`
	ResourceID []int64 `json:"resourceId"`
}

// SupplierResponse is the full supplier representation.
type SupplierResponse struct {
	ID              string    `json:"id"`
	NameSupplier    string    `json:"nameSupplier"`
	NoTelpSupplier  string    `json:"noTelpSupplier"`
	EmailSupplier   string    `json:"emailSupplier"`
	CompanySupplier string    `json:"companySupplier"`
	AddressSupplier string    `json:"addressSupplier"`
	ResourceIDs     []int64   `json:"resourceIds"`
	AssetIDs        []string  `json:"assetIds"`
	PurchaseIDs     []string  `json:"purchaseIds"`
	CreatedDate     Timestamp `json:"createdDate"`
	UpdatedDate     Timestamp `json:"updatedDate"`
}

// SupplierListResponse is the row shape for the list and paginated endpoints.
type SupplierListResponse struct {
	ID              string `json:"id"`
	NameSupplier    string `json:"nameSupplier"`
	CompanySupplier string `json:"companySupplier"`
	TotalPurchases  int    `json:"totalPurchases"`
}

// DetailSupplier is the aggregate returned by /api/supplier/detail/{id}: the
// supplier plus its assets, purchases and resources gathered from three other
// services.
type DetailSupplier struct {
	SupplierName    string             `json:"supplierName"`
	SupplierPhone   string             `json:"supplierPhone"`
	SupplierEmail   string             `json:"supplierEmail"`
	SupplierCompany string             `json:"supplierCompany"`
	SupplierAddress string             `json:"supplierAddress"`
	Assets          []Asset            `json:"assets"`
	Purchases       []PurchaseResponse `json:"purchases"`
	Resources       []ResourceResponse `json:"resources"`
}

// Asset is what the asset service returns for a supplier's assets.
type Asset struct {
	PlatNomor      string `json:"platNomor"`
	Nama           string `json:"nama"`
	NilaiPerolehan *int   `json:"nilaiPerolehan"`
}

// ResourceResponse is what the resource service returns.
type ResourceResponse struct {
	ID            int64  `json:"id"`
	ResourceName  string `json:"resourceName"`
	ResourcePrice *int   `json:"resourcePrice"`
}

// PurchaseResponse is what the purchase service returns. ActivityName is not
// sent by that service — profile fills it in for the detail view.
type PurchaseResponse struct {
	PurchaseID             string    `json:"purchaseId"`
	PurchaseSubmissionDate Timestamp `json:"purchaseSubmissionDate"`
	PurchaseStatus         string    `json:"purchaseStatus"`
	PurchaseType           string    `json:"purchaseType"`
	PurchaseNote           string    `json:"purchaseNote"`
	PurchasePrice          *int      `json:"purchasePrice"`
	ActivityName           string    `json:"activityName"`
}
