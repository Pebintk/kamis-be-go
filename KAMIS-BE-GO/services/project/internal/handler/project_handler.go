// Package handler is the transport layer (Spring @RestController).
//
// Errors go through httpx.RespondError, so every endpoint answers 400 for a
// caller mistake, 404 for a missing project and 500 for anything else.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/project/internal/dto"
	"github.com/karina/kamis-be-go/services/project/internal/repository"
	"github.com/karina/kamis-be-go/services/project/internal/service"
)

// filterDateLayout is the format the legacy @DateTimeFormat declared for the
// tanggalMulai and tanggalSelesai query parameters.
const filterDateLayout = "2006-01-02"

type ProjectHandler struct{ svc *service.ProjectService }

func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// Add handles POST /api/project/add.
func (h *ProjectHandler) Add(c *gin.Context) {
	var req dto.AddProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	project, err := h.svc.AddProject(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Proyek berhasil ditambahkan", project)
}

// Detail handles GET /api/project/{id}.
func (h *ProjectHandler) Detail(c *gin.Context) {
	project, err := h.svc.GetProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Proyek berhasil ditemukan", project)
}

// All handles GET /api/project/all.
func (h *ProjectHandler) All(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	projects, err := h.svc.ListProjects(c.Request.Context(), filter)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Daftar proyek berhasil ditemukan", projects)
}

// Paginated handles GET /api/project/all/paginated.
func (h *ProjectHandler) Paginated(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	number, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	result, err := h.svc.ListProjectsPaginated(c.Request.Context(), filter, number, size)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Daftar proyek berhasil ditemukan", result)
}

// parseFilter reads the list query parameters, whose names are Indonesian and
// do not match the fields they filter.
func parseFilter(c *gin.Context) (repository.Filter, error) {
	f := repository.Filter{
		IDSearch:        c.Query("idProject"),
		ProjectName:     c.Query("namaProject"),
		ProjectClientID: c.Query("clientProject"),
	}

	var err error
	if f.ProjectStatus, err = queryInt(c, "statusProject"); err != nil {
		return f, err
	}
	if f.ProjectType, err = queryBool(c, "tipeProject"); err != nil {
		return f, err
	}
	if f.StartDate, err = queryDate(c, "tanggalMulai"); err != nil {
		return f, err
	}
	if f.EndDate, err = queryDate(c, "tanggalSelesai"); err != nil {
		return f, err
	}
	if f.StartNominal, err = queryInt64(c, "startNominal"); err != nil {
		return f, err
	}
	if f.EndNominal, err = queryInt64(c, "endNominal"); err != nil {
		return f, err
	}
	return f, nil
}

func queryInt(c *gin.Context, name string) (*int, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, badQuery(name, raw, "angka")
	}
	return &v, nil
}

func queryInt64(c *gin.Context, name string) (*int64, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, badQuery(name, raw, "angka")
	}
	return &v, nil
}

func queryBool(c *gin.Context, name string) (*bool, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, badQuery(name, raw, "true atau false")
	}
	return &v, nil
}

func queryDate(c *gin.Context, name string) (*time.Time, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := time.Parse(filterDateLayout, raw)
	if err != nil {
		return nil, badQuery(name, raw, "tanggal yyyy-MM-dd")
	}
	return &v, nil
}

// badQuery reports an unparseable query parameter as a caller mistake.
func badQuery(name, value, want string) error {
	return apierr.Invalidf("Parameter %s tidak valid (%q): harus berupa %s", name, value, want)
}
