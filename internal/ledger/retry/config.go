package retry

import (
	"os"
	"strconv"
	"time"
)

// Config holds retry configuration
type Config struct {
	Enabled      bool          // Enable/disable retry mechanism
	MaxRetries   int           // Maximum number of retry attempts
	InitialDelay time.Duration // Initial delay before first retry
	MaxDelay     time.Duration // Maximum delay between retries
}

// LoadConfig loads retry configuration from environment variables
func LoadConfig() Config {
	return Config{
		Enabled:      getEnvAsBool("RETRY_ENABLED", true),
		MaxRetries:   getEnvAsInt("RETRY_MAX_RETRIES", 10),
		InitialDelay: time.Duration(getEnvAsInt("RETRY_INITIAL_DELAY_SEC", 1)) * time.Second,
		MaxDelay:     time.Duration(getEnvAsInt("RETRY_MAX_DELAY_SEC", 60)) * time.Second,
	}
}

// Helper: get bool from env
func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// Helper: get int from env
func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
