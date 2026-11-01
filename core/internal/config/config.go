package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	ArcRPCURL    string
	ATUsername   string
	ATAPIKey     string
	IsSandbox    bool
	Environment  string
}

func Load() *Config {
	loadDotEnv(".env")

	env := getEnv("APP_ENV", "development")

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		RedisURL:    getEnv("REDIS_URL", ""),
		ArcRPCURL:   getEnv("ARC_RPC_URL", "https://rpc.arc.network"),
		ATUsername:  getEnv("AT_USERNAME", "sandbox"),
		ATAPIKey:    getEnv("AT_API_KEY", ""),
		IsSandbox:   env != "production",
		Environment: env,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			_ = os.Setenv(key, val)
		}
	}
	log.Println("✓ Loaded environment configuration from .env")
}
