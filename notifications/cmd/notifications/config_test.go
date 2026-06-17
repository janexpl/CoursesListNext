package main

import (
	"strings"
	"testing"
)

// setBaseSendingEnv configures the minimum environment for a non-dry-run
// configuration; individual tests override SMTP_USERNAME / SMTP_TLS_MODE.
func setBaseSendingEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NOTIFICATIONS_API_BASE_URL", "https://api.example.com")
	t.Setenv("NOTIFICATIONS_API_TOKEN", "token")
	t.Setenv("NOTIFICATIONS_DRY_RUN", "false")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "biuro@example.com")
}

func TestLoadConfigRejectsUsernameWithoutTLS(t *testing.T) {
	setBaseSendingEnv(t)
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_TLS_MODE", "none")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for SMTP_USERNAME with SMTP_TLS_MODE=none")
	}
	if !strings.Contains(err.Error(), "SMTP_USERNAME") {
		t.Fatalf("expected error to mention SMTP_USERNAME, got %v", err)
	}
}

func TestLoadConfigAllowsUsernameWithTLS(t *testing.T) {
	for _, mode := range []string{"starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			setBaseSendingEnv(t)
			t.Setenv("SMTP_USERNAME", "mailer")
			t.Setenv("SMTP_TLS_MODE", mode)

			if _, err := loadConfig(); err != nil {
				t.Fatalf("expected no error for SMTP_TLS_MODE=%s, got %v", mode, err)
			}
		})
	}
}

func TestLoadConfigAllowsUsernameWithoutTLSInDryRun(t *testing.T) {
	t.Setenv("NOTIFICATIONS_API_BASE_URL", "https://api.example.com")
	t.Setenv("NOTIFICATIONS_API_TOKEN", "token")
	t.Setenv("NOTIFICATIONS_DRY_RUN", "true")
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_TLS_MODE", "none")

	if _, err := loadConfig(); err != nil {
		t.Fatalf("dry run must not enforce the TLS requirement, got %v", err)
	}
}
