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
	Port                   string
	DBHost                 string
	DBPort                 string
	DBUser                 string
	DBPassword             string
	DBName                 string
	DBSSLMode              string
	SessionTTL             time.Duration
	SessionCleanupInterval time.Duration
	SessionCookieName      string
	SessionCookieSecure    bool
	CORSAllowedOrigins     []string
	LoginRateLimit         float64
	GUSUrl                 string
	GUSToken               string
	NotificationsAPIToken  string
	// PublicBaseURL - publiczny adres API (np. https://courseslist.example.pl), z którego
	// webhook buduje pdf_url. Pusty pomija pdf_url.
	PublicBaseURL string
	// WebhooksEnabled - czy ta instancja uruchamia dispatcher webhooków.
	WebhooksEnabled bool
	// CertificateVerificationURL - wzorzec adresu publicznej strony weryfikacji
	// zaświadczenia, np. https://twoja-domena.pl/verify/{code}. Z niego powstaje kod QR
	// drukowany na dokumencie; {code} jest podmieniane na kod weryfikacyjny.
	// Pusty wyłącza QR - na wydruku nie pojawia się nic.
	CertificateVerificationURL string
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

	sessionCleanupInterval := os.Getenv("SESSION_CLEANUP_INTERVAL")
	if sessionCleanupInterval == "" {
		sessionCleanupInterval = "1h"
	}
	sessionCleanupIntervalDuration, err := time.ParseDuration(sessionCleanupInterval)
	if err != nil {
		log.Fatalf("Invalid SESSION_CLEANUP_INTERVAL value: %v", err)
	}
	if sessionCleanupIntervalDuration <= 0 {
		log.Fatalf("SESSION_CLEANUP_INTERVAL must be positive")
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
	gusURL := os.Getenv("GUS_URL")
	if gusURL == "" {
		gusURL = "https://wyszukiwarkaregontest.stat.gov.pl/wsbir/uslugabirzewnpubl.svc"
	}
	gusToken := strings.TrimSpace(os.Getenv("GUS_TOKEN"))

	notificationsAPIToken := strings.TrimSpace(os.Getenv("NOTIFICATIONS_API_TOKEN"))

	// Adres publicznej weryfikacji żyje w aplikacji webowej, a PUBLIC_BASE_URL wskazuje API,
	// więc to osobna zmienna, a nie doklejanie ścieżki do tamtej.
	certificateVerificationURL := strings.TrimSpace(os.Getenv("CERTIFICATE_VERIFICATION_URL"))
	if certificateVerificationURL != "" && !strings.Contains(certificateVerificationURL, "{code}") {
		log.Fatalf("CERTIFICATE_VERIFICATION_URL must contain the {code} placeholder, got %q", certificateVerificationURL)
	}

	webhooksEnabled := true
	if raw := os.Getenv("WEBHOOKS_ENABLED"); raw != "" {
		webhooksEnabled, err = strconv.ParseBool(raw)
		if err != nil {
			log.Fatalf("Invalid WEBHOOKS_ENABLED value: %v", err)
		}
	}

	return Config{
		Port:                   port,
		DBHost:                 dbhost,
		DBPort:                 dbport,
		DBUser:                 dbuser,
		DBPassword:             dbpass,
		DBName:                 dbname,
		DBSSLMode:              dbSSLMode,
		SessionTTL:             sessionTTLDuration,
		SessionCleanupInterval: sessionCleanupIntervalDuration,
		SessionCookieName:      sessionCookieName,
		SessionCookieSecure:    sessionCookieSecureBool,
		CORSAllowedOrigins:     corsOriginsList,
		LoginRateLimit:         loginRateLimit,
		GUSUrl:                 gusURL,
		GUSToken:               gusToken,
		NotificationsAPIToken:  notificationsAPIToken,
		PublicBaseURL:          os.Getenv("PUBLIC_BASE_URL"),
		WebhooksEnabled:        webhooksEnabled,

		CertificateVerificationURL: certificateVerificationURL,
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
