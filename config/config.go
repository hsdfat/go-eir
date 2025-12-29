package config

import (
	"github.com/hsdfat/go-eir/internal/logger"
	"github.com/joho/godotenv"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		logger.Log.Warnw("Error loading .env file", "error", err.Error())
	}
}
