package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const dateFormat = "2006-01-02"

// stateRetention is how long a sent-notification record is kept after the
// certificate's expiry date before it is pruned from the state file. Once a
// certificate has expired no further notifications are sent for it, so the
// record only needs to outlive the expiry by a safety margin.
const stateRetention = 90 * 24 * time.Hour

type Config struct {
	APIBaseURL    string
	APIToken      string
	LookaheadDays int
	Limit         int
	StateFile     string
	DryRun        bool
	RunInterval   time.Duration
	HTTPTimeout   time.Duration
	SubjectPrefix string
	SMTP          SMTPConfig
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	FromName string
	TLSMode  string
	Timeout  time.Duration
}

type CandidateResponse struct {
	Data []CertificateCandidate `json:"data"`
	Meta CandidateMeta          `json:"meta"`
}

type CandidateMeta struct {
	Limit      int              `json:"limit"`
	HasMore    bool             `json:"hasMore"`
	NextCursor *CandidateCursor `json:"nextCursor"`
}

type CandidateCursor struct {
	AfterExpiryDate    string `json:"afterExpiryDate"`
	AfterCertificateID int64  `json:"afterCertificateId"`
}

type CertificateCandidate struct {
	CertificateID   int64            `json:"certificateId"`
	CertificateDate string           `json:"certificateDate"`
	ExpiryDate      string           `json:"expiryDate"`
	RegistryYear    int64            `json:"registryYear"`
	RegistryNumber  int64            `json:"registryNumber"`
	LanguageCode    string           `json:"languageCode"`
	Student         CandidateStudent `json:"student"`
	Company         CandidateCompany `json:"company"`
	Course          CandidateCourse  `json:"course"`
}

type CandidateStudent struct {
	ID        int32   `json:"id"`
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	PESEL     *string `json:"pesel"`
}

type CandidateCompany struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CurrentName    string `json:"currentName"`
	RecipientEmail string `json:"recipientEmail"`
}

type CandidateCourse struct {
	Name      string  `json:"name"`
	Symbol    string  `json:"symbol"`
	DateStart string  `json:"dateStart"`
	DateEnd   *string `json:"dateEnd"`
}

type State struct {
	Sent map[string]SentNotification `json:"sent"`
}

type SentNotification struct {
	SentAt         string `json:"sentAt"`
	RecipientEmail string `json:"recipientEmail"`
	CompanyID      int64  `json:"companyId"`
}

type CompanyBatch struct {
	CompanyID      int64
	CompanyName    string
	RecipientEmail string
	Candidates     []CertificateCandidate
}

type EmailMessage struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, now func() time.Time, msg EmailMessage) error
}

type SMTPMailer struct {
	cfg SMTPConfig
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	worker := Worker{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.HTTPTimeout},
		mailer: SMTPMailer{cfg: cfg.SMTP},
		now:    time.Now,
	}

	log.Printf(
		"notifications configured: api=%s lookaheadDays=%d limit=%d dryRun=%t stateFile=%s runInterval=%s",
		cfg.APIBaseURL,
		cfg.LookaheadDays,
		cfg.Limit,
		cfg.DryRun,
		cfg.StateFile,
		cfg.RunInterval,
	)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.RunInterval <= 0 {
		if err := worker.RunOnce(ctx); err != nil {
			log.Fatalf("notification run failed: %v", err)
		}
		return
	}

	for {
		if err := worker.RunOnce(ctx); err != nil {
			log.Printf("notification run failed: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Printf("shutting down notifications: %v", ctx.Err())
			return
		case <-time.After(cfg.RunInterval):

		}
	}
}

type Worker struct {
	cfg    Config
	client *http.Client
	mailer Mailer
	now    func() time.Time
}

