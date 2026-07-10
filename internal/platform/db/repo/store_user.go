package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// UserStore 封装用户与 Session 持久化。
type UserStore struct {
	c *catalog
}

func newUserStore(c *catalog) *UserStore { return &UserStore{c: c} }

func (s *UserStore) Create(ctx context.Context, m *models.User) error {
	return s.c.user.Create(ctx, m)
}

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.c.user.GetByID(ctx, id)
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.c.user.GetByUsername(ctx, username)
}

func (s *UserStore) GetByLogin(ctx context.Context, login string) (*models.User, error) {
	return s.c.user.GetByLogin(ctx, login)
}

func (s *UserStore) Search(ctx context.Context, q string, limit int) ([]models.User, error) {
	return s.c.user.Search(ctx, q, limit)
}

func (s *UserStore) List(ctx context.Context, limit, offset int) ([]models.User, int64, error) {
	return s.c.user.List(ctx, limit, offset)
}

func (s *UserStore) Save(ctx context.Context, m *models.User) error {
	return s.c.user.Save(ctx, m)
}

func (s *UserStore) Delete(ctx context.Context, id uuid.UUID) error {
	return s.c.user.Delete(ctx, id)
}

func (s *UserStore) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return s.c.user.UpdatePasswordHash(ctx, id, hash)
}

func (s *UserStore) UpdateState(ctx context.Context, id uuid.UUID, state string) error {
	return s.c.user.UpdateState(ctx, id, state)
}

func (s *UserStore) UpdateLastSignIn(ctx context.Context, id uuid.UUID, t time.Time) error {
	return s.c.user.UpdateLastSignIn(ctx, id, t)
}

func (s *UserStore) Count(ctx context.Context) (int64, error) {
	return s.c.user.Count(ctx)
}

func (s *UserStore) CountProjectMemberships(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.c.user.CountProjectMemberships(ctx, userID)
}

func (s *UserStore) CreateSession(ctx context.Context, m *models.Session) error {
	return s.c.session.Create(ctx, m)
}

func (s *UserStore) GetValidSession(ctx context.Context, tokenHash string, now time.Time) (*models.Session, error) {
	return s.c.session.GetValidByTokenHash(ctx, tokenHash, now)
}

func (s *UserStore) RevokeSession(ctx context.Context, tokenHash string) error {
	return s.c.session.DeleteByTokenHash(ctx, tokenHash)
}
