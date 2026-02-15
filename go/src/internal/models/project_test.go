package models

import (
	"testing"

	"github.com/circle-oo/flux/internal/testutil"
)

func TestProjectStore_CreateAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewProjectStore(db)

	project := &Project{
		Name:      "flux",
		Type:      ProjectTypeRepo,
		RepoURL:   "https://github.com/circle-oo/flux",
		TechStack: []string{"go", "react"},
	}

	if err := store.Create(project); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if project.ID == "" {
		t.Error("expected ID to be set")
	}
	if project.Status != ProjectProposed {
		t.Errorf("expected PROPOSED, got %s", project.Status)
	}

	got, err := store.GetByID(project.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "flux" {
		t.Errorf("expected flux, got %s", got.Name)
	}
	if len(got.TechStack) != 2 {
		t.Errorf("expected 2 tech stack, got %d", len(got.TechStack))
	}
}

func TestProjectStore_ApproveAndReject(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewProjectStore(db)

	p1 := &Project{Name: "approve-me", Type: ProjectTypeRepo}
	p2 := &Project{Name: "reject-me", Type: ProjectTypeRepo}
	store.Create(p1)
	store.Create(p2)

	if err := store.Approve(p1.ID); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	got, _ := store.GetByID(p1.ID)
	if got.Status != ProjectActive {
		t.Errorf("expected ACTIVE, got %s", got.Status)
	}

	if err := store.Reject(p2.ID); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	got, _ = store.GetByID(p2.ID)
	if got.Status != ProjectRejected {
		t.Errorf("expected REJECTED, got %s", got.Status)
	}
}

func TestProjectStore_Approve_NotProposed(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewProjectStore(db)

	p := &Project{Name: "already-active", Type: ProjectTypeRepo}
	store.Create(p)
	store.Approve(p.ID) // Now ACTIVE

	// Try to approve again
	err := store.Approve(p.ID)
	if err == nil {
		t.Error("expected error approving non-PROPOSED project")
	}
}

func TestProjectStore_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewProjectStore(db)

	store.Create(&Project{Name: "a", Type: ProjectTypeRepo})
	store.Create(&Project{Name: "b", Type: ProjectTypeRepo})

	projects, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2, got %d", len(projects))
	}

	// Filter by status
	store.Approve(projects[0].ID)
	active, _ := store.List(ProjectActive)
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestProjectStore_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewProjectStore(db)

	p := &Project{Name: "delete-me", Type: ProjectTypeRepo}
	store.Create(p)

	if err := store.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := store.GetByID(p.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}
