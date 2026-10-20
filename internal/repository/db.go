package repository

import (
	"W-dev/internal/config"
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Db membungkus *grom.Db
type DB struct {
	*gorm.DB
}

// NewDb open connection PostgreSQL + pooling + auto-migration
func NewDb(cfg *config.Config) (*DB, error) {
	logLevel := logger.Warn
	if !cfg.IsProduction() {
		logLevel = logger.Info
	}

	gormCfg := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: false,
		PrepareStmt:                              true,
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("gagl konek postgres %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	//test ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres tidak merespon %w", err)
	}

	log.Println("[db] berhasil terhubung ke PostgreSQL")
	return &DB{db}, nil
}

// AutoMigrate menjalankan migrasi tabel.
func (d *DB) AutoMigrate() error {
	return d.DB.AutoMigrate(
		&App{},
		&Deployment{},
		&Domain{},
		&EnvVar{},
		&UserToken{},
		&AppDatabase{},
	)
}

// Close menutup koneksi pool
func (d *DB) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
