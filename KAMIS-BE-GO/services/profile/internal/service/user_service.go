// Package service holds the profile service's business logic (the Spring
// @Service / restservice layer). It depends on repositories and pkg/httpx
// clients, never on gorm or Gin.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/auth"
	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/profile/internal/dto"
	"github.com/pebintk/kamis-be-go/services/profile/internal/model"
	"github.com/pebintk/kamis-be-go/services/profile/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidRole  = errors.New("invalid role")
	ErrUserExists   = errors.New("email or username already exists")

	// ErrInvalidRefreshToken covers every way a refresh can fail — unknown,
	// expired, or already spent — because telling them apart would tell an
	// attacker which of their guesses was once real.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type UserService struct {
	repo       *repository.UserRepository
	tokens     *repository.RefreshTokenRepository
	issuer     *auth.Issuer
	refreshTTL time.Duration
}

func NewUserService(repo *repository.UserRepository, tokens *repository.RefreshTokenRepository, issuer *auth.Issuer, refreshTTL time.Duration) *UserService {
	return &UserService{repo: repo, tokens: tokens, issuer: issuer, refreshTTL: refreshTTL}
}

// withRoles maps an account together with every role it holds. Passing nil
// discriminators falls back to the primary role alone, which is what an account
// created before multi-role holds anyway.
func withRoles(u *model.EndUser, discriminators []string) dto.EndUserResponse {
	if len(discriminators) == 0 {
		discriminators = []string{u.UserType}
	}
	return dto.EndUserResponse{
		Email:    u.Email,
		Username: u.Username,
		Role:     u.APIRole(),
		Roles:    model.APIRolesFrom(model.OrderRoles(u.UserType, discriminators)),
	}
}

// resolveRoles turns a primary API role plus any extras into the stored
// discriminators, primary first. An unrecognised role is rejected rather than
// dropped, so a typo cannot silently create a weaker account.
func resolveRoles(primary string, extras []string) ([]string, error) {
	first, ok := model.DiscriminatorFromAPIRole(primary)
	if !ok {
		return nil, ErrInvalidRole
	}
	resolved := []string{first}
	for _, extra := range extras {
		discriminator, ok := model.DiscriminatorFromAPIRole(extra)
		if !ok {
			return nil, ErrInvalidRole
		}
		resolved = append(resolved, discriminator)
	}
	return model.OrderRoles(first, resolved), nil
}

// Login authenticates by email (legacy behaviour) and mints a token whose
// subject is the username and whose role claim is the PascalCase authority.
// It returns ErrUserNotFound for an unknown email (→404) and
// auth.ErrInvalidCredentials for a bad password (→401), matching the Java codes.
func (s *UserService) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if errors.Is(err, database.ErrNotFound) {
		return dto.LoginResponse{}, ErrUserNotFound
	}
	if err != nil {
		return dto.LoginResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return dto.LoginResponse{}, auth.ErrInvalidCredentials
	}
	discriminators, err := s.repo.RolesFor(ctx, user.ID, user.UserType)
	if err != nil {
		return dto.LoginResponse{}, err
	}
	return s.issue(ctx, user.Username, model.AuthoritiesFrom(discriminators))
}

