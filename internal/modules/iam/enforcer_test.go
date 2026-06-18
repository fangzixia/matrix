package iam

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/db/models"
)

func setupEnforcerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedUser(t *testing.T, db *gorm.DB, username string) uuid.UUID {
	t.Helper()
	u := models.User{Username: username, Email: username + "@test.local", PasswordHash: "x"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func TestEffectiveRole_GroupInheritance(t *testing.T) {
	db := setupEnforcerTestDB(t)
	ctx := context.Background()
	e := NewEnforcer(db)

	ownerID := seedUser(t, db, "owner")
	memberID := seedUser(t, db, "member")

	g := models.Group{Name: "team", OwnerID: ownerID}
	if err := db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GroupMember{
		GroupID: g.ID, UserID: memberID, Role: string(RoleMaintainer),
	}).Error; err != nil {
		t.Fatal(err)
	}

	p := models.Project{Name: "proj", OwnerID: ownerID, GroupID: &g.ID, Visibility: "private"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ProjectMember{
		ProjectID: p.ID, UserID: memberID, Role: string(RoleDeveloper),
	}).Error; err != nil {
		t.Fatal(err)
	}

	role, ok, err := e.EffectiveRole(ctx, memberID, p.ID)
	if err != nil || !ok {
		t.Fatalf("EffectiveRole: ok=%v err=%v", ok, err)
	}
	if role != RoleMaintainer {
		t.Fatalf("want maintainer, got %s", role)
	}
}

func TestCanAccess_GroupMemberPrivateProject(t *testing.T) {
	db := setupEnforcerTestDB(t)
	ctx := context.Background()
	e := NewEnforcer(db)

	ownerID := seedUser(t, db, "owner2")
	devID := seedUser(t, db, "dev")

	g := models.Group{Name: "g2", OwnerID: ownerID}
	if err := db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GroupMember{
		GroupID: g.ID, UserID: devID, Role: string(RoleDeveloper),
	}).Error; err != nil {
		t.Fatal(err)
	}

	p := models.Project{Name: "private-proj", OwnerID: ownerID, GroupID: &g.ID, Visibility: "private"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}

	ok, err := e.CanAccess(ctx, devID, p.ID, RoleDeveloper, false)
	if err != nil || !ok {
		t.Fatalf("CanAccess developer: ok=%v err=%v", ok, err)
	}
}

func TestCanAccess_InternalProjectGuest(t *testing.T) {
	db := setupEnforcerTestDB(t)
	ctx := context.Background()
	e := NewEnforcer(db)

	ownerID := seedUser(t, db, "owner3")
	outsiderID := seedUser(t, db, "outsider")

	p := models.Project{Name: "internal-proj", OwnerID: ownerID, Visibility: "internal"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}

	ok, err := e.CanAccess(ctx, outsiderID, p.ID, RoleGuest, false)
	if err != nil || !ok {
		t.Fatalf("guest read internal: ok=%v err=%v", ok, err)
	}

	ok, err = e.CanAccess(ctx, outsiderID, p.ID, RoleDeveloper, false)
	if err != nil || ok {
		t.Fatalf("outsider developer on internal: ok=%v err=%v", ok, err)
	}
}

func TestValidateRole(t *testing.T) {
	if err := ValidateRole(RoleDeveloper); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRole(Role("superuser")); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestPermissionsForGroupRole(t *testing.T) {
	guest := PermissionsForGroupRole(RoleGuest)
	if !guest.Read || guest.ManageMembers || guest.DeleteGroup {
		t.Fatal("guest permissions mismatch")
	}
	owner := PermissionsForGroupRole(RoleOwner)
	if !owner.DeleteGroup || !owner.ManageMembers {
		t.Fatal("owner permissions mismatch")
	}
}