func (w Worker) RunOnce(ctx context.Context) error {
	today := truncateDate(w.now())
	dateFrom := today.Format(dateFormat)
	dateTo := today.AddDate(0, 0, w.cfg.LookaheadDays).Format(dateFormat)

	candidates, err := fetchCandidates(ctx, w.client, w.cfg, dateFrom, dateTo)
	if err != nil {
		return err
	}
	log.Printf("fetched %d certificate notification candidates for %s..%s", len(candidates), dateFrom, dateTo)

	state, err := loadState(w.cfg.StateFile)
	if err != nil {
		return err
	}
	if removed := pruneState(state, today, stateRetention); removed > 0 {
		log.Printf("pruned %d expired notification state entries", removed)
	}
	if err := saveState(w.cfg.StateFile, state); err != nil {
		return err
	}

	batches, err := buildCompanyBatches(candidates, state, w.cfg.LookaheadDays)
	if err != nil {
		return err
	}
	if len(batches) == 0 {
		log.Printf("no pending notifications for %s..%s", dateFrom, dateTo)
		return nil
	}
	var sendErrs []error
	for _, batch := range batches {
		if ctx.Err() != nil {
			sendErrs = append(sendErrs, ctx.Err())
			break
		}
		message := buildEmailMessage(batch, w.cfg, dateFrom, dateTo)
		if w.cfg.DryRun {
			log.Printf("dry run: would send %d certificate notifications to %s <%s>; state is not marked as sent", len(batch.Candidates), batch.CompanyName, batch.RecipientEmail)
			continue
		}

		if err := w.mailer.Send(ctx, w.now, message); err != nil {
			log.Printf("failed to send to %s <%s>: %v", batch.CompanyName, batch.RecipientEmail, err)
			sendErrs = append(sendErrs, fmt.Errorf("send to %s: %w", batch.RecipientEmail, err))
			continue
		}

		sentAt := w.now().UTC().Format(time.RFC3339)
		for _, candidate := range batch.Candidates {
			state.Sent[recipientNotificationKey(candidate, batch.RecipientEmail)] = SentNotification{
				SentAt:         sentAt,
				RecipientEmail: batch.RecipientEmail,
				CompanyID:      batch.CompanyID,
			}
		}
		if err := saveState(w.cfg.StateFile, state); err != nil {
			log.Printf("failed to persist state after %s <%s>: %v", batch.CompanyName, batch.RecipientEmail, err)
			sendErrs = append(sendErrs, fmt.Errorf("save state after %s: %w", batch.RecipientEmail, err))
		}
		log.Printf("sent %d certificate notifications to %s <%s>; state entries=%d", len(batch.Candidates), batch.CompanyName, batch.RecipientEmail, len(state.Sent))
	}

	return errors.Join(sendErrs...)
}

func loadConfig() (Config, error) {
	cfg := Config{
		APIBaseURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("NOTIFICATIONS_API_BASE_URL")), "/"),
		APIToken:      strings.TrimSpace(os.Getenv("NOTIFICATIONS_API_TOKEN")),
		LookaheadDays: intEnv("NOTIFICATIONS_LOOKAHEAD_DAYS", 30),
		Limit:         intEnv("NOTIFICATIONS_LIMIT", 500),
		StateFile:     stringEnv("NOTIFICATIONS_STATE_FILE", "/data/notifications-state.json"),
		DryRun:        boolEnv("NOTIFICATIONS_DRY_RUN", true),
		HTTPTimeout:   durationEnv("NOTIFICATIONS_HTTP_TIMEOUT", 30*time.Second),
		RunInterval:   durationEnv("NOTIFICATIONS_RUN_INTERVAL", 0),
		SubjectPrefix: strings.TrimSpace(os.Getenv("NOTIFICATIONS_SUBJECT_PREFIX")),
		SMTP: SMTPConfig{
			Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
			Port:     stringEnv("SMTP_PORT", "587"),
			Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password: strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
			From:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
			FromName: stringEnv("SMTP_FROM_NAME", "Powiadomienia BHP"),
			TLSMode:  strings.ToLower(stringEnv("SMTP_TLS_MODE", "auto")),
			Timeout:  durationEnv("SMTP_TIMEOUT", 30*time.Second),
		},
	}

	if cfg.APIBaseURL == "" {
		return Config{}, errors.New("NOTIFICATIONS_API_BASE_URL is required")
	}
	if cfg.APIToken == "" {
		return Config{}, errors.New("NOTIFICATIONS_API_TOKEN is required")
	}
	if cfg.LookaheadDays < 0 {
		return Config{}, errors.New("NOTIFICATIONS_LOOKAHEAD_DAYS must be non-negative")
	}
	if cfg.Limit <= 0 || cfg.Limit > 1000 {
		return Config{}, errors.New("NOTIFICATIONS_LIMIT must be between 1 and 1000")
	}
	if cfg.HTTPTimeout <= 0 {
		return Config{}, errors.New("NOTIFICATIONS_HTTP_TIMEOUT must be positive")
	}
	if cfg.SMTP.TLSMode != "auto" && cfg.SMTP.TLSMode != "starttls" && cfg.SMTP.TLSMode != "tls" && cfg.SMTP.TLSMode != "none" {
		return Config{}, errors.New("SMTP_TLS_MODE must be auto, starttls, tls, or none")
	}
	if !cfg.DryRun {
		if cfg.SMTP.Host == "" || cfg.SMTP.From == "" {
			return Config{}, errors.New("SMTP_HOST and SMTP_FROM are required when NOTIFICATIONS_DRY_RUN=false")
		}
		if cfg.SMTP.Username != "" && cfg.SMTP.TLSMode == "none" {
			return Config{}, errors.New("SMTP_USERNAME requires SMTP_TLS_MODE starttls or tls; refusing to send credentials over an unencrypted connection")
		}
		if cfg.SMTP.Username != "" && cfg.SMTP.TLSMode == "auto" {
			log.Printf("warning: SMTP_USERNAME is set with SMTP_TLS_MODE=auto; authentication will fail if the server does not advertise STARTTLS")
		}
	}
	if cfg.SMTP.Timeout <= 0 {
		return Config{}, errors.New("SMTP_TIMEOUT must be positive")
	}

	return cfg, nil
}

