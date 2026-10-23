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
	return r.db.WithContext(ctx).Clauses( // internal/repository/user_token_repo.go
		// internal/repository/user_token_repo.go
		clause.OnConflict{
			Columns: []clause.Column{{Name: "github_id"}}, // ← ubah ke git_hub_id
			DoUpdates: clause.AssignmentColumns([]string{
				"login", "avatar_url", "name", "access_token",
				"token_type", "scope", "updated_at",
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
