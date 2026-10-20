package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnvVarRepository mengelola env_vars per aplikasi.
type EnvVarRepository struct{ db *gorm.DB }

func NewEnvVarRepository(db *DB) *EnvVarRepository { return &EnvVarRepository{db: db.DB} }

func (r *EnvVarRepository) ListByApp(ctx context.Context, appID uint) ([]EnvVar, error) {
	var list []EnvVar
	err := r.db.WithContext(ctx).Where("app_id = ?", appID).Order("key").Find(&list).Error
	return list, err
}

// Set membuat atau memperbarui satu variabel (unik per app_id+key).
func (r *EnvVarRepository) Set(ctx context.Context, appID uint, key, value string) error {
	var existing EnvVar
	err := r.db.WithContext(ctx).Where("app_id = ? AND key = ?", appID, key).First(&existing).Error
	if err == nil {
		return r.db.WithContext(ctx).Model(&existing).Update("value", value).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&EnvVar{AppID: appID, Key: key, Value: value}).Error
}

func (r *EnvVarRepository) Delete(ctx context.Context, appID uint, key string) error {
	return r.db.WithContext(ctx).Where("app_id = ? AND key = ?", appID, key).Delete(&EnvVar{}).Error
}