func fetchCandidates(ctx context.Context, client *http.Client, cfg Config, dateFrom, dateTo string) ([]CertificateCandidate, error) {
	var all []CertificateCandidate
	var cursor *CandidateCursor

	for {
		endpoint, err := url.Parse(cfg.APIBaseURL + "/api/v1/internal/notifications/expiring-certificates")
		if err != nil {
			return nil, err
		}

		query := endpoint.Query()
		query.Set("dateFrom", dateFrom)
		query.Set("dateTo", dateTo)
		query.Set("limit", strconv.Itoa(cfg.Limit))
		if cursor != nil {
			query.Set("afterExpiryDate", cursor.AfterExpiryDate)
			query.Set("afterCertificateId", strconv.FormatInt(cursor.AfterCertificateID, 10))
		}
		endpoint.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var decoded CandidateResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			return nil, err
		}
		all = append(all, decoded.Data...)

		if !decoded.Meta.HasMore || decoded.Meta.NextCursor == nil {
			break
		}
		cursor = decoded.Meta.NextCursor
	}

	return all, nil
}

func buildCompanyBatches(candidates []CertificateCandidate, state State, lookaheadDays int) ([]CompanyBatch, error) {
	byKey := make(map[string]*CompanyBatch)

	for _, candidate := range candidates {
		if candidate.Company.RecipientEmail == "" {
			continue
		}
		recipients, err := parseRecipientEmails(candidate.Company.RecipientEmail)
		if err != nil {
			return nil, fmt.Errorf(
				"parse notification recipients for company %d: %w",
				candidate.Company.ID,
				err,
			)
		}

		for _, recipient := range recipients {
			if wasNotificationSent(state, candidate, recipient, lookaheadDays) {
				continue
			}

			key := fmt.Sprintf("%d:%s", candidate.Company.ID, strings.ToLower(recipient))
			batch, exists := byKey[key]
			if !exists {
				companyName := strings.TrimSpace(candidate.Company.Name)
				if companyName == "" {
					companyName = candidate.Company.CurrentName
				}
				batch = &CompanyBatch{
					CompanyID:      candidate.Company.ID,
					CompanyName:    companyName,
					RecipientEmail: recipient,
				}
				byKey[key] = batch
			}
			batch.Candidates = append(batch.Candidates, candidate)
		}
	}

	batches := make([]CompanyBatch, 0, len(byKey))
	for _, batch := range byKey {
		sort.Slice(batch.Candidates, func(i, j int) bool {
			if batch.Candidates[i].ExpiryDate == batch.Candidates[j].ExpiryDate {
				return batch.Candidates[i].CertificateID < batch.Candidates[j].CertificateID
			}
			return batch.Candidates[i].ExpiryDate < batch.Candidates[j].ExpiryDate
		})
		batches = append(batches, *batch)
	}
	sort.Slice(batches, func(i, j int) bool {
		if batches[i].CompanyName == batches[j].CompanyName {
			return batches[i].RecipientEmail < batches[j].RecipientEmail
		}
		return batches[i].CompanyName < batches[j].CompanyName
	})

	return batches, nil
}

