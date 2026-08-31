package dto

// AddClientRequest is the body of POST /api/client/add.
type AddClientRequest struct {
	NameClient   string `json:"nameClient" binding:"required"`
	NoTelpClient string `json:"noTelpClient" binding:"required"`
	EmailClient  string `json:"emailClient" binding:"required"`
	// TypeClient is false = perorangan, true = perusahaan. The legacy
	// @NotNull on a primitive boolean can never fail, so it is not required here
	// either — an omitted value means false, exactly as in Java.
	TypeClient    bool   `json:"typeClient"`
	CompanyClient string `json:"companyClient"`
	AddressClient string `json:"addressClient"`
}

// UpdateClientRequest is the body of PUT /api/client/update/{id}. Every field is
// @NotBlank in the legacy DTO, but that controller does not check the
// BindingResult, so blank values were never actually rejected — the service only
// applies non-null fields. Pointers reproduce that: omitted stays unchanged.
type UpdateClientRequest struct {
	NameClient    *string `json:"nameClient"`
	NoTelpClient  *string `json:"noTelpClient"`
	EmailClient   *string `json:"emailClient"`
	AddressClient *string `json:"addressClient"`
}

// ClientResponse is the detail representation, including the client's projects
// fetched from the project service.
type ClientResponse struct {
	ID            string            `json:"id"`
	NameClient    string            `json:"nameClient"`
	NoTelpClient  string            `json:"noTelpClient"`
	EmailClient   string            `json:"emailClient"`
	TypeClient    bool              `json:"typeClient"`
	CompanyClient string            `json:"companyClient"`
	AddressClient string            `json:"addressClient"`
	Projects      []ProjectResponse `json:"projects"`
	CreatedDate   Timestamp         `json:"createdDate"`
	UpdatedDate   Timestamp         `json:"updatedDate"`
}

// ClientListResponse is the row shape for the list and paginated endpoints.
type ClientListResponse struct {
	ID            string `json:"id"`
	NameClient    string `json:"nameClient"`
	NoTelpClient  string `json:"noTelpClient"`
	TypeClient    bool   `json:"typeClient"`
	CompanyClient string `json:"companyClient"`
	ProjectCount  int    `json:"projectCount"`
	TotalProfit   int64  `json:"totalProfit"`
}

// ProjectResponse is what the project service returns for a client's projects.
type ProjectResponse struct {
	ID                      string    `json:"id"`
	ProjectName             string    `json:"projectName"`
	ProjectStatus           string    `json:"projectStatus"`
	ProjectTotalPemasukkan  *int64    `json:"projectTotalPemasukkan"`
	ProjectTotalPengeluaran *int64    `json:"projectTotalPengeluaran"`
	ProjectType             *bool     `json:"projectType"`
	ProjectStartDate        Timestamp `json:"projectStartDate"`
	Profit                  *int64    `json:"profit"`
}

// CalculateProfit fills Profit from the two totals, leaving it null when either
// is missing — the Java ProjectResponseDTO.calculateProfit().
func (p *ProjectResponse) CalculateProfit() {
	if p.ProjectTotalPemasukkan != nil && p.ProjectTotalPengeluaran != nil {
		profit := *p.ProjectTotalPemasukkan - *p.ProjectTotalPengeluaran
		p.Profit = &profit
		return
	}
	p.Profit = nil
}
