package config

import (
	"github.com/hsdfat8/eir/internal/observability"
	"github.com/joho/godotenv"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		observability.Log.Warnw("Error loading .env file", "error", err.Error())
	}
}
