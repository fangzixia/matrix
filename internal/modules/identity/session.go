package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
)

type SessionService struct {
	db  *gorm.DB
	cfg config.SessionConfig
}

func NewSessionService(db *gorm.DB, cfg config.SessionConfig) *SessionService {
	if cfg.CookieName == "" {
		cfg.CookieName = "_matrix_session"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 720 * time.Hour
	}
	return &SessionService{db: db, cfg: cfg}
}

func (s *SessionService) CookieName() string { return s.cfg.CookieName }
func (s *SessionService) Secure() bool       { return s.cfg.Secure }
func (s *SessionService) TTL() time.Duration { return s.cfg.TTL }

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
	if err := s.db.WithContext(ctx).Create(&sess).Error; err != nil {
		return "", err
	}
	return token, nil
}

func (s *SessionService) Validate(ctx context.Context, token string) (*User, error) {
	hash := hashToken(token)
	var sess models.Session
	if err := s.db.WithContext(ctx).Where("token_hash = ? AND expires_at > ?", hash, time.Now()).First(&sess).Error; err != nil {
		return nil, errors.New("invalid session")
	}
	repo := NewUserRepo(s.db)
	return repo.GetByID(ctx, sess.UserID)
}

func (s *SessionService) Revoke(ctx context.Context, token string) error {
	hash := hashToken(token)
	return s.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&models.Session{}).Error
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
