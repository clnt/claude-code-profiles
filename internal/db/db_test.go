package db

import (
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpen_CreatesTables(t *testing.T) {
	d := testDB(t)

	// Verify schema_version is set
	v, err := d.GetState("schema_version")
	if err != nil {
		t.Fatal(err)
	}
	if v != "1" {
		t.Errorf("schema_version = %q, want %q", v, "1")
	}
}

func TestCreateProfile(t *testing.T) {
	d := testDB(t)

	if err := d.CreateProfile("work", "Work config", false); err != nil {
		t.Fatal(err)
	}

	p, err := d.GetProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "work" {
		t.Errorf("Name = %q, want %q", p.Name, "work")
	}
	if p.Description != "Work config" {
		t.Errorf("Description = %q, want %q", p.Description, "Work config")
	}
	if p.IsDefault {
		t.Error("should not be default")
	}
}

func TestCreateProfile_Duplicate(t *testing.T) {
	d := testDB(t)

	d.CreateProfile("work", "", false)
	err := d.CreateProfile("work", "", false)
	if err == nil {
		t.Error("expected error for duplicate profile")
	}
}

func TestSetDefault(t *testing.T) {
	d := testDB(t)

	d.CreateProfile("work", "", false)
	d.CreateProfile("personal", "", false)

	if err := d.SetDefault("work"); err != nil {
		t.Fatal(err)
	}

	p, _ := d.GetDefault()
	if p.Name != "work" {
		t.Errorf("default = %q, want %q", p.Name, "work")
	}

	// Change default
	if err := d.SetDefault("personal"); err != nil {
		t.Fatal(err)
	}

	p, _ = d.GetDefault()
	if p.Name != "personal" {
		t.Errorf("default = %q, want %q", p.Name, "personal")
	}

	// Verify old default is cleared
	work, _ := d.GetProfile("work")
	if work.IsDefault {
		t.Error("work should no longer be default")
	}
}

func TestSetDefault_NonExistent(t *testing.T) {
	d := testDB(t)

	err := d.SetDefault("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestListProfiles(t *testing.T) {
	d := testDB(t)

	d.CreateProfile("beta", "", false)
	d.CreateProfile("alpha", "", false)

	profiles, err := d.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	if profiles[0].Name != "alpha" {
		t.Errorf("first profile = %q, want %q (should be sorted)", profiles[0].Name, "alpha")
	}
}

func TestDeleteProfile(t *testing.T) {
	d := testDB(t)

	d.CreateProfile("work", "", false)
	if err := d.DeleteProfile("work"); err != nil {
		t.Fatal(err)
	}

	exists, _ := d.ProfileExists("work")
	if exists {
		t.Error("profile should be deleted")
	}
}

func TestDeleteProfile_NonExistent(t *testing.T) {
	d := testDB(t)

	err := d.DeleteProfile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestActiveProfile(t *testing.T) {
	d := testDB(t)

	// Initially no active profile
	active, err := d.GetActiveProfile()
	if err != nil {
		t.Fatal(err)
	}
	if active != "" {
		t.Errorf("active = %q, want empty", active)
	}

	// Set active
	d.SetActiveProfile("work")
	active, _ = d.GetActiveProfile()
	if active != "work" {
		t.Errorf("active = %q, want %q", active, "work")
	}

	// Clear active
	d.ClearActiveProfile()
	active, _ = d.GetActiveProfile()
	if active != "" {
		t.Errorf("active = %q, want empty", active)
	}
}

func TestProfileCount(t *testing.T) {
	d := testDB(t)

	count, _ := d.ProfileCount()
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	d.CreateProfile("a", "", false)
	d.CreateProfile("b", "", false)

	count, _ = d.ProfileCount()
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
