package handlers

import "testing"

func TestProductionDisallowsGeneratedAdminAndResetLinks(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("RENDER", "")
	t.Setenv("ADMIN_USERNAME", "operator")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("SHOW_RESET_LINKS", "1")
	if generated, err := InitAdmin(); err == nil || generated != "" {
		t.Fatal("production generated administrator credentials")
	}
	if showResetLinks() {
		t.Fatal("production exposed reset links")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("RENDER", "true")
	if generated, err := InitAdmin(); err == nil || generated != "" {
		t.Fatal("Render generated administrator credentials")
	}
	if showResetLinks() {
		t.Fatal("Render exposed reset links")
	}
}
