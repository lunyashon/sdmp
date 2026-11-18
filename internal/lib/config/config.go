package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	LogLevel            string
	LogFormat           string
	LogOutput           string
	LogOutputPath       string
	LogOutputMaxSize    int
	LogOutputMaxAge     int
	LogOutputMaxBackups int
	ReadHeaderTimeout   int
	ReadTimeout         int
	WriteTimeout        int
	IdleTimeout         int

	S3AccessKey string
	S3SecretKey string
	S3Endpoint  string
	S3Region    string
	S3Bucket    string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBMaxConns int

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	PhoneSearchURL   string
	PhoneSearchToken string
	ZaryaURL         string
	ZaryaBitrixURL   string
	// ZaryaForceClean — не ходить на crm-zennit.ru, писать «Чистый». Временно, пока Beget лежит.
	ZaryaForceClean bool

	SkorozvonLogin  string
	SkorozvonAPIKey string
	SkorozvonAppID  string
	SkorozvonAppKey string
	// SkorozvonRPS — исходящие запросы в СК. У них лимит 10/сек. 0 выключает.
	SkorozvonRPS int

	CrmcallsAPIKey  string
	CrmcallsBaseURL string
	CrmcallsRPS     int
	// CrmcallsPostbackKey — X-API-KEY на /rest/crmcalls/events. Пустой секрет = приём закрыт.
	CrmcallsPostbackKey string

	SodatechBx24URL string

	// PublicBaseURL — внешний хост API (ссылка на запись в Битрикс).
	PublicBaseURL string
	// RecordingLinkSecret — HMAC для /rest/recordings/{uuid}.mp3.
	RecordingLinkSecret string

	SwaggerVersion string
	SwaggerToken   string

	// AuthLogin / AuthPassword — единственная учётка UI. Пустой пароль выключает проверку.
	AuthLogin    string
	AuthPassword string

	Gomaxprocs int

	// CRMLeadAddRPS — лимит crm.lead.add на токен источника. 0 выключает.
	CRMLeadAddRPS   int
	CRMLeadAddBurst int
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "config/.env"
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("stat config: %w", err)
	}
	if info.IsDir() {
		path = filepath.Join(path, ".env")
	}

	if err := godotenv.Load(path); err != nil {
		return nil, fmt.Errorf("load env file %s: %w", path, err)
	}

	return getConfig(), nil
}

func getConfig() *Config {
	return &Config{
		Port:                envString("PORT", "8080"),
		LogLevel:            envString("LOG_LEVEL", "info"),
		LogFormat:           envString("LOG_FORMAT", "json"),
		LogOutput:           envString("LOG_OUTPUT", "stdout"),
		LogOutputPath:       envString("LOG_OUTPUT_PATH", "./logs/app.log"),
		LogOutputMaxSize:    envInt("LOG_OUTPUT_MAX_SIZE", 10),
		LogOutputMaxAge:     envInt("LOG_OUTPUT_MAX_AGE", 7),
		LogOutputMaxBackups: envInt("LOG_OUTPUT_MAX_BACKUPS", 10),
		ReadHeaderTimeout:   envInt("READ_HEADER_TIMEOUT", 5),
		ReadTimeout:         envInt("READ_TIMEOUT", 10),
		WriteTimeout:        envInt("WRITE_TIMEOUT", 10),
		IdleTimeout:         envInt("IDLE_TIMEOUT", 30),
		S3AccessKey:         envString("YA_S3_IDENTIFIED_KEY", ""),
		S3SecretKey:         envString("YA_S3_SECRET_KEY", ""),
		S3Endpoint:          envString("YA_S3_ENDPOINT", "https://storage.yandexcloud.net"),
		S3Region:            envString("YA_S3_REGION", "ru-central1"),
		S3Bucket:            envString("YA_S3_BUCKET", ""),
		DBHost:              envString("DB_HOST", "127.0.0.1"),
		DBPort:              envString("DB_PORT", "5432"),
		DBUser:              envString("DB_USER", "admin"),
		DBPassword:          envString("DB_PASS", ""),
		DBName:              envString("DB_NAME", "sdmp"),
		DBMaxConns:          envInt("DB_MAX_CONNS", 32),
		RedisAddr:           envString("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:       envString("REDIS_PASSWORD", ""),
		RedisDB:             envInt("REDIS_DB", 0),
		PhoneSearchURL:      envString("PHONE_SEARCH_URL", "https://bfl-activity.ru/api/v1/retarget/phone.search"),
		PhoneSearchToken:    envFirst("PHONE_SEARCH_SECRET_TOKEN", "PHONE_SEARCH_SECRE_TOKEN"),
		ZaryaURL:            envString("ZARYA_API_URL", ""),
		ZaryaBitrixURL:      envString("ZARYA_BITRIX_URL", ""),
		ZaryaForceClean:     envBool("ZARYA_FORCE_CLEAN"),
		Gomaxprocs:          envInt("GOMAXPROCS", 3),
		SkorozvonLogin:      envString("SKOROZVON_LOGIN", ""),
		SkorozvonAPIKey:     envString("SKOROZVON_API_KEY", ""),
		SkorozvonAppID:      envString("SKOROZVON_APP_ID", ""),
		SkorozvonAppKey:     envString("SKOROZVON_APP_KEY", ""),
		SkorozvonRPS:        envInt("SKOROZVON_RPS", 8),
		CrmcallsAPIKey:      envString("CRMCALLS_API_KEY", ""),
		CrmcallsBaseURL:     envString("CRMCALLS_BASE_URL", "https://cc.crmcalls.ru"),
		CrmcallsRPS:         envInt("CRMCALLS_RPS", 8),
		CrmcallsPostbackKey: envString("CRMCALLS_POSTBACK_KEY", ""),
		SodatechBx24URL:      envString("SODATECH_BX24_URL", ""),
		PublicBaseURL:       envString("PUBLIC_BASE_URL", "https://cdp-dmp.ru"),
		RecordingLinkSecret: envString("RECORDING_LINK_SECRET", ""),
		SwaggerVersion:      envString("SWAGGER_VERSION", "v1"),
		SwaggerToken:        envString("SWAGGER_TOKEN", ""),
		AuthLogin:           envString("AUTH_LOGIN", ""),
		AuthPassword:        envString("AUTH_PASSWORD", ""),
		CRMLeadAddRPS:       envInt("CRM_LEAD_ADD_RPS", 100),
		CRMLeadAddBurst:     envInt("CRM_LEAD_ADD_BURST", 32),
	}
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
