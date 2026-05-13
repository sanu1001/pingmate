package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	JWTSecret      string
	JWTExpiryHours int
}

var App Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — reading from environment")
	}

	hours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "72"))
	if err != nil {
		hours = 72
	}

	App = Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTExpiryHours: hours,
	}

	if App.DatabaseURL == "" {
		log.Fatal("FATAL: DATABASE_URL is not set")
	}

	if App.JWTSecret == "" {
		log.Fatal("FATAL: JWT_SECRET is not set")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
