package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestProductionStartupRejectsMemoryBeforeServing(t *testing.T) {
	if os.Getenv("DEGOSALON_STARTUP_TEST") == "1" {
		main()
		return
	}
	t.Setenv("DEGOSALON_STARTUP_TEST", "1")
	t.Setenv("APP_ENV", "production")
	t.Setenv("RENDER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProductionStartupRejectsMemoryBeforeServing$")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "DATABASE_URL is required in production") || strings.Contains(string(output), "server listening") {
		t.Fatalf("unsafe production startup: %v %s", err, output)
	}
}
