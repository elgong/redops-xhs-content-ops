package main

import (
	"bufio"
	"os"
	"strings"
)

func LoadConfig() Config {
	loadDotEnv(".env")
	return Config{
		Addr:               env("APP_ADDR", ":8080"),
		MySQLDSN:           env("MYSQL_DSN", ""),
		AutoMigrate:        envBool("AUTO_MIGRATE", true),
		SeedData:           envBool("SEED_DATA", false),
		SchedulerEnabled:   envBool("SCHEDULER_ENABLED", true),
		XHSAdapterMode:     strings.ToLower(env("XHS_ADAPTER", "openapi")),
		XHSBaseURL:         env("XHS_BASE_URL", "https://ark.xiaohongshu.com"),
		XHSAccessToken:     env("XHS_ACCESS_TOKEN", ""),
		XHSDraftEndpoint:   env("XHS_DRAFT_ENDPOINT", ""),
		XHSPublishEndpoint: env("XHS_PUBLISH_ENDPOINT", ""),
	}
}

func StoreMode() string {
	mode := strings.ToLower(strings.TrimSpace(env("APP_STORE", "")))
	if mode != "" {
		return mode
	}
	if strings.TrimSpace(os.Getenv("MYSQL_DSN")) != "" {
		return "mysql"
	}
	return "memory"
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
