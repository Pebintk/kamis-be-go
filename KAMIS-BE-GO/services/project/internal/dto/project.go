// Package dto holds the request and response bodies of the project API. The
// JSON field names are the contract the Vue frontend parses, so they match the
// Java restdto classes exactly — including the lower-cased
// listProjectResponseDTO and the wrapper shape around a project's detail.
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

// Timestamp renders dates in Asia/Jakarta, which every @JsonFormat in this
// service pins them to.
type Timestamp = jsontime.Time

// AssetUsage is one vehicle booked to a distribution.
type AssetUsage struct {
	TipeAset      string `json:"tipeAset"`
	PlatNomor     string `json:"platNomor"`
	AssetUseCost  *int   `json:"assetUseCost"`
	AssetFuelCost *int   `json:"assetFuelCost"`
}

// ResourceUsage is one catalogue item consumed by a sale.
type ResourceUsage struct {
	ResourceID        string `json:"resourceId"`
	SellPrice         *int   `json:"sellPrice"`
	ResourceStockUsed *int   `json:"resourceStockUsed"`
}

// AddProjectRequest is the body of POST /api/project/add.
//
// ProjectUseAsset applies to a distribution and ProjectUseResource to a sale;
// which one is read follows ProjectType.
type AddProjectRequest struct {
	ProjectType        *bool  `json:"projectType" binding:"required"`
	ProjectName        string `json:"projectName" binding:"required"`
	ProjectDescription string `json:"projectDescription"`
	ProjectClientID    string `json:"projectClientId" binding:"required"`

	ProjectUseAsset    []AssetUsage    `json:"projectUseAsset"`
	ProjectUseResource []ResourceUsage `json:"projectUseResource"`

	ProjectDeliveryAddress string `json:"projectDeliveryAddress" binding:"required"`
	ProjectPickupAddress   string `json:"projectPickupAddress"`
	ProjectPHLCount        *int   `json:"projectPHLCount"`
	ProjectPHLPay          *int64 `json:"projectPHLPay"`

	// The dates are required here, where the legacy DTO left them optional and
	// then dereferenced both: an omitted date raised a NullPointerException and
	// a 500.
	ProjectStartDate *Timestamp `json:"projectStartDate" binding:"required"`
	ProjectEndDate   *Timestamp `json:"projectEndDate" binding:"required"`

	// ProjectTotalPemasukkan is supplied for a distribution; for a sale it is
	// computed from the resources used and anything sent is ignored.
	ProjectTotalPemasukkan *int64 `json:"projectTotalPemasukkan"`
}

// UpdateProjectRequest is the body of PUT /api/project/update/{id}. The id in
// the path is authoritative; the one in the body is the legacy shape.
type UpdateProjectRequest struct {
	ID                 string          `json:"id"`
	ProjectStatus      *int            `json:"projectStatus"`
	ProjectDescription string          `json:"projectDescription"`
	ProjectUseAsset    []AssetUsage    `json:"projectUseAsset"`
	ProjectUseResource []ResourceUsage `json:"projectUseResource"`

	ProjectDeliveryAddress string `json:"projectDeliveryAddress" binding:"required"`
	ProjectPickupAddress   string `json:"projectPickupAddress"`
	ProjectPHLCount        *int   `json:"projectPHLCount"`
	ProjectPHLPay          *int64 `json:"projectPHLPay"`

	ProjectStartDate       *Timestamp `json:"projectStartDate"`
	ProjectEndDate         *Timestamp `json:"projectEndDate"`
	ProjectTotalPemasukkan *int64     `json:"projectTotalPemasukkan"`
}

// UpdateStatusRequest is the body of PUT /api/project/update-status/{id}.
type UpdateStatusRequest struct {
	ProjectStatus *int `json:"projectStatus" binding:"required"`
}

// UpdatePaymentRequest is the body of PUT /api/project/update-payment/{id}.
type UpdatePaymentRequest struct {
	ProjectPaymentStatus *int `json:"projectPaymentStatus" binding:"required"`
}

// LogResponse is one audit-trail entry.
type LogResponse struct {
	ID         string    `json:"id"`
	User       string    `json:"user"`
	Action     string    `json:"action"`
	ActionDate Timestamp `json:"actionDate"`
}

// SellResponse is a sale's detail. It carries no outgoings: a sale draws on
// stock already paid for.
type SellResponse struct {
	ID                     string          `json:"id"`
	ProjectType            bool            `json:"projectType"`
	ProjectPaymentStatus   *int            `json:"projectPaymentStatus"`
	ProjectStatus          int             `json:"projectStatus"`
	ProjectName            string          `json:"projectName"`
	ProjectDescription     string          `json:"projectDescription"`
	ProjectClientID        string          `json:"projectClientId"`
	ProjectClientName      string          `json:"projectClientName"`
	ProjectDeliveryAddress string          `json:"projectDeliveryAddress"`
	ProjectUseResource     []ResourceUsage `json:"projectUseResource"`
	ProjectStartDate       *Timestamp      `json:"projectStartDate"`
	ProjectEndDate         *Timestamp      `json:"projectEndDate"`
	ProjectTotalPemasukkan *int64          `json:"projectTotalPemasukkan"`
	ProjectPaymentDate     *Timestamp      `json:"projectPaymentDate"`
	ProjectLogs            []LogResponse   `json:"projectLogs"`
}

