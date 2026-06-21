package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/config"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash is compared against the supplied password when no user is
// found, so the login response time does not reveal whether an account exists.
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalization-placeholder"), bcrypt.DefaultCost)

type Handler struct {
	Queries *dbsql.Queries
	Config  *config.Config
}

func NewHandler(queries *dbsql.Queries, cfg *config.Config) *Handler {
	return &Handler{
		Queries: queries,
		Config:  cfg,
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	loginRequest := LoginRequest{}
	err := decoder.Decode(&loginRequest)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "failed to decode login request")
		return
	}
	if loginRequest.Email == "" || loginRequest.Password == "" {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "email and password are required")
		return
	}
	user, err := h.Queries.GetUserByEmail(r.Context(), loginRequest.Email)
	if err != nil {
		// Equalize timing with the success path so a missing account cannot be
		// distinguished from a wrong password by response latency.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(loginRequest.Password))
		response.WriteError(w, http.StatusUnauthorized, response.CodeInvalidCredentials, "invalid credentials")
		return
	}

	err = bcrypt.CompareHashAndPassword(user.Password, []byte(loginRequest.Password))
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, response.CodeInvalidCredentials, "invalid credentials")
		return
	}

	token, err := newSessionToken()
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to generate token")
		return
	}
	// Store only the hash of the token; the raw token lives solely in the client
	// cookie, so a database leak does not expose usable session tokens.
	_, err = h.Queries.CreateSession(r.Context(), dbsql.CreateSessionParams{Token: hashToken(token), UserID: user.ID, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(h.Config.SessionTTL), Valid: true}})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to create session")
		return
	}
	setSessionCookie(w, token, h.Config)

	resp := LoginResponse{
		Data: UserDTO{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.Firstname,
			LastName:  user.Lastname,
			Role:      user.Role,
		},
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(h.Config.SessionCookieName)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unable to get session token")
		return
	}

	token := c.Value
	err = h.Queries.DeleteSessionByToken(r.Context(), hashToken(token))
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "unable to delete session")
		return
	}
	clearSessionCookie(w, h.Config)

	response.WriteNoContent(w)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unable to get user from context")
		return
	}
	resp := MeResponse{
		Data: UserDTO{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.Firstname,
			LastName:  user.Lastname,
			Role:      user.Role,
		},
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken derives the value stored in the database from the raw session token
// held by the client. SHA-256 is sufficient here: the token already has 256
// bits of entropy, so it is not subject to brute force.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func setSessionCookie(w http.ResponseWriter, token string, config *config.Config) {
	cookie := http.Cookie{
		Name:     config.SessionCookieName,
		Value:    token,
		Expires:  time.Now().Add(config.SessionTTL),
		HttpOnly: true,
		Secure:   config.SessionCookieSecure,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &cookie)
}

func clearSessionCookie(w http.ResponseWriter, config *config.Config) {
	cookie := http.Cookie{
		Name:     config.SessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		MaxAge:   -1,
		Secure:   config.SessionCookieSecure,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &cookie)
}

func userFromContext(ctx context.Context) (dbsql.User, bool) {
	user, ok := ctx.Value(userContextKey).(dbsql.User)
	return user, ok
}

func UserFromContext(ctx context.Context) (dbsql.User, bool) {
	return userFromContext(ctx)
}

func ContextWithUser(ctx context.Context, user dbsql.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
