package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port                          string
	MySQLDSN                      string
	AIServiceBaseURL              string
	AIServiceTimeoutSeconds       int
	AIServiceAsyncTimeoutSeconds  int
	MaxConcurrentOCR              int
	MaxBatchSize                  int
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:                         getEnv("PORT", "8080"),
		MySQLDSN:                     strings.TrimSpace(os.Getenv("MYSQL_DSN")),
		AIServiceBaseURL:             strings.TrimSpace(os.Getenv("AI_SERVICE_BASE_URL")),
		AIServiceTimeoutSeconds:      getEnvInt("AI_SERVICE_TIMEOUT_SECONDS", 15),
		AIServiceAsyncTimeoutSeconds: getEnvInt("AI_SERVICE_ASYNC_TIMEOUT_SECONDS", 300),
		MaxConcurrentOCR:             getEnvInt("MAX_CONCURRENT_OCR", 3),
		MaxBatchSize:                 getEnvInt("MAX_BATCH_SIZE", 20),
	}

	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("missing required environment variable MYSQL_DSN")
	}
	if cfg.AIServiceBaseURL == "" {
		return Config{}, fmt.Errorf("missing required environment variable AI_SERVICE_BASE_URL")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	rawValue := os.Getenv(key)
	if rawValue == "" {
		return fallback
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return fallback
	}

	return value
}

func loadDotEnv() error {
	for _, candidate := range []string{".env", filepath.Join("backend", ".env")} {
		if err := loadEnvFile(candidate); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("load env file %s: %w", candidate, err)
		}
		return nil
	}

	return nil
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid line %d", lineNo)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("invalid empty key on line %d", lineNo)
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from line %d: %w", key, lineNo, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan file: %w", err)
	}

	return nil
}
