package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type AppRepository struct{ db *gorm.DB }

func NewAppRepository(db *DB) *AppRepository {
	return &AppRepository{db: db.DB}
}

func (r *AppRepository) Create(ctx context.Context, app *App) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *AppRepository) FindByID(ctx context.Context, id uint) (*App, error) {
	var app App
	err := r.db.WithContext(ctx).First(&app, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &app, err
}

func (r *AppRepository) FindBySlug(ctx context.Context, slug string) (*App, error) {
	var app App
	err := r.db.WithContext(ctx).
		Where("slug = ?", slug).
		First(&app).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &app, err
}

func (r *AppRepository) UpdateStatus(ctx context.Context, id uint, status, lastErr string) error {
	return r.db.WithContext(ctx).Model(&App{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "last_err": lastErr}).Error
}

func (r *AppRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete agar slug bisa dipakai ulang & FK ON DELETE CASCADE membersihkan relasi.
	return r.db.WithContext(ctx).Unscoped().Delete(&App{}, id).Error
}

// List mengembalikan semua app (opsional difilter pemilik).
func (r *AppRepository) List(ctx context.Context, ownerLogin string) ([]App, error) {
	var list []App
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if ownerLogin != "" {
		q = q.Where("owner_login = ?", ownerLogin)
	}
	err := q.Find(&list).Error
	return list, err
}

// FindByGitHubRepo mencari app yang terhubung ke repo "owner/repo" dan auto-deploy aktif.
func (r *AppRepository) FindByGitHubRepo(ctx context.Context, fullName string) ([]App, error) {
	var list []App
	err := r.db.WithContext(ctx).
		Where("LOWER(github_full_name) = LOWER(?) AND auto_deploy = ?", fullName, true).
		Find(&list).Error
	return list, err
}

// Update menyimpan field tertentu.
func (r *AppRepository) Update(ctx context.Context, id uint, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&App{}).Where("id = ?", id).Updates(fields).Error
}

// SetRuntime menyimpan container & image aktif.
func (r *AppRepository) SetRuntime(ctx context.Context, id uint, containerID, imageTag string) error {
	return r.Update(ctx, id, map[string]any{"container_id": containerID, "image_tag": imageTag})
}
