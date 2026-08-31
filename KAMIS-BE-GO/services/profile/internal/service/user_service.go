package service

import (
	"context"
	"errors"
	"strings"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/services/profile/internal/dto"
	"github.com/karina/kamis-be-go/services/profile/internal/model"
	"github.com/karina/kamis-be-go/services/profile/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidRole  = errors.New("invalid role")
	ErrUserExists   = errors.New("email or username already exists")
)

type UserService struct {
	repo   *repository.UserRepository
	issuer *auth.Issuer
}

func NewUserService(repo *repository.UserRepository, issuer *auth.Issuer) *UserService {
	return &UserService{repo: repo, issuer: issuer}
}

func toResponse(u *model.EndUser) dto.EndUserResponse {
	return dto.EndUserResponse{Email: u.Email, Username: u.Username, Role: u.APIRole()}
}

// Login authenticates by email (legacy behaviour) and mints a token whose
// subject is the username and whose role claim is the PascalCase authority.
// It returns ErrUserNotFound for an unknown email (→404) and
// auth.ErrInvalidCredentials for a bad password (→401), matching the Java codes.
func (s *UserService) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.LoginResponse{}, ErrUserNotFound
	}
	if err != nil {
		return dto.LoginResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return dto.LoginResponse{}, auth.ErrInvalidCredentials
	}
	token, err := s.issuer.Generate(user.Username, user.Authority())
	if err != nil {
		return dto.LoginResponse{}, err
	}
	return dto.LoginResponse{Token: token}, nil
}

// AddUser creates an account. role is the lowercase API form.
func (s *UserService) AddUser(ctx context.Context, req dto.AddUserRequest) (dto.EndUserResponse, error) {
	discriminator, ok := model.DiscriminatorFromAPIRole(req.Role)
	if !ok {
		return dto.EndUserResponse{}, ErrInvalidRole
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return dto.EndUserResponse{}, err
	}
	user := &model.EndUser{
		Email:    req.Email,
		Username: req.Username,
		Password: string(hash),
		UserType: discriminator,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return dto.EndUserResponse{}, mapDuplicate(err)
	}
	return toResponse(user), nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]dto.EndUserResponse, error) {
	users, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EndUserResponse, 0, len(users))
	for i := range users {
		out = append(out, toResponse(&users[i]))
	}
	return out, nil
}

func (s *UserService) GetAllUsersPaginated(ctx context.Context, page, size int, email, username, userType string) (dto.Page, error) {
	users, total, err := s.repo.FindPaginated(ctx, email, username, userType, page, size)
	if err != nil {
		return dto.Page{}, err
	}
	content := make([]dto.EndUserResponse, 0, len(users))
	for i := range users {
		content = append(content, toResponse(&users[i]))
	}
	return dto.NewPage(content, page, size, total), nil
}

// UpdateUser updates the account identified by email (the PUT /{id} path value
// is treated as the email, matching the legacy service).
func (s *UserService) UpdateUser(ctx context.Context, email string, req dto.UpdateUserRequest) (dto.EndUserResponse, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
	if err := s.repo.Save(ctx, user); err != nil {
		return dto.EndUserResponse{}, mapDuplicate(err)
	}
	return toResponse(user), nil
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
	if err != nil && (strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505")) {
		return ErrUserExists
	}
	return err
}
