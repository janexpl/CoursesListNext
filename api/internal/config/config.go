package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	DBSSLMode           string
	SessionTTL          time.Duration
	SessionCookieName   string
	SessionCookieSecure bool
	CORSAllowedOrigins  []string
	LoginRateLimit      float64
	GUSUrl              string
	GUSToken            string
}

func Load() Config {
	loadDotEnv()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	dbhost := os.Getenv("DB_HOST")
	if dbhost == "" {
		dbhost = "localhost"
	}
	dbport := os.Getenv("DB_PORT")
	if dbport == "" {
		dbport = "5432"
	}
	dbuser := os.Getenv("DB_USER")
	if dbuser == "" {
		dbuser = "postgres"
	}
	dbpass := os.Getenv("DB_PASS")
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "courselist"
	}
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}
	sessionTTL := os.Getenv("SESSION_TTL")
	if sessionTTL == "" {
		sessionTTL = "24h"
	}
	sessionCookieName := os.Getenv("SESSION_COOKIE_NAME")
	if sessionCookieName == "" {
		sessionCookieName = "session_token"
	}
	sessionCookieSecure := os.Getenv("SESSION_COOKIE_SECURE")
	if sessionCookieSecure == "" {
		sessionCookieSecure = "false"
	}
	sessionTTLDuration, err := time.ParseDuration(sessionTTL)
	if err != nil {
		log.Fatalf("Invalid SESSION_TTL value: %v", err)
	}
	sessionCookieSecureBool, err := strconv.ParseBool(sessionCookieSecure)
	if err != nil {
		log.Fatalf("Invalid SESSION_COOKIE_SECURE value: %v", err)
	}

	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	var corsOriginsList []string
	if corsOrigins != "" {
		for _, origin := range strings.Split(corsOrigins, ",") {
			if trimmed := strings.TrimSpace(origin); trimmed != "" {
				corsOriginsList = append(corsOriginsList, trimmed)
			}
		}
	} else {
		corsOriginsList = []string{"http://localhost:3000"}
	}

	loginRateLimit := 5.0
	if v := os.Getenv("LOGIN_RATE_LIMIT"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			loginRateLimit = parsed
		}
	}
	gusUrl := os.Getenv("GUS_URL")
	if gusUrl == "" {
		gusUrl = "https://wyszukiwarkaregontest.stat.gov.pl/wsbir/uslugabirzewnpubl.svc"
	}
	gusToken := strings.TrimSpace(os.Getenv("GUS_TOKEN"))

	return Config{
		Port:                port,
		DBHost:              dbhost,
		DBPort:              dbport,
		DBUser:              dbuser,
		DBPassword:          dbpass,
		DBName:              dbname,
		DBSSLMode:           dbSSLMode,
		SessionTTL:          sessionTTLDuration,
		SessionCookieName:   sessionCookieName,
		SessionCookieSecure: sessionCookieSecureBool,
		CORSAllowedOrigins:  corsOriginsList,
		LoginRateLimit:      loginRateLimit,
		GUSUrl:              gusUrl,
		GUSToken:            gusToken,
	}
}

func loadDotEnv() {
	candidates := []string{
		".env",
		filepath.Join("next", "api", ".env"),
	}

	for _, candidate := range candidates {
		if err := godotenv.Load(candidate); err == nil {
			return
		}
	}
}
