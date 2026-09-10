package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"erro-notebook/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const CookieName = "erro_session"

type contextKey struct{}

var userIDKey contextKey

var ErrUnauthenticated = errors.New("anonymous session is required")

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(userIDKey).(int64)
	return value, ok && value > 0
}

type Store interface {
	FindSession(tokenHash string, now time.Time) (*models.UserSession, error)
	CreateAnonymousSession(now time.Time, tokenHash string, expiresAt time.Time) (*models.UserSession, error)
	TouchSession(id int64, now time.Time) error
}

type GormStore struct{ db *gorm.DB }

func NewGormStore(db *gorm.DB) *GormStore { return &GormStore{db: db} }

func (s *GormStore) FindSession(tokenHash string, now time.Time) (*models.UserSession, error) {
	var session models.UserSession
	err := s.db.Where("token_hash = ? AND expires_at > ?", tokenHash, now).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find session: %w", err)
	}
	return &session, nil
}

func (s *GormStore) CreateAnonymousSession(now time.Time, tokenHash string, expiresAt time.Time) (*models.UserSession, error) {
	var session models.UserSession
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user := models.User{Kind: "anonymous"}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		session = models.UserSession{UserID: user.ID, TokenHash: tokenHash, ExpiresAt: expiresAt, LastSeenAt: now}
		return tx.Create(&session).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create anonymous session: %w", err)
	}
	return &session, nil
}

func (s *GormStore) TouchSession(id int64, now time.Time) error {
	return s.db.Model(&models.UserSession{}).Where("id = ?", id).Update("last_seen_at", now).Error
}

type Manager struct {
	store     Store
	secret    []byte
	cookieTTL time.Duration
	secure    bool
}

func NewManager(store Store, secret string, cookieTTL time.Duration, secure bool) *Manager {
	if strings.TrimSpace(secret) == "" {
		secret = "change-me-in-production"
	}
	if cookieTTL <= 0 {
		cookieTTL = 30 * 24 * time.Hour
	}
	return &Manager{store: store, secret: []byte(secret), cookieTTL: cookieTTL, secure: secure}
}

func (m *Manager) Middleware() func(*gin.Context) {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || strings.HasSuffix(c.Request.URL.Path, "/health") {
			c.Next()
			return
		}
		now := time.Now()
		value, _ := c.Request.Cookie(CookieName)
		var session *models.UserSession
		if value != nil {
			if token, ok := m.verify(value.Value); ok {
				var findErr error
				session, findErr = m.store.FindSession(hashToken(token), now)
				if findErr != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SESSION_LOOKUP_FAILED", "message": "anonymous session unavailable"}})
					return
				}
			}
		}
		newSession := session == nil
		if newSession {
			token, err := randomToken()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SESSION_CREATE_FAILED", "message": "anonymous session unavailable"}})
				return
			}
			expiresAt := now.Add(m.cookieTTL)
			created, err := m.store.CreateAnonymousSession(now, hashToken(token), expiresAt)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SESSION_CREATE_FAILED", "message": "anonymous session unavailable"}})
				return
			}
			session = created
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(CookieName, m.sign(token), int(m.cookieTTL.Seconds()), "/", "", m.secure, true)
			c.Header("X-Erro-Anonymous-Notice", "data-is-bound-to-this-browser")
		} else {
			_ = m.store.TouchSession(session.ID, now)
		}
		c.Request = c.Request.WithContext(WithUserID(c.Request.Context(), session.UserID))
		c.Set("anonymousUserID", session.UserID)
		c.Next()
	}
}

func (m *Manager) SessionInfo(c *gin.Context) gin.H {
	return gin.H{"userId": c.MustGet("anonymousUserID"), "kind": "anonymous", "notice": "未注册数据仅绑定当前浏览器；清除 Cookie 或更换设备后无法恢复。"}
}

func (m *Manager) sign(token string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(token))
	return token + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *Manager) verify(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" {
		return "", false
	}
	want := m.sign(parts[0])
	if !hmac.Equal([]byte(value), []byte(want)) {
		return "", false
	}
	return parts[0], true
}

func randomToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", digest[:])
}
