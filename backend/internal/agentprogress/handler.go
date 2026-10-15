// Package agentprogress exposes a user-scoped, bearer-only learning API.
package agentprogress

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Credentials struct {
	Enabled   bool
	UserID    uuid.UUID
	TokenHash [32]byte
	ExpiresAt time.Time
}

func LoadCredentials(getenv func(string) string) (Credentials, error) {
	hash, user, expires := getenv("AGENT_API_TOKEN_SHA256"), getenv("AGENT_USER_ID"), getenv("AGENT_TOKEN_EXPIRES_AT")
	if hash == "" && user == "" && expires == "" {
		return Credentials{}, nil
	}
	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != 32 {
		return Credentials{}, errors.New("AGENT_API_TOKEN_SHA256 must be a SHA-256 hex digest")
	}
	id, err := uuid.Parse(user)
	if err != nil || id == uuid.Nil {
		return Credentials{}, errors.New("AGENT_USER_ID must be a nonzero UUID")
	}
	expiry, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return Credentials{}, errors.New("AGENT_TOKEN_EXPIRES_AT must be RFC3339")
	}
	c := Credentials{Enabled: true, UserID: id, ExpiresAt: expiry}
	copy(c.TokenHash[:], decoded)
	return c, nil
}

type ChecklistItem struct {
	Key       string `json:"key"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}
type Topic struct {
	Slug           string          `json:"slug"`
	Title          string          `json:"title"`
	Status         string          `json:"status"`
	ReviewRequired bool            `json:"review_required"`
	Checklist      []ChecklistItem `json:"checklist"`
}
type Update struct {
	Status         string           `json:"status,omitempty"`
	ReviewRequired *bool            `json:"review_required,omitempty"`
	Checklist      map[string]*bool `json:"checklist,omitempty"`
	Summary        *string          `json:"summary,omitempty"`
}
type Result struct {
	Slug           string     `json:"slug"`
	UpdateID       string     `json:"update_id"`
	Status         string     `json:"status"`
	ReviewRequired bool       `json:"review_required"`
	NoteID         *uuid.UUID `json:"note_id,omitempty"`
}
type Store interface {
	Topics(context.Context, uuid.UUID) ([]Topic, error)
	Apply(context.Context, uuid.UUID, string, string, Update) (Result, error)
}

var (
	ErrNotFound     = errors.New("topic or checklist item not found")
	ErrConflict     = errors.New("update ID already used with different content")
	ErrUserNotFound = errors.New("credential owner not found")
	identifier      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
)

func Mount(router chi.Router, store Store, credentials Credentials) {
	router.Route("/agent", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				parts := strings.Fields(req.Header.Get("Authorization"))
				if !credentials.Enabled || !time.Now().Before(credentials.ExpiresAt) || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 512 || len(req.Header.Values("Authorization")) != 1 {
					writeError(w, req, 401, "unauthorized")
					return
				}
				hash := sha256.Sum256([]byte(parts[1]))
				if subtle.ConstantTimeCompare(hash[:], credentials.TokenHash[:]) != 1 {
					writeError(w, req, 401, "unauthorized")
					return
				}
				ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
				defer cancel()
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/topics", func(w http.ResponseWriter, req *http.Request) {
			topics, err := store.Topics(req.Context(), credentials.UserID)
			if err != nil {
				storeError(w, req, err)
				return
			}
			writeJSON(w, 200, map[string]any{"data": topics, "user_id": credentials.UserID})
		})
		r.Put("/topics/{slug}/updates/{update_id}", func(w http.ResponseWriter, req *http.Request) {
			slug, key := chi.URLParam(req, "slug"), chi.URLParam(req, "update_id")
			if !identifier.MatchString(slug) || !identifier.MatchString(key) {
				writeError(w, req, 400, "validation_error")
				return
			}
			input, err := decodeUpdate(w, req)
			if err != nil {
				writeError(w, req, 400, "validation_error")
				return
			}
			result, err := store.Apply(req.Context(), credentials.UserID, slug, key, input)
			if err != nil {
				storeError(w, req, err)
				return
			}
			writeJSON(w, 200, map[string]any{"data": result})
		})
	})
}

func decodeUpdate(w http.ResponseWriter, r *http.Request) (Update, error) {
	var input Update
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return input, errors.New("JSON required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&input); err != nil {
		return input, err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return input, errors.New("trailing JSON")
	}
	if input.Status != "" && input.Status != "learning" && input.Status != "review" && input.Status != "mastered" {
		return input, errors.New("invalid status")
	}
	if input.Status == "" && input.ReviewRequired == nil && len(input.Checklist) == 0 && input.Summary == nil {
		return input, errors.New("empty update")
	}
	if len(input.Checklist) > 100 {
		return input, errors.New("too many checklist items")
	}
	for key, value := range input.Checklist {
		if !identifier.MatchString(key) || value == nil {
			return input, errors.New("invalid checklist item")
		}
	}
	if input.Summary != nil && (strings.TrimSpace(*input.Summary) == "" || utf8.RuneCountInString(*input.Summary) > 20000) {
		return input, errors.New("invalid summary")
	}
	return input, nil
}
func storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, r, 404, "not_found")
	case errors.Is(err, ErrConflict):
		writeError(w, r, 409, "update_conflict")
	case errors.Is(err, ErrUserNotFound):
		writeError(w, r, 401, "unauthorized")
	default:
		writeError(w, r, 500, "internal_error")
	}
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code string) {
	if status == 401 {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": http.StatusText(status), "request_id": requestcontext.RequestID(r.Context())}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
