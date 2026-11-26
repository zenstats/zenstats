package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/response"
)

type failedLoginService struct {
	t               *testing.T
	userExists      bool
	passwordChecked bool
}

func (s *failedLoginService) GetUserByEmail(_ context.Context, email string) (*ent.User, error) {
	if email != "user@example.com" {
		s.t.Fatalf("unexpected email: %q", email)
	}
	if !s.userExists {
		return nil, &ent.NotFoundError{}
	}
	return &ent.User{ID: 1, Email: email}, nil
}

func (s *failedLoginService) CheckPassword(_ context.Context, user *ent.User, password string) bool {
	if user.ID != 1 || password != "wrong-password" {
		s.t.Fatal("unexpected password check arguments")
	}
	s.passwordChecked = true
	return false
}

func (s *failedLoginService) GetUserStatus(context.Context, int64) string {
	s.t.Fatal("failed authentication must not reach the account status check")
	return ""
}

func TestLoginInvalidCredentialsResponsesMatch(t *testing.T) {
	for _, locale := range []string{"en", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			var responses []*httptest.ResponseRecorder
			for _, exists := range []bool{false, true} {
				svc := &failedLoginService{t: t, userExists: exists}
				router := gin.New()
				router.Use(func(c *gin.Context) { c.Set("locale", locale) })
				router.POST("/login", login(svc))
				request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"user@example.com","password":"wrong-password"}`))
				request.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				// The shared response helper uses HTTP 200 with an application code of 401.
				if recorder.Code != http.StatusOK {
					t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusOK)
				}
				var body response.ErrorResponse
				if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Code != http.StatusUnauthorized || body.Error == "" {
					t.Fatalf("unexpected error response: %+v", body)
				}
				if svc.passwordChecked != exists {
					t.Fatalf("password check reached = %t, want %t", svc.passwordChecked, exists)
				}
				responses = append(responses, recorder)
			}
			if responses[0].Code != responses[1].Code || responses[0].Body.String() != responses[1].Body.String() || responses[0].Header().Get("Content-Type") != responses[1].Header().Get("Content-Type") {
				t.Fatalf("login failures differ: missing user=%s, wrong password=%s", responses[0].Body, responses[1].Body)
			}
		})
	}
}
