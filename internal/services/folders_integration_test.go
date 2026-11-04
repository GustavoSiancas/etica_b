package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"etica_back/internal/persistence/models"
	platform "etica_back/internal/platform/postgres"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"
)

func TestFolderOpeningDatePostgres(t *testing.T) {
	dsn := os.Getenv("ETICA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ETICA_TEST_DATABASE_URL for PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, pool, err := platform.Open(ctx, dsn)
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer pool.Close()
	schema := fmt.Sprintf("etica_folder_dates_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if err := admin.Exec("CREATE SCHEMA " + quoted).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.Exec("DROP SCHEMA " + quoted + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid test URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, testPool, err := platform.Open(ctx, u.String())
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer testPool.Close()
	apply := func(name string) {
		t.Helper()
		if err := platform.ApplyUpgradeMigration(ctx, db, filepath.Join("..", "..", "migrations", name)); err != nil {
			t.Fatal(err)
		}
	}
	apply("001_initial.up.sql")
	apply("004_expediente_folders.up.sql")
	if err := db.Exec(`INSERT INTO expedientes(description,opening_date) VALUES ('Existing','2026-11-04');
 INSERT INTO expediente_folders(expediente_id,folder_type_id,created_at)
 SELECT 1,id,'2026-10-01T18:30:00Z' FROM folder_types WHERE name='EXPEDIENTE CAUTELAR'`).Error; err != nil {
		t.Fatal(err)
	}
	// Run in a different session timezone to detect accidental implicit conversions.
	if err := db.Connection(func(conn *gorm.DB) error {
		if err := conn.Exec("SET TIME ZONE 'Asia/Tokyo'").Error; err != nil {
			return err
		}
		defer conn.Exec("SET TIME ZONE 'UTC'")
		migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "006_folder_opening_date.up.sql"))
		if err != nil {
			return err
		}
		return conn.Exec(string(migration)).Error
	}); err != nil {
		t.Fatal(err)
	}
	apply("006_folder_opening_date.up.sql") // Versioned migration must be safe to rerun.
	service := NewDocumentService(db)
	folders, err := service.ListFolders(ctx, 1)
	if err != nil || len(folders) != 2 {
		t.Fatalf("folders: %+v %v", folders, err)
	}
	for _, folder := range folders {
		want := "2026-11-04T05:00:00Z"
		if folder.Name == "EXPEDIENTE CAUTELAR" {
			want = "2026-10-01T18:30:00Z"
		}
		if folder.OpeningDate == nil || folder.OpeningDate.Format(time.RFC3339) != want {
			t.Fatalf("backfill: %+v", folder)
		}
	}
	expediente := models.Expediente{OpeningDate: func() *time.Time { d := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC); return &d }()}
	if err := db.Create(&expediente).Error; err != nil {
		t.Fatal(err)
	}
	folders, err = service.ListFolders(ctx, expediente.ID)
	if err != nil || len(folders) != 1 || folders[0].OpeningDate == nil || folders[0].OpeningDate.Format(time.RFC3339) != "2026-11-05T05:00:00Z" {
		t.Fatalf("principal trigger: %+v %v", folders, err)
	}
	var kind models.FolderType
	if err := db.Where("name = ?", "EXPEDIENTE CAUTELAR").First(&kind).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateFolder(ctx, expediente.ID, CreateFolderInput{FolderTypeID: kind.ID}); err != ErrSectionInvalid {
		t.Fatalf("missing date accepted: %v", err)
	}
	var input CreateFolderInput
	if err := json.Unmarshal([]byte(fmt.Sprintf(`{"folder_type_id":%d,"opening_date":"2026-11-06T23:30:00-05:00"}`, kind.ID)), &input); err != nil {
		t.Fatal(err)
	}
	folder, err := service.CreateFolder(ctx, expediente.ID, input)
	if err != nil || folder.OpeningDate == nil || folder.OpeningDate.Format(time.RFC3339) != "2026-11-07T04:30:00Z" {
		t.Fatalf("custom opening date: %+v %v", folder, err)
	}
	again, err := service.CreateFolder(ctx, expediente.ID, CreateFolderInput{FolderTypeID: kind.ID})
	if err != nil || again.ID != folder.ID || again.OpeningDate == nil || !again.OpeningDate.Equal(*folder.OpeningDate) {
		t.Fatalf("idempotent creation: %+v %v", again, err)
	}
}
