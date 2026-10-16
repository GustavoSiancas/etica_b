package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvBOMAndEnvironmentPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	t.Setenv("ETICA_TEST_BOM", "from-system")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbf# config\nETICA_TEST_BOM=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ETICA_TEST_BOM") != "from-system" {
		t.Fatal("overrode system environment")
	}
	if err := os.WriteFile(path, []byte("SECRET=\"private-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	err := loadEnvFile(path)
	if err == nil {
		t.Fatal("accepted invalid syntax")
	}
	if strings.Contains(err.Error(), "private-secret") {
		t.Fatal("leaked file contents")
	}
}

func TestLoadEscapesDatabaseCredentials(t *testing.T) {
	for key, value := range map[string]string{
		"PG_HOST": "localhost", "PG_PORT": "5432", "PG_DATABASE": "etica_cal", "PG_USER": "user@example", "PG_PASSWORD": "p@ss:/?# word", "PG_SSLMODE": "disable",
		"R2_ACCESS_KEY_ID": "key", "R2_SECRET_ACCESS_KEY": "secret", "R2_BUCKET": "documents", "CLOUDFLARE_ACCOUNT_ID": "account", "R2_ENDPOINT": "", "R2_REGION": "",
	} {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if password != "p@ss:/?# word" || u.User.Username() != "user@example" {
		t.Fatal("credentials did not round-trip")
	}
	if cfg.R2Endpoint != "https://account.r2.cloudflarestorage.com" || cfg.R2Region != "auto" {
		t.Fatal("incorrect R2 defaults")
	}
	t.Setenv("R2_ENDPOINT", "http://insecure.example")
	if _, err = Load(); err == nil {
		t.Fatal("accepted insecure R2 endpoint")
	}
	t.Setenv("PG_PASSWORD", "")
	if _, err = Load(); err == nil {
		t.Fatal("accepted missing database password")
	}
}
