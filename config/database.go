package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sauryagur/unicycle/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Database connected successfully")
	return nil
}

func MigrateDB() error {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Bicycle{},
		&models.Router{},
		&models.Ride{},
		&models.Report{},
		&models.Transaction{},
		&models.BikeTelemetry{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := CreateGORMIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}

func CreateGORMIndexes() error {
	migrator := DB.Migrator()

	// Ride composite index: (user_id, started_at)
	if !migrator.HasIndex(&models.Ride{}, "idx_rides_user_started") {
		if err := migrator.CreateIndex(&models.Ride{}, "idx_rides_user_started"); err != nil {
			return fmt.Errorf("failed creating ride composite index: %w", err)
		}
	}

	// Telemetry composite index: (bike_id, time)
	if !migrator.HasIndex(&models.BikeTelemetry{}, "idx_telemetry_bike_time") {
		if err := migrator.CreateIndex(&models.BikeTelemetry{}, "idx_telemetry_bike_time"); err != nil {
			return fmt.Errorf("failed creating telemetry composite index: %w", err)
		}
	}

	return nil
}

func CloseDB() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
