package repository

import (
	"context"

	"gorm.io/gorm"
)

// AppDatabaseRepository mengelola catatan database terisolasi per app.
type AppDatabaseRepository struct{ db *gorm.DB }

func NewAppDatabaseRepository(db *DB) *AppDatabaseRepository {
	return &AppDatabaseRepository{db: db.DB}
}

func (r *AppDatabaseRepository) Create(ctx context.Context, d *AppDatabase) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *AppDatabaseRepository) ListByApp(ctx context.Context, appID uint) ([]AppDatabase, error) {
	var list []AppDatabase
	err := r.db.WithContext(ctx).Where("app_id = ?", appID).Find(&list).Error
	return list, err
}

func (r *AppDatabaseRepository) DeleteByApp(ctx context.Context, appID uint) error {
	return r.db.WithContext(ctx).Where("app_id = ?", appID).Delete(&AppDatabase{}).Error
}
