package config

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr          string
	R2PublicURL       string
	DatabaseURL       string
	R2Endpoint        string
	R2Region          string
	R2Bucket          string
	R2AccessKeyID     string
	R2SecretAccessKey string
	EnableDemoReset   bool
}

// Load preserves environment variables supplied by the deployment platform.
func Load() (Config, error) {
	if err := loadEnvFile(".env"); err != nil {
		return Config{}, err
	}
	required := []string{"PG_HOST", "PG_PORT", "PG_DATABASE", "PG_USER", "PG_PASSWORD", "PG_SSLMODE", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_BUCKET"}
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return Config{}, fmt.Errorf("missing configuration: %s", key)
		}
	}
	db := &url.URL{Scheme: "postgres", Host: os.Getenv("PG_HOST") + ":" + os.Getenv("PG_PORT"), Path: "/" + os.Getenv("PG_DATABASE"), User: url.UserPassword(os.Getenv("PG_USER"), os.Getenv("PG_PASSWORD"))}
	q := db.Query()
	q.Set("sslmode", os.Getenv("PG_SSLMODE"))
	db.RawQuery = q.Encode()
	endpoint := os.Getenv("R2_ENDPOINT")
	if endpoint == "" {
		account := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
		if account == "" {
			return Config{}, fmt.Errorf("missing configuration: CLOUDFLARE_ACCOUNT_ID or R2_ENDPOINT")
		}
		endpoint = "https://" + account + ".r2.cloudflarestorage.com"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return Config{}, fmt.Errorf("R2_ENDPOINT must be an HTTPS origin")
	}
	region := os.Getenv("R2_REGION")
	if region == "" {
		region = "auto"
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{HTTPAddr: addr, R2PublicURL: os.Getenv("R2_PUBLIC_URL"), DatabaseURL: db.String(), R2Endpoint: endpoint, R2Region: region, R2Bucket: os.Getenv("R2_BUCKET"), R2AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"), R2SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"), EnableDemoReset: strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_DEMO_RESET")), "true")}, nil
}

func loadEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read .env file")
	}
	values, err := godotenv.Parse(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	if err != nil {
		// Parser errors can contain the entire file, including secrets.
		return fmt.Errorf("invalid .env format; check KEY=VALUE lines and quotes")
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("cannot apply .env configuration")
			}
		}
	}
	return nil
}
