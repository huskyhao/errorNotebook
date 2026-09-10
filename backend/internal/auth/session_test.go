package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erro-notebook/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type memoryStore struct {
	nextUser int64
	nextID   int64
	sessions map[string]*models.UserSession
}

func newMemoryStore() *memoryStore {
	return &memoryStore{nextUser: 10, nextID: 1, sessions: map[string]*models.UserSession{}}
}

func (s *memoryStore) FindSession(hash string, now time.Time) (*models.UserSession, error) {
	item := s.sessions[hash]
	if item == nil || !item.ExpiresAt.After(now) {
		return nil, nil
	}
	copy := *item
	return &copy, nil
}
func (s *memoryStore) CreateAnonymousSession(now time.Time, hash string, expires time.Time) (*models.UserSession, error) {
	s.nextUser++
	s.nextID++
	item := &models.UserSession{ID: s.nextID, UserID: s.nextUser, TokenHash: hash, ExpiresAt: expires, LastSeenAt: now}
	s.sessions[hash] = item
	return item, nil
}
func (s *memoryStore) TouchSession(id int64, now time.Time) error { return nil }

func TestAnonymousSessionFirstVisitReuseIsolationAndForgery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newMemoryStore()
	manager := NewManager(store, "test-secret", time.Hour, false)
	router := gin.New()
	router.Use(manager.Middleware())
	router.GET("/api/v1/session", func(c *gin.Context) { c.JSON(http.StatusOK, manager.SessionInfo(c)) })

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/session", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}
	cookie := first.Result().Cookies()[0]
	if cookie.Name != CookieName || !cookie.HttpOnly {
		t.Fatalf("cookie = %#v, want signed HttpOnly session", cookie)
	}
	if first.Header().Get("X-Erro-Anonymous-Notice") == "" {
		t.Fatal("first visit did not explain anonymous data scope")
	}
	var firstInfo map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstInfo); err != nil {
		t.Fatalf("decode first session: %v", err)
	}
	firstUserID := firstInfo["userId"].(float64)

	reused := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(reused, req)
	var reusedInfo map[string]any
	if err := json.Unmarshal(reused.Body.Bytes(), &reusedInfo); err != nil {
		t.Fatalf("decode reused session: %v", err)
	}
	if got := reusedInfo["userId"].(float64); got != firstUserID {
		t.Fatalf("same cookie changed identity: first=%v reused=%v", firstUserID, got)
	}

	other := httptest.NewRecorder()
	router.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/api/v1/session", nil))
	var otherInfo map[string]any
	if err := json.Unmarshal(other.Body.Bytes(), &otherInfo); err != nil {
		t.Fatalf("decode other session: %v", err)
	}
	if got := otherInfo["userId"].(float64); got == firstUserID {
		t.Fatal("different cookie/browser reused anonymous identity")
	}

	forged := *cookie
	forged.Value = cookie.Value + "tampered"
	forgedResponse := httptest.NewRecorder()
	forgedReq := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	forgedReq.AddCookie(&forged)
	router.ServeHTTP(forgedResponse, forgedReq)
	var forgedInfo map[string]any
	if err := json.Unmarshal(forgedResponse.Body.Bytes(), &forgedInfo); err != nil {
		t.Fatalf("decode forged session: %v", err)
	}
	if got := forgedInfo["userId"].(float64); got == firstUserID {
		t.Fatal("forged cookie retained original identity")
	}
}
