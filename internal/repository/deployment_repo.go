package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type DeploymentRepository struct{ db *gorm.DB }

func NewDeploymentRepository(db *DB) *DeploymentRepository {
	return &DeploymentRepository{db: db.DB}
}

func (r *DeploymentRepository) Create(ctx context.Context, d *Deployment) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *DeploymentRepository) UpdateStatus(
	ctx context.Context, id uint, status, logStr, errMsg string,
) error {
	now := time.Now()
	updates := map[string]any{
		"status":     status,
		"build_log":  logStr,
		"error_msg":  errMsg,
		"updated_at": now,
	}
	if status == "running" || status == "failed" || status == "stopped" {
		updates["finished_at"] = now
	}
	return r.db.WithContext(ctx).Model(&Deployment{}).Where("id = ?", id).Updates(updates).Error
}

func (r *DeploymentRepository) ListByApp(ctx context.Context, appID uint, limit int) ([]Deployment, error) {
	var list []Deployment
	q := r.db.WithContext(ctx).Where("app_id = ?", appID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&list).Error
	return list, err
}

func (r *DeploymentRepository) FindByID(ctx context.Context, id uint) (*Deployment, error) {
	var d Deployment
	err := r.db.WithContext(ctx).First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

// MarkStarted mengisi started_at & status building.
func (r *DeploymentRepository) MarkStarted(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&Deployment{}).Where("id = ?", id).
		Updates(map[string]any{"status": "building", "started_at": now}).Error
}

// SetCommit menyimpan info commit hasil clone.
func (r *DeploymentRepository) SetCommit(ctx context.Context, id uint, sha, msg string) error {
	return r.db.WithContext(ctx).Model(&Deployment{}).Where("id = ?", id).
		Updates(map[string]any{"commit_sha": sha, "commit_msg": msg}).Error
}

// Finish menutup deployment dengan status akhir, log, image, dan durasi.
func (r *DeploymentRepository) Finish(
	ctx context.Context, id uint, status, logStr, errMsg, imageTag string,
) error {
	d, err := r.FindByID(ctx, id)
	if err != nil || d == nil {
		return err
	}
	now := time.Now()
	dur := 0
	if d.StartedAt != nil {
		dur = int(now.Sub(*d.StartedAt).Seconds())
	}
	return r.db.WithContext(ctx).Model(&Deployment{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "build_log": logStr, "error_msg": errMsg, "image_tag": imageTag,
		"finished_at": now, "duration_sec": dur,
	}).Error
}
// SetResolvedConfig menyimpan snapshot hasil resolusi (Phase 2).
func (r *DeploymentRepository) SetResolvedConfig(
	ctx context.Context, id uint,
	projectType, configSource, configJSON string,
) error {
	return r.db.WithContext(ctx).Model(&Deployment{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"project_type":  projectType,
			"config_source": configSource,
			"config_json":   configJSON,
		}).Error
}