func buildEmailMessage(batch CompanyBatch, cfg Config, dateFrom, dateTo string) EmailMessage {
	subject := fmt.Sprintf("Zaświadczenia wygasające w ciągu %d dni - %s", cfg.LookaheadDays, batch.CompanyName)
	if cfg.SubjectPrefix != "" {
		subject = cfg.SubjectPrefix + " " + subject
	}

	var body strings.Builder
	body.WriteString("<!doctype html><html><body style=\"font-family:Arial,sans-serif;color:#111827;line-height:1.4;\">")
	body.WriteString("<p>Dzień dobry,</p>")
	fmt.Fprintf(
		&body,
		"<p>Poniżej znajduje się lista zaświadczeń dla firmy <strong>%s</strong>, których ważność kończy się w okresie <strong>%s - %s</strong>.</p>",
		html.EscapeString(batch.CompanyName),
		html.EscapeString(dateFrom),
		html.EscapeString(dateTo),
	)
	body.WriteString("<table style=\"border-collapse:collapse;width:100%;max-width:960px;\">")
	body.WriteString("<thead><tr>")
	body.WriteString("<th style=\"border:1px solid #d1d5db;padding:8px;text-align:left;background:#f3f4f6;\">Lp.</th>")
	body.WriteString("<th style=\"border:1px solid #d1d5db;padding:8px;text-align:left;background:#f3f4f6;\">Uczestnik</th>")
	body.WriteString("<th style=\"border:1px solid #d1d5db;padding:8px;text-align:left;background:#f3f4f6;\">Kurs</th>")
	body.WriteString("<th style=\"border:1px solid #d1d5db;padding:8px;text-align:left;background:#f3f4f6;\">Nr z rejestru</th>")
	body.WriteString("<th style=\"border:1px solid #d1d5db;padding:8px;text-align:left;background:#f3f4f6;\">Ważne do</th>")
	body.WriteString("</tr></thead><tbody>")

	for index, candidate := range batch.Candidates {
		studentName := strings.TrimSpace(candidate.Student.FirstName + " " + candidate.Student.LastName)
		fmt.Fprintf(
			&body,
			"<tr><td style=\"border:1px solid #d1d5db;padding:8px;\">%d</td><td style=\"border:1px solid #d1d5db;padding:8px;\">%s</td><td style=\"border:1px solid #d1d5db;padding:8px;\">%s</td><td style=\"border:1px solid #d1d5db;padding:8px;white-space:nowrap;\">%s</td><td style=\"border:1px solid #d1d5db;padding:8px;white-space:nowrap;\">%s</td></tr>",
			index+1,
			html.EscapeString(studentName),
			html.EscapeString(candidate.Course.Name),
			html.EscapeString(formatRegistryNumber(candidate)),
			html.EscapeString(candidate.ExpiryDate),
		)
	}

	body.WriteString("</tbody></table>")
	body.WriteString("<p>Jeżeli są Państwo zainteresowani przedłużeniem ważności zaświadczeń prosimy o kontakt z nami pod numerem 600969600 lub na email biuro@naszaera.pl</p>")
	body.WriteString("<p style=\"color:#6b7280;\">Wiadomość została wygenerowana automatycznie.</p>")
	body.WriteString("</body></html>")

	return EmailMessage{
		To:      batch.RecipientEmail,
		Subject: subject,
		Body:    body.String(),
	}
}

func formatRegistryNumber(candidate CertificateCandidate) string {
	symbol := strings.TrimSpace(candidate.Course.Symbol)
	number := fmt.Sprintf("%d/%d", candidate.RegistryNumber, candidate.RegistryYear)
	if symbol == "" {
		return number
	}
	return symbol + " " + number
}

