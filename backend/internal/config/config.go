package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv                          string
	AppPort                         string
	DatabaseDSN                     string
	RedisAddr                       string
	RedisPassword                   string
	RedisDB                         int
	NATSURL                         string
	JWTSecret                       string
	AutoMigrate                     bool
	AdminUsername                   string
	AdminPassword                   string
	AdminEmail                      string
	CORSOrigins                     []string
	DBMaxIdleConns                  int
	DBMaxOpenConns                  int
	DBConnMaxLifetime               time.Duration
	DBConnMaxIdleTime               time.Duration
	HTTPMaxInFlight                 int
	WorkerConcurrency               int
	TaskRunLockStaleAfter           time.Duration
	TaskQueueAckWait                time.Duration
	TaskQueueMaxDeliver             int
	LogRetentionDays                int
	SchedulerInterval               time.Duration
	ImportStageRetentionHours       int
	TelegramSyncPython              string
	TelegramSyncScript              string
	TelegramSyncTimeoutSeconds      int
	ListenerAccountCheckConcurrency int
	ListenerProxyCheckConcurrency   int
	TelegramApplyScript             string
	TelegramMessageScript           string
	TelegramListenScript            string
	TelegramApplyTimeoutSeconds     int
	AppVersion                      string
	UpdateEnabled                   bool
	UpdateDockerSocket              string
	UpdateDockerContainer           string
	UpdateCommand                   string
	UpdateLatestReleaseURL          string
}

func (c Config) ValidateGateway() error {
	if c.AppEnv != "production" {
		return nil
	}
	if strings.TrimSpace(c.JWTSecret) == "" ||
		c.JWTSecret == "change-me-in-production" ||
		c.JWTSecret == "local-dev-secret-change-me" ||
		c.JWTSecret == "replace-with-production-secret" ||
		len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be set to a strong production value")
	}
	if strings.TrimSpace(c.AdminPassword) == "" ||
		c.AdminPassword == "admin123456" ||
		c.AdminPassword == "replace-with-production-admin-password" {
		return fmt.Errorf("ADMIN_PASSWORD must be changed before production startup")
	}
	return nil
}

