package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

// SignatureHeader niesie HMAC-SHA256 surowego ciała (hex, małe litery).
const SignatureHeader = "X-Az-Signature"

// Config steruje doręczaniem.
type Config struct {
	// PollInterval - jak często dispatcher sprawdza kolejkę, gdy nie dostał powiadomienia.
	PollInterval time.Duration
	// RequestTimeout - limit czasu jednej próby; przekroczenie liczy się jak brak odpowiedzi.
	RequestTimeout time.Duration
	// RetryDelays - odstępy przed kolejnymi próbami po błędzie przejściowym. Liczba prób
	// to len(RetryDelays)+1; po ostatniej doręczenie przechodzi w stan failed.
	RetryDelays []time.Duration
	// BatchSize - ile doręczeń pobiera jeden cykl.
	BatchSize int
}

// DefaultConfig: 8 prób w ciągu ok. 8 godzin (1 min, 5 min, 15 min, 30 min, 1 h, 2 h, 4 h).
func DefaultConfig() Config {
	return Config{
		PollInterval:   5 * time.Second,
		RequestTimeout: 10 * time.Second,
		RetryDelays: []time.Duration{
			time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute,
			time.Hour, 2 * time.Hour, 4 * time.Hour,
		},
		BatchSize: 20,
	}
}

// Dispatcher wysyła zdarzenia z tabeli webhook_deliveries. Kilka instancji API może działać
// równolegle - doręczenia są pobierane z FOR UPDATE SKIP LOCKED i dzierżawą.
type Dispatcher struct {
	pool    *pgxpool.Pool
	queries *dbsqlc.Queries
	cfg     Config
	client  *http.Client
}

func NewDispatcher(pool *pgxpool.Pool, cfg Config) *Dispatcher {
	defaults := DefaultConfig()
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaults.PollInterval
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaults.RequestTimeout
	}
	if cfg.RetryDelays == nil {
		cfg.RetryDelays = defaults.RetryDelays
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaults.BatchSize
	}
	return &Dispatcher{
		pool:    pool,
		queries: dbsqlc.New(pool),
		cfg:     cfg,
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
			// Przekierowanie to błędna konfiguracja adresu, nie powód do wysłania ciała gdzie indziej.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Sign zwraca wartość nagłówka X-Az-Signature dla surowego ciała.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Run doręcza zdarzenia do zakończenia ctx. Budzi się na NOTIFY webhook_deliveries (wysyłane
// przy zatwierdzeniu transakcji z nowym zdarzeniem) albo co PollInterval.
func (d *Dispatcher) Run(ctx context.Context) {
	wake := make(chan struct{}, 1)
	go d.listen(ctx, wake)

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()
	for {
		d.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-wake:
		}
	}
}

func (d *Dispatcher) listen(ctx context.Context, wake chan<- struct{}) {
	for ctx.Err() == nil {
		if err := d.listenOnce(ctx, wake); err != nil && ctx.Err() == nil {
			log.Printf("webhooks: listen failed, falling back to polling: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(d.cfg.PollInterval):
			}
		}
	}
}

func (d *Dispatcher) listenOnce(ctx context.Context, wake chan<- struct{}) error {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN webhook_deliveries"); err != nil {
		return err
	}
	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			// Połączenie po przerwanym oczekiwaniu nie nadaje się do puli.
			if closeErr := conn.Conn().Close(context.Background()); closeErr != nil {
				log.Printf("webhooks: closing listen connection: %v", closeErr)
			}
			return err
		}
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

// drain wysyła, dopóki są gotowe doręczenia.
func (d *Dispatcher) drain(ctx context.Context) {
	lease := (d.cfg.RequestTimeout + 30*time.Second).Seconds()
	for ctx.Err() == nil {
		batch, err := d.queries.ClaimWebhookDeliveries(ctx, dbsqlc.ClaimWebhookDeliveriesParams{
			BatchSize:    int32(d.cfg.BatchSize),
			LeaseSeconds: lease,
		})
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("webhooks: claiming deliveries failed: %v", err)
			}
			return
		}
		if len(batch) == 0 {
			return
		}
		var wg sync.WaitGroup
		for _, delivery := range batch {
			wg.Add(1)
			go func() {
				defer wg.Done()
				d.deliver(ctx, delivery)
			}()
		}
		wg.Wait()
	}
}

func (d *Dispatcher) deliver(ctx context.Context, delivery dbsqlc.ClaimWebhookDeliveriesRow) {
	statusCode, sendErr := d.send(ctx, delivery)
	if ctx.Err() != nil {
		// Zamykanie API: dzierżawa wygaśnie i doręczenie wróci do kolejki.
		return
	}

	// Zapis wyniku nie może zależeć od ctx dispatchera - próba już się odbyła.
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code := pgtype.Int4{}
	if statusCode > 0 {
		code = pgtype.Int4{Int32: int32(statusCode), Valid: true}
	}
	lastError := pgtype.Text{}
	if sendErr != nil {
		lastError = pgtype.Text{String: sendErr.Error(), Valid: true}
	}

	var err error
	switch {
	case sendErr == nil && statusCode >= 200 && statusCode < 300:
		err = d.queries.MarkWebhookDelivered(writeCtx, dbsqlc.MarkWebhookDeliveredParams{
			ID:         delivery.ID,
			StatusCode: code,
		})
	case isRetryable(statusCode, sendErr) && int(delivery.Attempts) <= len(d.cfg.RetryDelays):
		delay := d.cfg.RetryDelays[delivery.Attempts-1]
		log.Printf("webhooks: delivery %d (%s) attempt %d failed (status %d, error %v), retrying in %s",
			delivery.ID, delivery.EventType, delivery.Attempts, statusCode, sendErr, delay)
		err = d.queries.MarkWebhookRetry(writeCtx, dbsqlc.MarkWebhookRetryParams{
			ID:           delivery.ID,
			DelaySeconds: delay.Seconds(),
			StatusCode:   code,
			LastError:    lastError,
		})
	default:
		// Błąd trwały (400, 401, 422 i inne kody spoza 2xx/5xx) albo wyczerpane próby.
		// Nie ponawiamy - to wymaga reakcji operatora.
		log.Printf("WEBHOOK ALERT: delivery %d (%s) to endpoint %d failed permanently after %d attempt(s): status %d, error %v",
			delivery.ID, delivery.EventType, delivery.EndpointID, delivery.Attempts, statusCode, sendErr)
		err = d.queries.MarkWebhookFailed(writeCtx, dbsqlc.MarkWebhookFailedParams{
			ID:         delivery.ID,
			StatusCode: code,
			LastError:  lastError,
		})
	}
	if err != nil {
		log.Printf("webhooks: recording result of delivery %d failed: %v", delivery.ID, err)
	}
}

// isRetryable: ponawiamy wyłącznie brak odpowiedzi (błąd sieci, timeout) i 5xx.
func isRetryable(statusCode int, sendErr error) bool {
	if sendErr != nil {
		return true
	}
	return statusCode >= 500 && statusCode <= 599
}

func (d *Dispatcher) send(ctx context.Context, delivery dbsqlc.ClaimWebhookDeliveriesRow) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.Url, bytes.NewReader(delivery.Payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(delivery.Secret, delivery.Payload))
	req.Header.Set("User-Agent", "CoursesList-Webhooks/1")
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("webhooks: reading response of delivery %d: %v", delivery.ID, err)
	}
	return resp.StatusCode, nil
}