func (m SMTPMailer) Send(ctx context.Context, now func() time.Time, msg EmailMessage) error {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	from := mail.Address{Name: m.cfg.FromName, Address: m.cfg.From}
	to := mail.Address{Address: msg.To}

	var data bytes.Buffer
	data.WriteString("Date: ")
	data.WriteString(now().Format(time.RFC1123Z))
	data.WriteString("\r\n")
	data.WriteString("Message-ID: ")
	data.WriteString(newMessageID(m.cfg.From))
	data.WriteString("\r\n")
	data.WriteString("From: ")
	data.WriteString(from.String())
	data.WriteString("\r\n")
	data.WriteString("To: ")
	data.WriteString(to.String())
	data.WriteString("\r\n")
	data.WriteString("Subject: ")
	data.WriteString(mime.QEncoding.Encode("utf-8", msg.Subject))
	data.WriteString("\r\n")
	data.WriteString("MIME-Version: 1.0\r\n")
	data.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	data.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	data.WriteString("\r\n")

	qp := quotedprintable.NewWriter(&data)
	if _, err := qp.Write([]byte(msg.Body)); err != nil {
		return err
	}
	if err := qp.Close(); err != nil {
		return err
	}

	address := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	client, err := dialSMTP(ctx, m.cfg, address)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Hello("localhost"); err != nil {
		return err
	}

	if m.cfg.TLSMode == "starttls" || m.cfg.TLSMode == "auto" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		} else if m.cfg.TLSMode == "starttls" {
			return errors.New("SMTP server does not support STARTTLS")
		}
	}

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(m.cfg.From); err != nil {
		return err
	}
	if err := client.Rcpt(msg.To); err != nil {
		return err
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(data.Bytes()); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func newMessageID(from string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	domain := "localhost"
	if i := strings.LastIndex(from, "@"); i >= 0 && i < len(from)-1 {
		domain = from[i+1:]
	}
	return "<" + hex.EncodeToString(b) + "@" + domain + ">"
}

func dialSMTP(ctx context.Context, cfg SMTPConfig, address string) (*smtp.Client, error) {
	dialer := net.Dialer{Timeout: cfg.Timeout}
	var conn net.Conn
	var err error
	if cfg.TLSMode == "tls" {
		tlsDialer := tls.Dialer{NetDialer: &dialer, Config: &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}}
		conn, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	return smtp.NewClient(conn, cfg.Host)
}

func loadState(path string) (State, error) {
	state := State{Sent: map[string]SentNotification{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return State{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	if state.Sent == nil {
		state.Sent = map[string]SentNotification{}
	}

	return state, nil
}

func saveState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func wasNotificationSent(state State, candidate CertificateCandidate, recipientEmail string, lookaheadDays int) bool {
	if _, exists := state.Sent[recipientNotificationKey(candidate, recipientEmail)]; exists {
		return true
	}
	if _, exists := state.Sent[notificationKey(candidate)]; exists {
		return true
	}
	if _, exists := state.Sent[legacyNotificationKey(candidate, lookaheadDays)]; exists {
		return true
	}
	return false
}

// pruneState removes sent-notification records whose certificate expiry date is
// older than retention before today, keeping the state file from growing without
// bound. Keys begin with "<id>:<expiry>", so the expiry is always the second
// field for current and legacy entries. Unparseable keys are left untouched.
func pruneState(state State, today time.Time, retention time.Duration) int {
	cutoff := today.Add(-retention)
	removed := 0
	for key := range state.Sent {
		parts := strings.Split(key, ":")
		if len(parts) < 2 {
			continue
		}
		expiry, err := time.Parse(dateFormat, parts[1])
		if err != nil {
			continue
		}
		if expiry.Before(cutoff) {
			delete(state.Sent, key)
			removed++
		}
	}
	return removed
}

func notificationKey(candidate CertificateCandidate) string {
	return fmt.Sprintf("%d:%s", candidate.CertificateID, candidate.ExpiryDate)
}

func recipientNotificationKey(candidate CertificateCandidate, recipientEmail string) string {
	return fmt.Sprintf(
		"%d:%s:%s",
		candidate.CertificateID,
		candidate.ExpiryDate,
		strings.ToLower(strings.TrimSpace(recipientEmail)),
	)
}

func legacyNotificationKey(candidate CertificateCandidate, lookaheadDays int) string {
	return fmt.Sprintf("%d:%s:%d", candidate.CertificateID, candidate.ExpiryDate, lookaheadDays)
}

func truncateDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func stringEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseRecipientEmails(value string) ([]string, error) {

	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return []string{}, err
	}
	emails := make([]string, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))

	for _, addr := range addresses {
		email := strings.TrimSpace(addr.Address)
		key := strings.ToLower(email)

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		emails = append(emails, email)
	}

	return emails, nil
}
