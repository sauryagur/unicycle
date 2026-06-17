package utils

import (
	"fmt"

	"github.com/joho/godotenv"
)

// InitialEnv loads the environment file and returns the error if it fails
func InitialEnv() error {
	// Note: If running from root, you may need godotenv.Load() or godotenv.Load(".env")
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("failed to load environment file: %w", err)
	}
	return nil
}
