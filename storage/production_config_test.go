package storage

import "testing"

func TestProductionDatabaseCannotFallBackToMemory(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("RENDER", "")
	t.Setenv("DATABASE_URL", "")
	if err := InitDatabase(); err == nil {
		t.Fatal("production used memory storage")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("RENDER", "true")
	if err := InitDatabase(); err == nil {
		t.Fatal("Render used memory storage")
	}
	t.Setenv("RENDER", "")
	if err := InitDatabase(); err != nil {
		t.Fatalf("development memory: %v", err)
	}
}
