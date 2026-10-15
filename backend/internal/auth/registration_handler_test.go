package auth

import (
	"bytes"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegistrationHTTPContract(t *testing.T) {
	code, _ := GenerateSessionToken()
	for _, tc := range []struct {
		name      string
		storeErr  error
		status    int
		errorCode string
	}{
		{"created", nil, 201, ""}, {"used", ErrInvalidInvitation, 400, "invalid_invitation"}, {"taken login", ErrLoginUnavailable, 409, "login_unavailable"}, {"limited", ErrRegistrationRateLimited, 429, "rate_limited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &memoryRegistration{}
			if tc.storeErr == ErrLoginUnavailable {
				store.registerErr = tc.storeErr
			} else {
				store.attemptErr = tc.storeErr
			}
			h := NewHandler(NewService(&memorySessions{}, &recordingVerifier{}, time.Hour), true).WithRegistration(NewRegistrationService(store, &recordingVerifier{}, time.Hour))
			router := chi.NewRouter()
			h.Mount(router)
			body, _ := json.Marshal(map[string]string{"login": "new-user", "password": "long-password-123", "invitation_code": code.Plaintext})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("POST", "/auth/register", bytes.NewReader(body)))
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			for _, secret := range []string{code.Plaintext, "long-password-123", "new-encoded-hash"} {
				if strings.Contains(response.Body.String(), secret) {
					t.Fatal("response leaks secrets")
				}
			}
			if tc.status == 201 {
				cookies := response.Result().Cookies()
				if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
					t.Fatal("session cookie is missing or unsafe")
				}
			} else {
				if !strings.Contains(response.Body.String(), tc.errorCode) || len(response.Result().Cookies()) != 0 {
					t.Fatal("invalid error contract")
				}
			}
			if tc.status == 429 && response.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry information")
			}
		})
	}
}

func TestRegistrationRejectsUnknownOrTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"unexpected":true}`, `{} {}`, `null`, strings.Repeat("x", 5000)} {
		store := &memoryRegistration{}
		h := NewHandler(NewService(&memorySessions{}, &recordingVerifier{}, time.Hour), false).WithRegistration(NewRegistrationService(store, &recordingVerifier{}, time.Hour))
		router := chi.NewRouter()
		h.Mount(router)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/auth/register", strings.NewReader(body)))
		if response.Code != 400 || store.attempts != 0 {
			t.Fatalf("status=%d attempts=%d", response.Code, store.attempts)
		}
	}
}
