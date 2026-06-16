package group

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/modules/iam"
	"matrix/internal/platform/db/models"
)

type GroupDTO struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	ParentID   *uuid.UUID `json:"parent_id,omitempty"`
	Visibility string     `json:"visibility"`
	OwnerID    uuid.UUID  `json:"owner_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type MemberDTO struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      iam.Role  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateInput struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Visibility string `json:"visibility"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]GroupDTO, error) {
	var rows []models.Group
	q := s.db.WithContext(ctx).Model(&models.Group{})
	if !isAdmin {
		q = q.Where(
			"owner_id = ? OR id IN (?)",
			userID,
			s.db.Model(&models.GroupMember{}).Select("group_id").Where("user_id = ?", userID),
		)
	}
	if err := q.Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]GroupDTO, len(rows))
	for i := range rows {
		out[i] = toDTO(&rows[i])
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*GroupDTO, error) {
	var m models.Group
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return new(toDTO(&m)), nil
}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (*GroupDTO, error) {
	path := strings.TrimSpace(in.Path)
	if path == "" {
		path = slugify(in.Name)
	}
	if path == "" {
		return nil, errors.New("组路径不能为空")
	}
	vis := in.Visibility
	if vis == "" {
		vis = "private"
	}
	m := models.Group{Name: in.Name, Path: path, Visibility: vis, OwnerID: ownerID}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	_ = s.db.WithContext(ctx).Create(&models.GroupMember{
		GroupID: m.ID, UserID: ownerID, Role: string(iam.RoleOwner),
	}).Error
	return new(toDTO(&m)), nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, name, visibility *string) (*GroupDTO, error) {
	var m models.Group
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if name != nil {
		m.Name = *name
	}
	if visibility != nil && *visibility != "" {
		m.Visibility = *visibility
	}
	if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
		return nil, err
	}
	return new(toDTO(&m)), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Group{}, "id = ?", id).Error
	})
}

func (s *Service) ListMembers(ctx context.Context, groupID uuid.UUID) ([]MemberDTO, error) {
	var rows []models.GroupMember
	if err := s.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MemberDTO, 0, len(rows))
	for _, r := range rows {
		var u models.User
		if err := s.db.WithContext(ctx).First(&u, "id = ?", r.UserID).Error; err != nil {
			continue
		}
		out = append(out, MemberDTO{
			UserID: r.UserID, Username: u.Username, Email: u.Email,
			Name: u.Name, Role: iam.Role(r.Role), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) AddMember(ctx context.Context, groupID, userID uuid.UUID, role iam.Role) error {
	m := models.GroupMember{GroupID: groupID, UserID: userID, Role: string(role)}
	return s.db.WithContext(ctx).Save(&m).Error
}

func (s *Service) UpdateMember(ctx context.Context, groupID, userID uuid.UUID, role iam.Role) error {
	return s.db.WithContext(ctx).Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Update("role", string(role)).Error
}

func (s *Service) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&models.GroupMember{}).Error
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
				b.WriteByte('-')
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func toDTO(m *models.Group) GroupDTO {
	return GroupDTO{
		ID: m.ID, Name: m.Name, Path: m.Path, ParentID: m.ParentID,
		Visibility: m.Visibility, OwnerID: m.OwnerID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
