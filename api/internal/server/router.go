// Package server
package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/apikeys"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/certassets"
	"github.com/janexpl/CoursesListNext/api/internal/certificates"
	"github.com/janexpl/CoursesListNext/api/internal/companies"
	"github.com/janexpl/CoursesListNext/api/internal/config"
	"github.com/janexpl/CoursesListNext/api/internal/courses"
	"github.com/janexpl/CoursesListNext/api/internal/dashboard"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/gusclient"
	"github.com/janexpl/CoursesListNext/api/internal/journals"
	"github.com/janexpl/CoursesListNext/api/internal/registries"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"github.com/janexpl/CoursesListNext/api/internal/students"
	"github.com/janexpl/CoursesListNext/api/internal/users"
	"github.com/janexpl/CoursesListNext/api/internal/webhooks"
)

type Dependencies struct {
	Queries *dbsql.Queries
	Config  *config.Config
	Pool    *pgxpool.Pool
}

type Handler struct {
	queries *dbsql.Queries
}

func NewRouter(deps Dependencies) http.Handler {
	h := Handler{queries: deps.Queries}
	recorder := auditlog.NewRecorder()
	studentService := students.NewService(deps.Pool, deps.Queries, recorder)
	studentHandler := students.NewHandler(deps.Queries, studentService)
	companyService := companies.NewService(deps.Pool, deps.Queries, recorder)
	companyHandler := companies.NewHandler(deps.Queries, companyService)
	authHandler := auth.NewHandler(deps.Queries, deps.Config)
	webhookPublisher := webhooks.NewPublisher(deps.Config.PublicBaseURL)
	certificateService := certificates.NewService(deps.Pool, deps.Queries, recorder)
	certificateService.SetWebhookPublisher(webhookPublisher)
	userService := users.NewServiceWithAudit(deps.Pool, deps.Queries, recorder)
	userHandler := users.NewHandler(deps.Queries, userService)
	certificateHandler := certificates.NewHandler(deps.Queries, certificateService)
	certificateHandler.SetVerificationURLTemplate(deps.Config.CertificateVerificationURL)
	certAssetService := certassets.NewService(deps.Pool, deps.Queries, recorder)
	certAssetHandler := certassets.NewHandler(deps.Queries, certAssetService)
	dashboardHandler := dashboard.NewHandler(deps.Queries)
	coursesService := courses.NewService(deps.Pool, deps.Queries, recorder)
	coursesService.SetWebhookPublisher(webhookPublisher)
	courseHandler := courses.NewHandler(deps.Queries, coursesService)
	registryHandler := registries.NewHandler(deps.Queries)
	journalService := journals.NewService(deps.Pool, deps.Queries, recorder)
	journalHandler := journals.NewHandler(deps.Queries, journalService)
	auditLogHandler := auditlog.NewHandler(deps.Queries)
	gusclientHandler := gusclient.NewHandler(deps.Config)
	apiKeyService := apikeys.NewService(deps.Pool, deps.Queries, recorder)
	apiKeyHandler := apikeys.NewHandler(deps.Queries, apiKeyService)
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(limitRequestBody(maxJSONBodyBytes))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.Config.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	loginLimiter := newIPLimiter(deps.Config.LoginRateLimit/60, 10)
	// 20 sprawdzeń na minutę dla jednego dokumentu wystarczy człowiekowi ze skanerem,
	// a odcina młócenie pojedynczego kodu. Sufit całej trasy: 5 żądań na sekundę.
	publicVerificationCodeLimiter := newIPLimiter(20.0/60, 20)
	publicVerificationTotalLimiter := newIPLimiter(5, 50)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", h.healthzHandler)
		r.With(RateLimitByIP(loginLimiter)).Post("/auth/login", authHandler.Login)

		// Publiczna weryfikacja zaświadczenia - adres z kodu QR na wydruku. Jedyna
		// trasa zwracająca dane bez uwierzytelnienia, więc odpowiedź jest zawężona
		// do tego, co widać na dokumencie (certificates.PublicCertificateDTO).
		//
		// Limit po kodzie, nie po adresie IP: przeglądarka woła API przez proxy
		// aplikacji webowej, więc wszystkie żądania mają ten sam adres. Drugi limiter
		// jest sufitem dla całej trasy.
		r.With(
			RateLimitByKey(publicVerificationTotalLimiter, func(*http.Request) string { return "public-verification" }),
			RateLimitByKey(publicVerificationCodeLimiter, certificates.PublicVerificationCodeFromRequest),
		).Get("/public/certificates/{code}", certificateHandler.GetPublicVerification)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireBearerToken(deps.Config.NotificationsAPIToken))
			r.Get("/internal/notifications/expiring-certificates", certificateHandler.ListExpiringNotificationCandidates)
		})
		r.Group(func(r chi.Router) {
			// Ciasteczko sesji obsługuje przeglądarkę, klucz API integracje
			// serwer-serwer. Obie metody kończą się tym samym użytkownikiem
			// w kontekście, więc handlery poniżej nie widzą różnicy.
			r.Use(auth.Authenticate(
				auth.SessionAuthenticator(deps.Queries, deps.Config),
				auth.APIKeyAuthenticator(deps.Queries),
			))

			// Wylogowanie i zarządzanie własnym kontem dotyczą zalogowanego
			// człowieka - klucz API nie ma czego tu szukać.
			r.With(auth.RequireSession()).Post("/auth/logout", authHandler.Logout)
			r.With(auth.RequireSession()).Patch("/account/profile", userHandler.PatchProfile)
			r.With(auth.RequireSession()).Patch("/account/password", userHandler.PatchPassword)

			// Bez zakresu: pozwala integracji sprawdzić, czy klucz działa
			// i na jakim koncie.
			r.Get("/auth/me", authHandler.Me)

			r.With(auth.RequireScope(auth.ScopeStudentsRead)).Get("/students", studentHandler.List)
			r.With(auth.RequireScope(auth.ScopeStudentsRead)).Get("/students/{id}", studentHandler.Get)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/students/{id}/certificates", studentHandler.ListCertificatesByStudent)
			r.With(auth.RequireScope(auth.ScopeStudentsWrite)).Patch("/students/{id}", studentHandler.Patch)
			r.With(auth.RequireScope(auth.ScopeStudentsWrite)).Post("/students", studentHandler.CreateStudent)
			r.With(auth.RequireScope(auth.ScopeStudentsWrite)).Put("/students/by-external-id/{externalId}", studentHandler.PutByExternalID)

			r.With(auth.RequireScope(auth.ScopeCompaniesRead)).Get("/companies", companyHandler.List)
			r.With(auth.RequireScope(auth.ScopeCompaniesRead)).Get("/companies/{id}", companyHandler.Get)
			r.With(auth.RequireScope(auth.ScopeStudentsRead)).Get("/companies/{id}/students", studentHandler.ListStudentsByCompanyId)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/companies/{id}/certificates", certificateHandler.ListByCompanyID)
			r.With(auth.RequireScope(auth.ScopeCompaniesWrite)).Patch("/companies/{id}", companyHandler.Patch)
			r.With(auth.RequireScope(auth.ScopeCompaniesWrite)).Post("/companies", companyHandler.CreateCompany)
			r.With(auth.RequireScope(auth.ScopeCompaniesWrite)).Put("/companies/by-external-id/{externalId}", companyHandler.PutByExternalID)
			r.With(auth.RequireScope(auth.ScopeCompaniesRead)).Get("/companies/lookup-by-nip", gusclientHandler.FindCompany)

			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificates", certificateHandler.List)
			r.With(auth.RequireScope(auth.ScopeCertificatesWrite)).Post("/certificates", certificateHandler.Create)
			// Statyczny segment wyprzedza w chi wzorzec /certificates/{id}.
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificates/by-verification-code/{code}", certificateHandler.GetByVerificationCode)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificates/{id}", certificateHandler.Get)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificates/{id}/pdf", certificateHandler.PDF)
			r.With(auth.RequireScope(auth.ScopeCertificatesWrite)).Patch("/certificates/{id}", certificateHandler.Patch)
			r.With(auth.RequireScope(auth.ScopeCertificatesWrite)).Post("/certificates/{id}/revoke", certificateHandler.Revoke)
			r.With(auth.RequireScope(auth.ScopeCertificatesWrite)).Post("/certificates/{id}/duplicate", certificateHandler.Duplicate)

			// Nadruki zaświadczeń: pieczątki i podpis. Odczyt jest dostępny dla każdego
			// zalogowanego, bo podgląd zaświadczenia w przeglądarce musi pokazać to samo,
			// co wydrukuje serwer. Statyczny "guilloche" wyprzedza wzorzec {kind}.
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificate-print-assets", certAssetHandler.List)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificate-print-assets/guilloche", certAssetHandler.Guilloche)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/certificate-print-assets/{kind}/file", certAssetHandler.GetFile)

			r.With(auth.RequireScope(auth.ScopeDashboardRead)).Get("/dashboard", dashboardHandler.Get)

			r.With(auth.RequireScope(auth.ScopeCoursesRead)).Get("/courses", courseHandler.List)
			// Statyczny segment wyprzedza w chi wzorzec /courses/{id}, więc
			// "details" nie trafi do Get jako identyfikator kursu.
			r.With(auth.RequireScope(auth.ScopeCoursesRead)).Get("/courses/details", courseHandler.ListDetails)
			r.With(auth.RequireScope(auth.ScopeCoursesRead)).Get("/courses/{id}", courseHandler.Get)
			r.With(auth.RequireScope(auth.ScopeCoursesWrite)).Patch("/courses/{id}", courseHandler.Patch)
			r.With(auth.RequireScope(auth.ScopeCoursesWrite)).Post("/courses", courseHandler.CreateCourse)
			r.With(auth.RequireScope(auth.ScopeCertificatesRead)).Get("/courses/{id}/certificates", certificateHandler.ListByCourseID)
			r.With(auth.RequireScope(auth.ScopeCoursesRead)).Get("/courses/{id}/platform-delivery", courseHandler.GetPlatformDelivery)
			r.With(auth.RequireScope(auth.ScopeCoursesWrite)).Put("/courses/{id}/platform-delivery", courseHandler.PutPlatformDelivery)

			r.With(auth.RequireScope(auth.ScopeRegistriesRead)).Get("/registries/next-number", registryHandler.GetNextNumber)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireScope(auth.ScopeJournalsRead))
				r.Get("/journals", journalHandler.List)
				r.Get("/journals/{id}", journalHandler.Get)
				r.Get("/journals/{id}/pdf", journalHandler.PDF)
				r.Get("/journals/{id}/sessions", journalHandler.ListSessions)
				r.Get("/journals/{id}/attendees", journalHandler.ListAttendees)
				r.Get("/journals/{id}/attendance", journalHandler.ListAttendance)
				r.Get("/journals/{id}/attendance-scan/meta", journalHandler.GetJournalAttendanceScanMeta)
				r.Get("/journals/{id}/attendance-scan", journalHandler.GetJournalAttendanceScanFile)
				r.Get("/journals/{id}/signed-scan/meta", journalHandler.GetJournalSignedScanMeta)
				r.Get("/journals/{id}/signed-scan", journalHandler.GetJournalSignedScanFile)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireScope(auth.ScopeJournalsWrite))
				r.Post("/journals", journalHandler.Create)
				r.Delete("/journals/{id}", journalHandler.Delete)
				r.Patch("/journals/{id}", journalHandler.UpdateHeader)
				r.Post("/journals/{id}/close", journalHandler.Close)
				r.Post("/journals/{id}/sessions/generate-from-course", journalHandler.GenerateSessionsFromCourse)
				r.Patch("/journals/{id}/sessions/{sessionId}", journalHandler.PatchSession)
				r.Post("/journals/{id}/attendees", journalHandler.AddJournalAttendee)
				r.Post("/journals/{id}/attendees/{attendeeId}/certificate/generate", journalHandler.GenerateAttendeeCertificate)
				r.Patch("/journals/{id}/attendees/{attendeeId}/certificate", journalHandler.PatchAttendeeCertificate)
				r.Delete("/journals/{id}/attendees/{attendeeId}", journalHandler.DeleteAttendee)
				r.Patch("/journals/{id}/attendance", journalHandler.PatchAttendance)
				r.Post("/journals/{id}/attendance-scan", journalHandler.UpsertJournalAttendanceScan)
				r.Delete("/journals/{id}/attendance-scan", journalHandler.DeleteJournalAttendanceScanFile)
				r.Post("/journals/{id}/signed-scan", journalHandler.UpsertJournalSignedScan)
				r.Delete("/journals/{id}/signed-scan", journalHandler.DeleteJournalSignedScanFile)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin())
				r.With(auth.RequireScope(auth.ScopeCertificatesWrite)).Delete("/certificates/{id}", certificateHandler.SoftDeleteCertificate)
				r.With(auth.RequireScope(auth.ScopeUsersWrite)).Post("/admin/users", userHandler.CreateUser)
				r.With(auth.RequireScope(auth.ScopeUsersRead)).Get("/admin/users", userHandler.List)
				r.With(auth.RequireScope(auth.ScopeUsersWrite)).Delete("/admin/users/{id}", userHandler.Delete)
				r.With(auth.RequireScope(auth.ScopeUsersWrite)).Patch("/admin/users/{id}", userHandler.Patch)
				r.With(auth.RequireScope(auth.ScopeUsersWrite)).Patch("/admin/users/{id}/password", userHandler.PatchPasswordByAdmin)

				// Wystawianie kluczy kluczem API byłoby ścieżką eskalacji
				// uprawnień - klucz z users:write mógłby wygenerować sobie
				// kolejny, szerszy. Zarządzanie kluczami zostaje w przeglądarce.
				r.With(auth.RequireSession()).Get("/admin/api-keys", apiKeyHandler.List)
				r.With(auth.RequireSession()).Get("/admin/api-keys/scopes", apiKeyHandler.ListScopes)
				r.With(auth.RequireSession()).Post("/admin/api-keys", apiKeyHandler.Create)
				r.With(auth.RequireSession()).Delete("/admin/api-keys/{id}", apiKeyHandler.Revoke)

				// Pieczątka i podpis decydują o wyglądzie każdego dokumentu wychodzącego
				// z instytucji, więc podmiana zostaje w przeglądarce - klucz integracji
				// z certificates:write nie powinien móc podłożyć własnego podpisu.
				r.With(auth.RequireSession()).Post("/admin/certificate-print-assets/{kind}", certAssetHandler.Upsert)
				r.With(auth.RequireSession()).Delete("/admin/certificate-print-assets/{kind}", certAssetHandler.Delete)

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireScope(auth.ScopeAuditLogRead))
					r.Get("/courses/{id}/audit-log", auditLogHandler.ListByEntity("course"))
					r.Get("/certificates/{id}/audit-log", auditLogHandler.ListByEntity("certificate"))
					r.Get("/companies/{id}/audit-log", auditLogHandler.ListByEntity("company"))
					r.Get("/students/{id}/audit-log", auditLogHandler.ListByEntity("student"))
					r.Get("/admin/users/{id}/audit-log", auditLogHandler.ListByEntity("user"))
				})
			})
		})
	})

	return r
}

func (h Handler) healthzHandler(w http.ResponseWriter, r *http.Request) {
	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