// issue mints an access token and a fresh refresh token for a username.
func (s *UserService) issue(ctx context.Context, username string, roles []string) (dto.LoginResponse, error) {
	access, err := s.issuer.GenerateMulti(username, roles)
	if err != nil {
		return dto.LoginResponse{}, err
	}

	refresh, hash, err := auth.NewRefreshToken()
	if err != nil {
		return dto.LoginResponse{}, err
	}
	err = s.tokens.Create(ctx, &model.RefreshToken{
		TokenHash: hash,
		Username:  username,
		Role:      strings.Join(roles, ","),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return dto.LoginResponse{}, err
	}

	return dto.LoginResponse{
		Token:         access,
		RefreshToken:  refresh,
		ExpiresInSecs: int(s.issuer.TTL().Seconds()),
	}, nil
}

// Refresh exchanges a refresh token for a new access token, and rotates the
// refresh token itself.
//
// Rotation is what makes theft detectable: each token is single-use, so a token
// presented twice means someone kept a copy. In that case every token the user
// holds is revoked, which logs out both the thief and the victim — the victim
// can log in again, the thief cannot.
func (s *UserService) Refresh(ctx context.Context, presented string) (dto.LoginResponse, error) {
	stored, err := s.tokens.FindByHash(ctx, auth.HashRefreshToken(presented))
	if errors.Is(err, database.ErrNotFound) {
		return dto.LoginResponse{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return dto.LoginResponse{}, err
	}

	now := time.Now()
	if stored.RevokedAt != nil {
		// Replay of a spent token. Treat the whole family as compromised.
		if revokeErr := s.tokens.RevokeAllFor(ctx, stored.Username, now); revokeErr != nil {
			return dto.LoginResponse{}, revokeErr
		}
		return dto.LoginResponse{}, ErrInvalidRefreshToken
	}
	if !stored.Usable(now) {
		return dto.LoginResponse{}, ErrInvalidRefreshToken
	}

	if err := s.tokens.Revoke(ctx, stored.TokenHash, now); err != nil {
		return dto.LoginResponse{}, err
	}
	// The roles are the ones captured at login, so a role change takes effect on
	// the next login rather than silently mid-session.
	return s.issue(ctx, stored.Username, strings.Split(stored.Role, ","))
}

// Logout revokes a refresh token. It is deliberately silent about a token it
// does not recognise: logging out is not a place to confirm what exists.
func (s *UserService) Logout(ctx context.Context, presented string) error {
	err := s.tokens.Revoke(ctx, auth.HashRefreshToken(presented), time.Now())
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	return err
}

// AddUser creates an account. role is the lowercase API form.
func (s *UserService) AddUser(ctx context.Context, req dto.AddUserRequest) (dto.EndUserResponse, error) {
	discriminators, err := resolveRoles(req.Role, req.Roles)
	if err != nil {
		return dto.EndUserResponse{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return dto.EndUserResponse{}, err
	}
	user := &model.EndUser{
		Email:    req.Email,
		Username: req.Username,
		Password: string(hash),
		UserType: discriminators[0],
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return dto.EndUserResponse{}, mapDuplicate(err)
	}
	if err := s.repo.ReplaceRoles(ctx, user.ID, discriminators); err != nil {
		return dto.EndUserResponse{}, err
	}
	return withRoles(user, discriminators), nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]dto.EndUserResponse, error) {
	users, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return s.responses(ctx, users)
}

// responses maps a list of accounts, resolving every role set in one query
// rather than one per row.
func (s *UserService) responses(ctx context.Context, users []model.EndUser) ([]dto.EndUserResponse, error) {
	ids := make([]string, 0, len(users))
	for i := range users {
		ids = append(ids, users[i].ID)
	}
	roles, err := s.repo.RolesForAll(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]dto.EndUserResponse, 0, len(users))
	for i := range users {
		out = append(out, withRoles(&users[i], roles[users[i].ID]))
	}
	return out, nil
}

func (s *UserService) GetAllUsersPaginated(ctx context.Context, page, size int, email, username, userType string) (dto.Page, error) {
	users, total, err := s.repo.FindPaginated(ctx, email, username, userType, page, size)
	if err != nil {
		return dto.Page{}, err
	}
	content, err := s.responses(ctx, users)
	if err != nil {
		return dto.Page{}, err
	}
	return dto.NewPage(content, page, size, total), nil
}

// UpdateUser updates the account identified by email (the PUT /{id} path value
// is treated as the email, matching the legacy service).
func (s *UserService) UpdateUser(ctx context.Context, email string, req dto.UpdateUserRequest) (dto.EndUserResponse, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return dto.EndUserResponse{}, ErrUserNotFound
	}
	if err != nil {
		return dto.EndUserResponse{}, err
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return dto.EndUserResponse{}, err
		}
		user.Password = string(hash)
	}

	discriminators, err := s.repo.RolesFor(ctx, user.ID, user.UserType)
	if err != nil {
		return dto.EndUserResponse{}, err
	}
	if req.Roles != nil {
		if len(*req.Roles) == 0 {
			// An account with no role could authenticate and do nothing, which
			// is worse than refusing the edit.
			return dto.EndUserResponse{}, ErrInvalidRole
		}
		roles := *req.Roles
		if discriminators, err = resolveRoles(roles[0], roles[1:]); err != nil {
			return dto.EndUserResponse{}, err
		}
		user.UserType = discriminators[0]
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return dto.EndUserResponse{}, mapDuplicate(err)
	}
	if req.Roles != nil {
		if err := s.repo.ReplaceRoles(ctx, user.ID, discriminators); err != nil {
			return dto.EndUserResponse{}, err
		}
	}
	return withRoles(user, discriminators), nil
}

// EnsureAdmin seeds the default admin account if it does not already exist
// (the legacy AdminInitializer CommandLineRunner).
func (s *UserService) EnsureAdmin(ctx context.Context, email, username, password string) error {
	if email == "" {
		return nil
	}
	exists, err := s.repo.ExistsByEmail(ctx, email)
	if err != nil || exists {
		return err
	}
	_, err = s.AddUser(ctx, dto.AddUserRequest{
		Email:    email,
		Username: username,
		Password: password,
		Role:     "admin",
	})
	return err
}

func mapDuplicate(err error) error {
	if errors.Is(err, database.ErrDuplicate) {
		return ErrUserExists
	}
	return err
}
