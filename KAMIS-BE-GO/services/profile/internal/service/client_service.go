package service

import (
	"context"
	"errors"
	"log"
	"net/url"
	"sync"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/profile/internal/dto"
	"github.com/pebintk/kamis-be-go/services/profile/internal/model"
	"github.com/pebintk/kamis-be-go/services/profile/internal/repository"
)

// ErrClientNotFound is returned for an unknown client id (→404).
var ErrClientNotFound = errors.New("client not found")

// projectFetchConcurrency bounds the fan-out when a list endpoint needs every
// client's projects. The Java service fetched them one client at a time, which
// made a page of N clients take N round trips end to end; the JSON is identical
// either way.
const projectFetchConcurrency = 8

type ClientService struct {
	repo    *repository.ClientRepository
	project *httpx.Client
}

func NewClientService(repo *repository.ClientRepository, project *httpx.Client) *ClientService {
	return &ClientService{repo: repo, project: project}
}

// fetchProjects returns the client's projects from the project service with
// their profit computed. Like the Java version it never fails the caller: a
// timeout, a 404 or an unreachable service all yield an empty list.
func (s *ClientService) fetchProjects(ctx context.Context, clientID string) []dto.ProjectResponse {
	path := "/project/all?clientProject=" + url.QueryEscape(clientID)
	projects, err := httpx.GetData[[]dto.ProjectResponse](ctx, s.project, path)
	if err != nil {
		if !errors.Is(err, httpx.ErrNotFound) {
			log.Printf("client %s: fetching projects: %v", clientID, err)
		}
		return []dto.ProjectResponse{}
	}
	for i := range projects {
		projects[i].CalculateProfit()
	}
	return projects
}

// fetchProjectsFor resolves the projects of many clients concurrently.
func (s *ClientService) fetchProjectsFor(ctx context.Context, clients []model.Client) map[string][]dto.ProjectResponse {
	out := make(map[string][]dto.ProjectResponse, len(clients))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, projectFetchConcurrency)

	for _, c := range clients {
		// Acquire before spawning so the semaphore bounds live goroutines, not
		// just the work they do.
		sem <- struct{}{}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()

			projects := s.fetchProjects(ctx, id)
			mu.Lock()
			out[id] = projects
			mu.Unlock()
		}(c.ID)
	}
	wg.Wait()
	return out
}

func toClientResponse(c *model.Client, projects []dto.ProjectResponse) dto.ClientResponse {
	if projects == nil {
		projects = []dto.ProjectResponse{}
	}
	return dto.ClientResponse{
		ID:            c.ID,
		NameClient:    c.NameClient,
		NoTelpClient:  c.NoTelpClient,
		EmailClient:   c.EmailClient,
		TypeClient:    c.TypeClient,
		CompanyClient: c.CompanyClient,
		AddressClient: c.AddressClient,
		Projects:      projects,
		CreatedDate:   dto.JakartaTime(c.CreatedAt),
		UpdatedDate:   dto.JakartaTime(c.UpdatedAt),
	}
}

func toClientListResponse(c *model.Client, projects []dto.ProjectResponse) dto.ClientListResponse {
	var totalProfit int64
	for _, p := range projects {
		if p.Profit != nil {
			totalProfit += *p.Profit
		}
	}
	return dto.ClientListResponse{
		ID:            c.ID,
		NameClient:    c.NameClient,
		NoTelpClient:  c.NoTelpClient,
		TypeClient:    c.TypeClient,
		CompanyClient: c.CompanyClient,
		ProjectCount:  len(projects),
		TotalProfit:   totalProfit,
	}
}

func (s *ClientService) AddClient(ctx context.Context, req dto.AddClientRequest) (dto.ClientResponse, error) {
	client := &model.Client{
		NameClient:    req.NameClient,
		NoTelpClient:  req.NoTelpClient,
		EmailClient:   req.EmailClient,
		TypeClient:    req.TypeClient,
		CompanyClient: req.CompanyClient,
		AddressClient: req.AddressClient,
	}
	if err := s.repo.Create(ctx, client); err != nil {
		return dto.ClientResponse{}, err
	}
	return toClientResponse(client, s.fetchProjects(ctx, client.ID)), nil
}

func (s *ClientService) GetClientByID(ctx context.Context, id string) (dto.ClientResponse, error) {
	client, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return dto.ClientResponse{}, ErrClientNotFound
	}
	if err != nil {
		return dto.ClientResponse{}, err
	}
	return toClientResponse(client, s.fetchProjects(ctx, client.ID)), nil
}

// UpdateClient applies the non-nil fields. typeClient and companyClient are not
// updatable, matching the legacy service.
func (s *ClientService) UpdateClient(ctx context.Context, id string, req dto.UpdateClientRequest) (dto.ClientResponse, error) {
	client, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return dto.ClientResponse{}, ErrClientNotFound
	}
	if err != nil {
		return dto.ClientResponse{}, err
	}
	if req.NameClient != nil {
		client.NameClient = *req.NameClient
	}
	if req.NoTelpClient != nil {
		client.NoTelpClient = *req.NoTelpClient
	}
	if req.EmailClient != nil {
		client.EmailClient = *req.EmailClient
	}
	if req.AddressClient != nil {
		client.AddressClient = *req.AddressClient
	}
	if err := s.repo.Save(ctx, client); err != nil {
		return dto.ClientResponse{}, err
	}
	return toClientResponse(client, s.fetchProjects(ctx, client.ID)), nil
}

// FilterClients lists clients, optionally narrowed by name, type and a profit
// range. minProfit/maxProfit are applied in memory because the profit comes from
// the project service, not the database.
func (s *ClientService) FilterClients(ctx context.Context, nameClient string, typeClient *bool, minProfit, maxProfit *int64) ([]dto.ClientListResponse, error) {
	clients, err := s.repo.FindFiltered(ctx, nameClient, typeClient)
	if err != nil {
		return nil, err
	}
	projects := s.fetchProjectsFor(ctx, clients)

	out := make([]dto.ClientListResponse, 0, len(clients))
	for i := range clients {
		row := toClientListResponse(&clients[i], projects[clients[i].ID])
		if withinProfit(row.TotalProfit, minProfit, maxProfit) {
			out = append(out, row)
		}
	}
	return out, nil
}

// FilterClientsPaginated is the paginated counterpart.
//
// Divergence from the legacy service: its filterClientsPaginated replaced
// clients outside the profit range with null instead of removing them, so the
// page's content array came back padded with nulls. Here they are dropped. The
// page metadata still describes the unfiltered query, exactly as in Java, since
// the profit range cannot be expressed in the SQL count.
func (s *ClientService) FilterClientsPaginated(ctx context.Context, nameClient string, typeClient *bool, minProfit, maxProfit *int64, page, size int) (dto.PageOf[dto.ClientListResponse], error) {
	clients, total, err := s.repo.FindFilteredPaginated(ctx, nameClient, typeClient, page, size)
	if err != nil {
		return dto.PageOf[dto.ClientListResponse]{}, err
	}
	projects := s.fetchProjectsFor(ctx, clients)

	content := make([]dto.ClientListResponse, 0, len(clients))
	for i := range clients {
		row := toClientListResponse(&clients[i], projects[clients[i].ID])
		if withinProfit(row.TotalProfit, minProfit, maxProfit) {
			content = append(content, row)
		}
	}
	return dto.NewPage(content, page, size, total), nil
}

func withinProfit(profit int64, min, max *int64) bool {
	if min != nil && profit < *min {
		return false
	}
	if max != nil && profit > *max {
		return false
	}
	return true
}