// DistributionResponse is a distribution's detail, including the vehicles used
// and what the job cost.
type DistributionResponse struct {
	ID                      string        `json:"id"`
	ProjectType             bool          `json:"projectType"`
	ProjectPaymentStatus    *int          `json:"projectPaymentStatus"`
	ProjectStatus           int           `json:"projectStatus"`
	ProjectName             string        `json:"projectName"`
	ProjectDescription      string        `json:"projectDescription"`
	ProjectClientID         string        `json:"projectClientId"`
	ProjectClientName       string        `json:"projectClientName"`
	ProjectUseAsset         []AssetUsage  `json:"projectUseAsset"`
	ProjectDeliveryAddress  string        `json:"projectDeliveryAddress"`
	ProjectPickupAddress    string        `json:"projectPickupAddress"`
	ProjectPHLCount         *int          `json:"projectPHLCount"`
	ProjectPHLPay           *int64        `json:"projectPHLPay"`
	ProjectStartDate        *Timestamp    `json:"projectStartDate"`
	ProjectEndDate          *Timestamp    `json:"projectEndDate"`
	ProjectTotalPemasukkan  *int64        `json:"projectTotalPemasukkan"`
	ProjectTotalPengeluaran *int64        `json:"projectTotalPengeluaran"`
	ProjectLogs             []LogResponse `json:"projectLogs"`
	ProjectPaymentDate      *Timestamp    `json:"projectPaymentDate"`
}

// ProjectResponseWrapper is what the detail endpoints return: the kind, plus a
// body whose shape follows it. The frontend switches on projectType to decide
// how to read data.
type ProjectResponseWrapper struct {
	ProjectType bool `json:"projectType"`
	Data        any  `json:"data"`
}

// WrapSell and WrapDistribution build the wrapper for each kind.
func WrapSell(sell SellResponse) ProjectResponseWrapper {
	return ProjectResponseWrapper{ProjectType: false, Data: sell}
}

func WrapDistribution(distribution DistributionResponse) ProjectResponseWrapper {
	return ProjectResponseWrapper{ProjectType: true, Data: distribution}
}

// ProjectListResponse is the row shape for the list and paginated endpoints. It
// is flat across both kinds, unlike the detail.
type ProjectListResponse struct {
	ID                      string     `json:"id"`
	ProjectType             bool       `json:"projectType"`
	ProjectStatus           int        `json:"projectStatus"`
	ProjectPaymentStatus    *int       `json:"projectPaymentStatus"`
	ProjectName             string     `json:"projectName"`
	ProjectDescription      string     `json:"projectDescription"`
	ProjectClientID         string     `json:"projectClientId"`
	ProjectClientName       string     `json:"projectClientName"`
	ProjectStartDate        *Timestamp `json:"projectStartDate"`
	ProjectEndDate          *Timestamp `json:"projectEndDate"`
	ProjectTotalPemasukkan  *int64     `json:"projectTotalPemasukkan"`
	ProjectTotalPengeluaran *int64     `json:"projectTotalPengeluaran"`
	ProjectProfit           *int64     `json:"projectProfit"`
	ProjectPaymentDate      *Timestamp `json:"projectPaymentDate"`
}

// ---- cross-service shapes ----

// ClientDetail is what the profile service returns for a client.
type ClientDetail struct {
	ID         string `json:"id"`
	NameClient string `json:"nameClient"`
}

// ResourceDetail is what the resource service returns for a catalogue item.
type ResourceDetail struct {
	ID            int64  `json:"id"`
	ResourceName  string `json:"resourceName"`
	ResourceStock int    `json:"resourceStock"`
	ResourcePrice int    `json:"resourcePrice"`
}

// AssetDetail is what the asset service returns for a vehicle.
type AssetDetail struct {
	PlatNomor string `json:"platNomor"`
	Nama      string `json:"nama"`
	JenisAset string `json:"jenisAset"`
	Status    string `json:"status"`
}

// AssetAvailabilityRequest asks the asset service whether vehicles are free for
// a window. ExcludeProjectID lets a project's own bookings not count against it
// when its dates are edited.
type AssetAvailabilityRequest struct {
	PlatNomors       []string  `json:"platNomors"`
	StartDate        Timestamp `json:"startDate"`
	EndDate          Timestamp `json:"endDate"`
	ExcludeProjectID string    `json:"excludeProjectId,omitempty"`
}

// AssetReservationRequest books vehicles to a project for a window.
type AssetReservationRequest struct {
	PlatNomors []string  `json:"platNomors"`
	ProjectID  string    `json:"projectId"`
	StartDate  Timestamp `json:"startDate"`
	EndDate    Timestamp `json:"endDate"`
}

// AddLapkeuRequest is the body project sends to finance when a project is paid.
type AddLapkeuRequest struct {
	ID           string    `json:"id"`
	ActivityType int       `json:"activityType"`
	Pemasukan    int64     `json:"pemasukan"`
	Pengeluaran  int64     `json:"pengeluaran"`
	Description  string    `json:"description"`
	PaymentDate  Timestamp `json:"paymentDate"`
}
