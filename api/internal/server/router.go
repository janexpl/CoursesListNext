// Package server
package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
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
	certificateService := certificates.NewService(deps.Pool, deps.Queries, recorder)
	userService := users.NewServiceWithAudit(deps.Pool, deps.Queries, recorder)
	userHandler := users.NewHandler(deps.Queries, userService)
	certificateHandler := certificates.NewHandler(deps.Queries, certificateService)
	dashboardHandler := dashboard.NewHandler(deps.Queries)
	coursesService := courses.NewService(deps.Pool, deps.Queries, recorder)
	courseHandler := courses.NewHandler(deps.Queries, coursesService)
	registryHandler := registries.NewHandler(deps.Queries)
	journalService := journals.NewService(deps.Pool, deps.Queries, recorder)
	journalHandler := journals.NewHandler(deps.Queries, journalService)
	auditLogHandler := auditlog.NewHandler(deps.Queries)
	gusclientHandler := gusclient.NewHandler(deps.Config)
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.Config.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	loginLimiter := newIPLimiter(deps.Config.LoginRateLimit/60, 10)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", h.healthzHandler)
		r.With(RateLimitByIP(loginLimiter)).Post("/auth/login", authHandler.Login)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(authHandler.Queries, authHandler.Config))
			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/auth/me", authHandler.Me)

			r.Get("/students", studentHandler.List)
			r.Get("/students/{id}", studentHandler.Get)
			r.Get("/students/{id}/certificates", studentHandler.ListCertificatesByStudent)
			r.Patch("/students/{id}", studentHandler.Patch)
			r.Post("/students", studentHandler.CreateStudent)

			r.Get("/companies", companyHandler.List)
			r.Get("/companies/{id}", companyHandler.Get)
			r.Get("/companies/{id}/students", studentHandler.ListStudentsByCompanyId)
			r.Patch("/companies/{id}", companyHandler.Patch)
			r.Post("/companies", companyHandler.CreateCompany)
			r.Get("/companies/lookup-by-nip", gusclientHandler.FindCompany)

			r.Get("/certificates", certificateHandler.List)
			r.Post("/certificates", certificateHandler.Create)
			r.Get("/certificates/{id}", certificateHandler.Get)
			r.Get("/certificates/{id}/pdf", certificateHandler.PDF)
			r.Patch("/certificates/{id}", certificateHandler.Patch)

			r.Get("/dashboard", dashboardHandler.Get)

			r.Get("/courses", courseHandler.List)
			r.Get("/courses/{id}", courseHandler.Get)
			r.Patch("/courses/{id}", courseHandler.Patch)
			r.Post("/courses", courseHandler.CreateCourse)
			r.Get("/courses/{id}/certificates", certificateHandler.ListByCourseID)

			r.Get("/registries/next-number", registryHandler.GetNextNumber)
			r.Patch("/account/profile", userHandler.PatchProfile)
			r.Patch("/account/password", userHandler.PatchPassword)

			r.Get("/journals", journalHandler.List)
			r.Get("/journals/{id}", journalHandler.Get)
			r.Get("/journals/{id}/pdf", journalHandler.PDF)
			r.Post("/journals", journalHandler.Create)
			r.Delete("/journals/{id}", journalHandler.Delete)
			r.Post("/journals/{id}/close", journalHandler.Close)
			r.Get("/journals/{id}/sessions", journalHandler.ListSessions)
			r.Post("/journals/{id}/sessions/generate-from-course", journalHandler.GenerateSessionsFromCourse)
			r.Patch("/journals/{id}/sessions/{sessionId}", journalHandler.PatchSession)
			r.Get("/journals/{id}/attendees", journalHandler.ListAttendees)
			r.Post("/journals/{id}/attendees", journalHandler.AddJournalAttendee)
			r.Post("/journals/{id}/attendees/{attendeeId}/certificate/generate", journalHandler.GenerateAttendeeCertificate)
			r.Patch("/journals/{id}/attendees/{attendeeId}/certificate", journalHandler.PatchAttendeeCertificate)
			r.Delete("/journals/{id}/attendees/{attendeeId}", journalHandler.DeleteAttendee)
			r.Get("/journals/{id}/attendance", journalHandler.ListAttendance)
			r.Patch("/journals/{id}/attendance", journalHandler.PatchAttendance)
			r.Post("/journals/{id}/attendance-scan", journalHandler.UpsertJournalAttendanceScan)
			r.Get("/journals/{id}/attendance-scan/meta", journalHandler.GetJournalAttendanceScanMeta)
			r.Get("/journals/{id}/attendance-scan", journalHandler.GetJournalAttendanceScanFile)
			r.Delete("/journals/{id}/attendance-scan", journalHandler.DeleteJournalAttendanceScanFile)
			r.Post("/journals/{id}/signed-scan", journalHandler.UpsertJournalSignedScan)
			r.Get("/journals/{id}/signed-scan/meta", journalHandler.GetJournalSignedScanMeta)
			r.Get("/journals/{id}/signed-scan", journalHandler.GetJournalSignedScanFile)
			r.Delete("/journals/{id}/signed-scan", journalHandler.DeleteJournalSignedScanFile)
			r.Patch("/journals/{id}", journalHandler.UpdateHeader)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin())
				r.Delete("/certificates/{id}", certificateHandler.SoftDeleteCertificate)
				r.Post("/admin/users", userHandler.CreateUser)
				r.Get("/admin/users", userHandler.List)
				r.Delete("/admin/users/{id}", userHandler.Delete)
				r.Patch("/admin/users/{id}", userHandler.Patch)
				r.Patch("/admin/users/{id}/password", userHandler.PatchPasswordByAdmin)
				r.Get("/courses/{id}/audit-log", auditLogHandler.ListByEntity("course"))
				r.Get("/certificates/{id}/audit-log", auditLogHandler.ListByEntity("certificate"))
				r.Get("/companies/{id}/audit-log", auditLogHandler.ListByEntity("company"))
				r.Get("/students/{id}/audit-log", auditLogHandler.ListByEntity("student"))
				r.Get("/admin/users/{id}/audit-log", auditLogHandler.ListByEntity("user"))
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
