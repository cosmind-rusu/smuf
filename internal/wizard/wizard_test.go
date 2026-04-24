package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSecureToken(t *testing.T) {
	token, err := generateSecureToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("expected token length 64, got %d", len(token))
	}
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("token contains invalid character: %c", c)
		}
	}
}

func TestLoadEnvFile(t *testing.T) {
	// Crear un .env temporal
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `SMUF_SERVER=test.com:7000
SMUF_AUTH_TOKEN=abc123
# comentario
SMUF_DOMAIN=example.com
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Cambiar al directorio temporal
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Limpiar variables antes de cargar
	os.Unsetenv("SMUF_SERVER")
	os.Unsetenv("SMUF_AUTH_TOKEN")
	os.Unsetenv("SMUF_DOMAIN")

	LoadEnvFile()

	if os.Getenv("SMUF_SERVER") != "test.com:7000" {
		t.Errorf("expected SMUF_SERVER=test.com:7000, got %s", os.Getenv("SMUF_SERVER"))
	}
	if os.Getenv("SMUF_AUTH_TOKEN") != "abc123" {
		t.Errorf("expected SMUF_AUTH_TOKEN=abc123, got %s", os.Getenv("SMUF_AUTH_TOKEN"))
	}
	if os.Getenv("SMUF_DOMAIN") != "example.com" {
		t.Errorf("expected SMUF_DOMAIN=example.com, got %s", os.Getenv("SMUF_DOMAIN"))
	}
}

func TestHumanServerErrorMapping(t *testing.T) {
	// Esta función está en cmd/smuf/main.go, no en wizard.
	// Dejamos placeholder para recordar que los tests de integración
	// del cliente deberían ir en su propio paquete.
}

func TestConfigPath(t *testing.T) {
	p := configPath()
	if !strings.Contains(p, "smuf") {
		t.Errorf("expected config path to contain 'smuf', got %s", p)
	}
	if !strings.HasSuffix(p, "config.json") {
		t.Errorf("expected config path to end with config.json, got %s", p)
	}
}
