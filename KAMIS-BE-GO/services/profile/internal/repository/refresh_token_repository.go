package repository

import (
	"context"
	"time"

	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/profile/internal/model"
	"gorm.io/gorm"
)

type RefreshTokenRepository struct{ db *gorm.DB }

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	return database.Translate(r.db.WithContext(ctx).Create(token).Error)
}

// FindByHash looks a presented token up. A revoked row is returned rather than
// hidden, because recognising a replay is the point of keeping it.
func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &token, nil
}

// Revoke marks one token spent. Already-revoked rows keep their original time,
// so the first revocation is what the audit trail shows.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, hash string, at time.Time) error {
	return database.Translate(r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Update("revoked_at", at).Error)
}

// RevokeAllFor kills every live token a user holds. It runs on a detected
// replay, where one stolen token means the whole family is suspect.
func (r *RefreshTokenRepository) RevokeAllFor(ctx context.Context, username string, at time.Time) error {
	return database.Translate(r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("username = ? AND revoked_at IS NULL", username).
		Update("revoked_at", at).Error)
}

// DeleteExpired sweeps rows that can no longer be presented. A revoked row is
// kept until its natural expiry, because that is exactly the window in which a
// replay could still arrive.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < ?", before).Delete(&model.RefreshToken{})
	return result.RowsAffected, database.Translate(result.Error)
}