func Load() Config {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	setDefault("APP_ENV", "development")
	setDefault("APP_PORT", "8080")
	setDefault("DATABASE_DSN", "host=localhost user=codex3 password=codex3 dbname=codex3 port=5432 sslmode=disable TimeZone=Asia/Shanghai")
	setDefault("REDIS_ADDR", "localhost:6379")
	setDefault("REDIS_PASSWORD", "")
	setDefault("REDIS_DB", 0)
	setDefault("NATS_URL", "nats://localhost:4222")
	setDefault("JWT_SECRET", "change-me-in-production")
	setDefault("AUTO_MIGRATE", true)
	setDefault("ADMIN_USERNAME", "admin")
	setDefault("ADMIN_PASSWORD", "admin123456")
	setDefault("ADMIN_EMAIL", "admin@example.com")
	setDefault("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")
	setDefault("DB_MAX_IDLE_CONNS", 10)
	setDefault("DB_MAX_OPEN_CONNS", 50)
	setDefault("DB_CONN_MAX_LIFETIME_SECONDS", 3600)
	setDefault("DB_CONN_MAX_IDLE_TIME_SECONDS", 300)
	setDefault("HTTP_MAX_IN_FLIGHT", 2000)
	setDefault("WORKER_CONCURRENCY", 8)
	setDefault("TASK_RUN_LOCK_STALE_SECONDS", 93600)
	setDefault("TASK_QUEUE_ACK_WAIT_SECONDS", 93600)
	setDefault("TASK_QUEUE_MAX_DELIVER", 5)
	setDefault("LOG_RETENTION_DAYS", 30)
	setDefault("SCHEDULER_INTERVAL_SECONDS", 3600)
	setDefault("IMPORT_STAGE_RETENTION_HOURS", 24)
	setDefault("TELEGRAM_SYNC_PYTHON", "./.venv/bin/python")
	setDefault("TELEGRAM_SYNC_SCRIPT", "./scripts/telegram_profile_sync.py")
	setDefault("TELEGRAM_SYNC_TIMEOUT_SECONDS", 90)
	setDefault("LISTENER_ACCOUNT_CHECK_CONCURRENCY", 10)
	setDefault("LISTENER_PROXY_CHECK_CONCURRENCY", 20)
	setDefault("TELEGRAM_APPLY_SCRIPT", "./scripts/telegram_profile_apply.py")
	setDefault("TELEGRAM_MESSAGE_SCRIPT", "./scripts/telegram_message_send.py")
	setDefault("TELEGRAM_LISTEN_SCRIPT", "./scripts/telegram_keyword_listen.py")
	setDefault("TELEGRAM_APPLY_TIMEOUT_SECONDS", 90)
	setDefault("APP_VERSION", "1.0.69")
	setDefault("APP_UPDATE_ENABLED", false)
	setDefault("APP_UPDATE_DOCKER_SOCKET", "/var/run/docker.sock")
	setDefault("APP_UPDATE_DOCKER_CONTAINER", "tg-updater")
	setDefault("APP_UPDATE_COMMAND", "cd /workspace && docker compose pull || true; docker compose up -d --build --remove-orphans frontend gateway worker scheduler postgres redis nats")
	setDefault("APP_UPDATE_LATEST_RELEASE_URL", "https://api.github.com/repos/bailang1999-a11y/TG-Marketing-Assistant/releases/latest")

	return Config{
		AppEnv:                          viper.GetString("APP_ENV"),
		AppPort:                         viper.GetString("APP_PORT"),
		DatabaseDSN:                     viper.GetString("DATABASE_DSN"),
		RedisAddr:                       viper.GetString("REDIS_ADDR"),
		RedisPassword:                   viper.GetString("REDIS_PASSWORD"),
		RedisDB:                         viper.GetInt("REDIS_DB"),
		NATSURL:                         viper.GetString("NATS_URL"),
		JWTSecret:                       viper.GetString("JWT_SECRET"),
		AutoMigrate:                     viper.GetBool("AUTO_MIGRATE"),
		AdminUsername:                   viper.GetString("ADMIN_USERNAME"),
		AdminPassword:                   viper.GetString("ADMIN_PASSWORD"),
		AdminEmail:                      viper.GetString("ADMIN_EMAIL"),
		CORSOrigins:                     splitCSV(viper.GetString("CORS_ORIGINS")),
		DBMaxIdleConns:                  viper.GetInt("DB_MAX_IDLE_CONNS"),
		DBMaxOpenConns:                  viper.GetInt("DB_MAX_OPEN_CONNS"),
		DBConnMaxLifetime:               time.Duration(viper.GetInt("DB_CONN_MAX_LIFETIME_SECONDS")) * time.Second,
		DBConnMaxIdleTime:               time.Duration(viper.GetInt("DB_CONN_MAX_IDLE_TIME_SECONDS")) * time.Second,
		HTTPMaxInFlight:                 viper.GetInt("HTTP_MAX_IN_FLIGHT"),
		WorkerConcurrency:               viper.GetInt("WORKER_CONCURRENCY"),
		TaskRunLockStaleAfter:           time.Duration(viper.GetInt("TASK_RUN_LOCK_STALE_SECONDS")) * time.Second,
		TaskQueueAckWait:                time.Duration(viper.GetInt("TASK_QUEUE_ACK_WAIT_SECONDS")) * time.Second,
		TaskQueueMaxDeliver:             viper.GetInt("TASK_QUEUE_MAX_DELIVER"),
		LogRetentionDays:                viper.GetInt("LOG_RETENTION_DAYS"),
		SchedulerInterval:               time.Duration(viper.GetInt("SCHEDULER_INTERVAL_SECONDS")) * time.Second,
		ImportStageRetentionHours:       viper.GetInt("IMPORT_STAGE_RETENTION_HOURS"),
		TelegramSyncPython:              viper.GetString("TELEGRAM_SYNC_PYTHON"),
		TelegramSyncScript:              viper.GetString("TELEGRAM_SYNC_SCRIPT"),
		TelegramSyncTimeoutSeconds:      viper.GetInt("TELEGRAM_SYNC_TIMEOUT_SECONDS"),
		ListenerAccountCheckConcurrency: viper.GetInt("LISTENER_ACCOUNT_CHECK_CONCURRENCY"),
		ListenerProxyCheckConcurrency:   viper.GetInt("LISTENER_PROXY_CHECK_CONCURRENCY"),
		TelegramApplyScript:             viper.GetString("TELEGRAM_APPLY_SCRIPT"),
		TelegramMessageScript:           viper.GetString("TELEGRAM_MESSAGE_SCRIPT"),
		TelegramListenScript:            viper.GetString("TELEGRAM_LISTEN_SCRIPT"),
		TelegramApplyTimeoutSeconds:     viper.GetInt("TELEGRAM_APPLY_TIMEOUT_SECONDS"),
		AppVersion:                      viper.GetString("APP_VERSION"),
		UpdateEnabled:                   viper.GetBool("APP_UPDATE_ENABLED"),
		UpdateDockerSocket:              viper.GetString("APP_UPDATE_DOCKER_SOCKET"),
		UpdateDockerContainer:           viper.GetString("APP_UPDATE_DOCKER_CONTAINER"),
		UpdateCommand:                   viper.GetString("APP_UPDATE_COMMAND"),
		UpdateLatestReleaseURL:          viper.GetString("APP_UPDATE_LATEST_RELEASE_URL"),
	}
}

func setDefault(key string, value any) {
	viper.SetDefault(key, value)
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
