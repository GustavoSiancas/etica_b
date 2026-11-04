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
	apply("007_folder_type_abbreviation.up.sql")
	apply("007_folder_type_abbreviation.up.sql")
	if err := db.Exec("UPDATE expedientes SET status='CERRADO',location='CONSEJO DE ETICA' WHERE id=1").Error; err != nil {
		t.Fatal(err)
	}
	apply("008_folder_state_location.up.sql")
	apply("008_folder_state_location.up.sql")
	service := NewDocumentService(db)
	defaults, err := service.ListFolderTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range defaults {
		want := "EP"
		if kind.Name == "EXPEDIENTE CAUTELAR" {
			want = "MC"
		}
		if kind.Abbreviation == nil || *kind.Abbreviation != want {
			t.Fatalf("default abbreviation: %+v", kind)
		}
	}
	custom, err := service.CreateFolderType(ctx, CreateFolderTypeInput{Name: "  expediente especial  ", Abbreviation: " ee "})
	if err != nil || custom.Name != "EXPEDIENTE ESPECIAL" || custom.Abbreviation == nil || *custom.Abbreviation != "EE" {
		t.Fatalf("custom folder type: %+v %v", custom, err)
	}
	if _, err := service.CreateFolderType(ctx, CreateFolderTypeInput{Name: "Expediente Especial", Abbreviation: "EE"}); err != ErrFolderTypeDuplicate {
		t.Fatalf("duplicate accepted: %v", err)
	}
	types, err := service.ListFolderTypes(ctx)
	if err != nil || len(types) != 3 {
		t.Fatalf("custom type missing from catalogue: %+v %v", types, err)
	}
	var customDate DocumentDate
	if err := json.Unmarshal([]byte(`"2026-11-04"`), &customDate); err != nil {
		t.Fatal(err)
	}
	customFolder, err := service.CreateFolder(ctx, 1, CreateFolderInput{FolderTypeID: custom.ID, OpeningDate: &customDate})
	if err != nil || customFolder.Name != custom.Name || customFolder.FolderTypeID != custom.ID || customFolder.Abbreviation == nil || *customFolder.Abbreviation != "EE" {
		t.Fatalf("custom type not usable: %+v %v", customFolder, err)
	}
	folders, err := service.ListFolders(ctx, 1)
	if err != nil || len(folders) != 3 {
		t.Fatalf("folders: %+v %v", folders, err)
	}
	for _, folder := range folders {
		if folder.Abbreviation == nil {
			t.Fatal("folder abbreviation missing")
		}
		want := "2026-11-04T05:00:00Z"
		if folder.Name == "EXPEDIENTE CAUTELAR" {
			want = "2026-10-01T18:30:00Z"
		}
		if folder.OpeningDate == nil || folder.OpeningDate.Format(time.RFC3339) != want {
			t.Fatalf("backfill: %+v", folder)
		}
	}
	var oldPrincipal FolderResponse
	for _, folder := range folders {
		if folder.Name == "EXPEDIENTE PRINCIPAL" {
			oldPrincipal = folder
		}
	}
	if oldPrincipal.Status != FolderCerrado || oldPrincipal.Location != UbicacionConsejo {
		t.Fatalf("principal did not inherit state: %+v", oldPrincipal)
	}
	// An open sibling remains editable while the principal is closed.
	section, err := service.CreateSection(ctx, 1, CreateSectionInput{Name: "Open sibling", Type: "OTROS", FolderID: &customFolder.ID})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := service.CreateInSection(ctx, 1, section.ID, CreateDocumentInput{Name: "Shared evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateSection(ctx, 1, CreateSectionInput{Name: "Closed principal", Type: "OTROS"}); err != ErrFolderClosed {
		t.Fatalf("closed principal editable: %v", err)
	}
	if _, err := service.UpdateFolderStatus(ctx, 1, oldPrincipal.ID, UpdateFolderStatusInput{Status: FolderAbierto}); err != nil {
		t.Fatal(err)
	}
	principalSection, err := service.CreateSection(ctx, 1, CreateSectionInput{Name: "Principal evidence", Type: "OTROS"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AttachDocument(ctx, 1, principalSection.ID, doc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateFolderStatus(ctx, 1, oldPrincipal.ID, UpdateFolderStatusInput{Status: FolderCerrado}); err != nil {
		t.Fatal(err)
	}
	newName := "Protected"
	if _, err := service.Update(ctx, doc.ID, UpdateDocumentInput{Name: &newName}); err != ErrFolderClosed {
		t.Fatalf("shared document editable: %v", err)
	}
	if _, err := service.UpdateSection(ctx, 1, section.ID, UpdateSectionInput{Name: &newName}); err != nil {
		t.Fatalf("open sibling blocked: %v", err)
	}
	changed, err := service.UpdateFolderLocation(ctx, 1, oldPrincipal.ID, UpdateFolderLocationInput{Location: UbicacionTribunal})
	if err != nil || changed.Status != FolderCerrado || changed.Location != UbicacionTribunal {
		t.Fatalf("closed location change: %+v %v", changed, err)
	}
	if _, err := service.UpdateFolderStatus(ctx, 99999, oldPrincipal.ID, UpdateFolderStatusInput{Status: FolderAbierto}); err != ErrExpedienteNotFound {
		t.Fatalf("missing expediente: %v", err)
	}
	var otherExpediente models.Expediente
	otherExpediente.Description = "Other"
	otherDate := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	otherExpediente.OpeningDate = &otherDate
	if err := db.Create(&otherExpediente).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateFolderStatus(ctx, otherExpediente.ID, oldPrincipal.ID, UpdateFolderStatusInput{Status: FolderAbierto}); err != ErrFolderNotFound {
		t.Fatalf("cross-expediente update accepted: %v", err)
	}
	page, err := NewExpedienteService(db).List(ctx, 20, 0, ExpedienteFilters{Status: FolderCerrado, Location: UbicacionTribunal})
	if err != nil || page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != 1 {
		t.Fatalf("folder filters: %+v %v", page, err)
	}
	// Status and location must come from one folder, not different siblings.
	page, err = NewExpedienteService(db).List(ctx, 20, 0, ExpedienteFilters{Status: FolderCerrado, Location: UbicacionDireccion})
	if err != nil || page.Total != 0 {
		t.Fatalf("cross-folder filter match: %+v %v", page, err)
	}
	expediente := models.Expediente{OpeningDate: func() *time.Time { d := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC); return &d }()}
	if err := db.Create(&expediente).Error; err != nil {
		t.Fatal(err)
	}
	folders, err = service.ListFolders(ctx, expediente.ID)
	if err != nil || len(folders) != 1 || folders[0].OpeningDate == nil || folders[0].OpeningDate.Format(time.RFC3339) != "2026-11-05T05:00:00Z" || folders[0].Status != FolderAbierto || folders[0].Location != UbicacionDireccion {
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
