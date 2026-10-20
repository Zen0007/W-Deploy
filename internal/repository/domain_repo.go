package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type DomainRepository struct{ db *gorm.DB }

func NewDomainRepository(db *DB) *DomainRepository {
	return &DomainRepository{db: db.DB}
}

func (r *DomainRepository) Create(ctx context.Context, d *Domain) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *DomainRepository) FindByHostname(ctx context.Context, hostname string) (*Domain, error) {
	var d Domain
	err := r.db.WithContext(ctx).Where("hostname = ?", hostname).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

func (r *DomainRepository) UpdateCloudflare(
	ctx context.Context, id uint, zoneID, recordID, tunnelID string,
) error {
	return r.db.WithContext(ctx).Model(&Domain{}).Where("id = ?", id).Updates(map[string]any{
		"cf_zone_id":   zoneID,
		"cf_record_id": recordID,
		"cf_tunnel_id": tunnelID,
		"status":       "active",
	}).Error
}

func (r *DomainRepository) ListByApp(ctx context.Context, appID uint) ([]Domain, error) {
	var list []Domain
	err := r.db.WithContext(ctx).Where("app_id = ?", appID).Find(&list).Error
	return list, err
}
