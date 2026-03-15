package env

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// GetString returns the value of the environment variable or the default.
func GetString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

// GetInt returns the integer value of the environment variable or the default.
func GetInt(key string, defaultVal int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// GetBool returns the boolean value of the environment variable or the default.
func GetBool(key string, defaultVal bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

// GetDuration returns the duration value of the environment variable or the default.
func GetDuration(key string, defaultVal string) time.Duration {
	val := GetString(key, defaultVal)
	d, err := time.ParseDuration(val)
	if err != nil {
		d, _ = time.ParseDuration(defaultVal)
		return d
	}
	return d
}

// GetStringSlice returns a comma-separated environment variable as a string slice.
func GetStringSlice(key string, defaultVal string) []string {
	val := GetString(key, defaultVal)
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// GetStringFromFile reads the content of a file specified by the environment variable.
// This supports the _FILE pattern for secrets (e.g. DB_PASSWORD_FILE=/run/secrets/db_password).
func GetStringFromFile(key string) (string, error) {
	path, ok := os.LookupEnv(key)
	if !ok {
		return "", os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
