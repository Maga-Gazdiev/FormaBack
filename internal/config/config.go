package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host              string
	Port              string
	CORSOrigins       []string
	MaxFileSize       int64
	ConversionTimeout time.Duration
	MaxConcurrent     int
	TempDir           string
}

func (c Config) Address() string { return net.JoinHostPort(c.Host, c.Port) }
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}
	cfg := Config{Host: env("HOST", "0.0.0.0"), Port: env("PORT", "8080"), TempDir: os.Getenv("TEMP_DIR")}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return cfg, errors.New("PORT must be between 1 and 65535")
	}
	mb, err := positiveInt("MAX_FILE_SIZE_MB", 50, 1024)
	if err != nil {
		return cfg, err
	}
	cfg.MaxFileSize = int64(mb) << 20
	seconds, err := positiveInt("CONVERSION_TIMEOUT_SECONDS", 120, 3600)
	if err != nil {
		return cfg, err
	}
	cfg.ConversionTimeout = time.Duration(seconds) * time.Second
	cfg.MaxConcurrent, err = positiveInt("MAX_CONCURRENT_CONVERSIONS", 2, 32)
	if err != nil {
		return cfg, err
	}
	for _, origin := range strings.Split(env("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000,http://localhost:3080,http://127.0.0.1:3080"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin != "*" {
			u, e := url.Parse(origin)
			if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
				return cfg, fmt.Errorf("invalid CORS origin %q: use scheme://host:port without trailing slash", origin)
			}
		}
		cfg.CORSOrigins = append(cfg.CORSOrigins, origin)
	}
	return cfg, nil
}
func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
func positiveInt(key string, fallback, max int) (int, error) {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 || value > max {
		return 0, fmt.Errorf("%s must be between 1 and %d", key, max)
	}
	return value, nil
}

// Environment variables take precedence over the optional local .env file.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		s := strings.TrimSpace(scanner.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		key, value, ok := strings.Cut(s, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("invalid .env line %d", line)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
