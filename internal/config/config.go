package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURl        string
	PolicyFilePath     string
	PrometheusEndpoint string
	StartControlPlane  bool
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file %w", err)
	}

	config := &Config{
		DatabaseURl:        os.Getenv("DATABASE_URL"),
		PolicyFilePath:     os.Getenv("POLICY_FILE_PATH"),
		PrometheusEndpoint: os.Getenv("PROMETHEUS_ENDPOINT"),
		StartControlPlane:  os.Getenv("START_CONTROL_PLANE_LOOP") == "true",
	}

	log.Println("Loaded environment variables", config)

	return config, nil
}
