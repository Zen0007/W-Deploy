package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserTokenRepository mengelola tabel user_tokens.
type UserTokenRepository struct{ db *gorm.DB }

func NewUserTokenRepository(db *DB) *UserTokenRepository {
	return &UserTokenRepository{db: db.DB}
}

// Upsert menyimpan token baru atau memperbarui token user yang sama (key: github_id).
func (r *UserTokenRepository) Upsert(ctx context.Context, t *UserToken) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "github_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"login", "avatar_url", "name", "access_token",
			"token_type", "scope",
			"refresh_token", "token_expires_at", "refresh_token_expires_at",
			"updated_at",
		}),
	}).Create(t).Error
}

func (r *UserTokenRepository) FindByLogin(ctx context.Context, login string) (*UserToken, error) {
	var t UserToken
	err := r.db.WithContext(ctx).Where("login = ?", login).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &t, err
}

func (r *UserTokenRepository) Delete(ctx context.Context, login string) error {
	return r.db.WithContext(ctx).Where("login = ?", login).Delete(&UserToken{}).Error
}

// Update memakai map agar partial update (mis. hanya token) tidak menghapus field lain.
func (r *UserTokenRepository) Update(ctx context.Context, login string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&UserToken{}).Where("login = ?", login).Updates(updates).Error
}
