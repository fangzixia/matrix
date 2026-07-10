package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"
	"time"

	"github.com/google/uuid"
)

// SessionService 管理 Session Cookie 的创建与校验。
type SessionService struct {
	users *repo.UserStore
	cfg   config.SessionConfig
}

// NewSessionService 创建 SessionService 并填充默认 Cookie 名与 TTL。
func NewSessionService(stores *repo.Stores, cfg config.SessionConfig) *SessionService {
	if cfg.CookieName == "" {
		cfg.CookieName = "_matrix_session"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 720 * time.Hour
	}
	return &SessionService{users: stores.User, cfg: cfg}
}

// CookieName 返回 Session Cookie 名称。
func (s *SessionService) CookieName() string { return s.cfg.CookieName }

// Secure 返回 Cookie 是否启用 Secure 标志。
func (s *SessionService) Secure() bool { return s.cfg.Secure }

// TTL 返回 Session 有效期。
func (s *SessionService) TTL() time.Duration { return s.cfg.TTL }

// Create 创建记录。
func (s *SessionService) Create(ctx context.Context, userID uuid.UUID, ip, ua string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	sess := models.Session{
		UserID:    userID,
		TokenHash: hash,
		IP:        ip,
		UserAgent: ua,
		ExpiresAt: time.Now().Add(s.cfg.TTL),
	}
	if err := s.users.CreateSession(ctx, &sess); err != nil {
		return "", err
	}
	return token, nil
}

// Validate 校验 Session 令牌并返回用户 ID。
func (s *SessionService) Validate(ctx context.Context, token string) (*User, error) {
	hash := hashToken(token)
	sess, err := s.users.GetValidSession(ctx, hash, time.Now())
	if err != nil {
		return nil, errors.New("invalid session")
	}
	m, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, errors.New("invalid session")
	}
	return toUser(m), nil
}

// Revoke 吊销指定 Session。
func (s *SessionService) Revoke(ctx context.Context, token string) error {
	hash := hashToken(token)
	return s.users.RevokeSession(ctx, hash)
}

// hashToken 对 Session token 做单向哈希。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
