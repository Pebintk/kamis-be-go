// Package repository is the profile service's data-access layer (the Spring
// Data JPA repositories). Driver and ORM errors are translated to the
// pkg/database domain errors here, so the service layer never imports gorm.
package repository

import (
	"context"

	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/profile/internal/model"
	"gorm.io/gorm"
)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.EndUser, error) {
	var u model.EndUser
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &u, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.EndUser, error) {
	var u model.EndUser
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &u, nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.EndUser{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *UserRepository) Create(ctx context.Context, u *model.EndUser) error {
	return database.Translate(r.db.WithContext(ctx).Create(u).Error)
}

func (r *UserRepository) Save(ctx context.Context, u *model.EndUser) error {
	return database.Translate(r.db.WithContext(ctx).Save(u).Error)
}

func (r *UserRepository) FindAll(ctx context.Context) ([]model.EndUser, error) {
	var users []model.EndUser
	err := r.db.WithContext(ctx).Order("username").Find(&users).Error
	return users, err
}

// FindPaginated applies optional case-insensitive "contains" filters (the legacy
// findBy...ContainingIgnoreCase queries) and returns the page plus total count.
func (r *UserRepository) FindPaginated(ctx context.Context, email, username, userType string, page, size int) ([]model.EndUser, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.EndUser{})
	if email != "" {
		q = q.Where("email ILIKE ?", "%"+email+"%")
	}
	if username != "" {
		q = q.Where("username ILIKE ?", "%"+username+"%")
	}
	if userType != "" {
		q = q.Where("user_type ILIKE ?", "%"+userType+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.EndUser
	if err := q.Order("username").Offset(page * size).Limit(size).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// ---- roles ----

// RolesFor returns every role discriminator an account holds, primary first.
func (r *UserRepository) RolesFor(ctx context.Context, userID, primary string) ([]string, error) {
	var stored []string
	err := r.db.WithContext(ctx).Model(&model.UserRole{}).
		Where("user_id = ?", userID).
		Order("discriminator").
		Pluck("discriminator", &stored).Error
	if err != nil {
		return nil, err
	}
	return model.OrderRoles(primary, stored), nil
}

// RolesForAll returns the roles of several accounts at once, keyed by user id.
// The list endpoints need them per row.
func (r *UserRepository) RolesForAll(ctx context.Context, userIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	var rows []model.UserRole
	if err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).
		Order("discriminator").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = append(out[row.UserID], row.Discriminator)
	}
	return out, nil
}

// ReplaceRoles makes discriminators the complete set of roles an account holds.
// The caller has already put the primary role first.
func (r *UserRepository) ReplaceRoles(ctx context.Context, userID string, discriminators []string) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		rows := make([]model.UserRole, 0, len(discriminators))
		for _, d := range discriminators {
			rows = append(rows, model.UserRole{UserID: userID, Discriminator: d})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	}))
}
