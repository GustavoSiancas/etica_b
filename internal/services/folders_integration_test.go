package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	apply("009_expediente_status_closing_dates.up.sql")
	apply("009_expediente_status_closing_dates.up.sql")
	apply("010_expediente_history.up.sql")
	apply("010_expediente_history.up.sql")
	apply("011_folder_history_details.up.sql")
	apply("011_folder_history_details.up.sql")
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
	global, err := service.ListAllFolders(ctx, 100, 0, FolderFilters{})
	if err != nil || global.Total != 4 || len(global.Data) != 4 {
		t.Fatalf("global folders: %+v %v", global, err)
	}
	foundPrincipal, foundCustom := false, false
	for i, folder := range global.Data {
		if i > 0 && global.Data[i-1].CreatedAt.Before(folder.CreatedAt) {
			t.Fatal("folders out of order")
		}
		if folder.ID == oldPrincipal.ID {
			foundPrincipal = true
			if folder.Denomination != folder.ExpedienteNumber+" EP" || folder.Status != FolderCerrado || folder.Location != UbicacionTribunal {
				t.Fatalf("principal global identity: %+v", folder)
			}
		}
		if folder.ID == customFolder.ID {
			foundCustom = true
			if folder.Denomination != folder.ExpedienteNumber+" EE" || folder.Status != FolderAbierto || folder.Location != UbicacionDireccion {
				t.Fatalf("custom global identity: %+v", folder)
			}
		}
	}
	if !foundPrincipal || !foundCustom {
		t.Fatal("global list missing folders")
	}
	seen := map[int64]bool{}
	for _, offset := range []int{0, 2, 4} {
		paged, err := service.ListAllFolders(ctx, 2, offset, FolderFilters{})
		want := 2
		if offset == 4 {
			want = 0
		}
		if err != nil || paged.Total != 4 || len(paged.Data) != want || paged.Limit != 2 || paged.Offset != offset || paged.Data == nil {
			t.Fatalf("folder pagination: %+v %v", paged, err)
		}
		for _, folder := range paged.Data {
			if seen[folder.ID] {
				t.Fatal("duplicate across folder pages")
			}
			seen[folder.ID] = true
		}
	}
	principalType := oldPrincipal.FolderTypeID
	expedienteID := int64(1)
	filtered, err := service.ListAllFolders(ctx, 1, 0, FolderFilters{Status: FolderCerrado, Location: UbicacionTribunal, FolderTypeID: &principalType, ExpedienteID: &expedienteID, Q: " ep", From: "2026-11-04", To: "2026-11-04"})
	if err != nil || filtered.Total != 1 || len(filtered.Data) != 1 || filtered.Data[0].ID != oldPrincipal.ID {
		t.Fatalf("combined folder filters: %+v %v", filtered, err)
	}
	for _, filter := range []FolderFilters{{Q: "%"}, {Status: FolderCerrado, Q: custom.Name}, {Q: "no such folder"}, {FolderTypeID: &custom.ID, ExpedienteID: &otherExpediente.ID}} {
		result, err := service.ListAllFolders(ctx, 20, 0, filter)
		if err != nil || result.Total != 0 || result.Data == nil || len(result.Data) != 0 {
			t.Fatalf("empty folder filter %+v: %+v %v", filter, result, err)
		}
	}
	// Date bounds use the full Peruvian calendar day, not the UTC calendar day.
	for _, tc := range []struct {
		instant string
		want    int64
	}{
		{"2026-11-04T04:59:59Z", 0}, {"2026-11-04T05:00:00Z", 1},
		{"2026-11-05T04:59:59Z", 1}, {"2026-11-05T05:00:00Z", 0},
	} {
		if err := db.Model(&models.ExpedienteFolder{}).Where("id = ?", customFolder.ID).Update("opening_date", tc.instant).Error; err != nil {
			t.Fatal(err)
		}
		result, err := service.ListAllFolders(ctx, 20, 0, FolderFilters{FolderTypeID: &custom.ID, From: "2026-11-04", To: "2026-11-04"})
		if err != nil || result.Total != tc.want {
			t.Fatalf("Peru day boundary %s: %+v %v", tc.instant, result, err)
		}
	}
	// Legacy types without abbreviations still have an unambiguous label.
	if err := db.Model(&models.FolderType{}).Where("id = ?", custom.ID).Update("abbreviation", nil).Error; err != nil {
		t.Fatal(err)
	}
	global, err = service.ListAllFolders(ctx, 100, 0, FolderFilters{})
	if err != nil {
		t.Fatal(err)
	}
	for _, folder := range global.Data {
		if folder.ID == customFolder.ID && folder.Denomination != folder.ExpedienteNumber+" "+custom.Name {
			t.Fatalf("legacy designation: %+v", folder)
		}
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
	t.Run("folder close preview", func(t *testing.T) {
		cases := NewExpedienteService(db)
		e, err := cases.Create(ctx, CreateExpedienteInput{OpeningDate: &Date{Time: time.Now()}, InitType: "DENUNCIA_DE_OFICIO"})
		if err != nil {
			t.Fatal(err)
		}
		folders, err := service.ListFolders(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		principal := folders[0]
		before, err := cases.ListHistory(ctx, e.ID, 1, 0)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := service.GetFolderCloseStatus(ctx, e.ID, principal.ID)
		if err != nil || !preview.WillCloseExpediente || preview.FolderID != principal.ID || preview.FolderStatus != FolderAbierto || preview.ExpedienteStatus != FolderAbierto {
			t.Fatalf("single folder: %+v %v", preview, err)
		}
		after, err := cases.ListHistory(ctx, e.ID, 1, 0)
		if err != nil || after.Total != before.Total {
			t.Fatalf("preview changed history: %+v %v", after, err)
		}
		extra, err := service.CreateFolder(ctx, e.ID, CreateFolderInput{FolderTypeID: kind.ID, OpeningDate: &DocumentDate{Time: time.Now()}})
		if err != nil {
			t.Fatal(err)
		}
		preview, err = service.GetFolderCloseStatus(ctx, e.ID, principal.ID)
		if err != nil || preview.WillCloseExpediente {
			t.Fatalf("two open folders: %+v %v", preview, err)
		}
		if _, err := service.UpdateFolderStatus(ctx, e.ID, extra.ID, UpdateFolderStatusInput{Status: FolderCerrado}); err != nil {
			t.Fatal(err)
		}
		preview, err = service.GetFolderCloseStatus(ctx, e.ID, extra.ID)
		if err != nil || preview.WillCloseExpediente || preview.FolderStatus != FolderCerrado {
			t.Fatalf("closed target: %+v %v", preview, err)
		}
		preview, err = service.GetFolderCloseStatus(ctx, e.ID, principal.ID)
		if err != nil || !preview.WillCloseExpediente {
			t.Fatalf("last open: %+v %v", preview, err)
		}
		if _, err := service.UpdateFolderStatus(ctx, e.ID, principal.ID, UpdateFolderStatusInput{Status: FolderCerrado}); err != nil {
			t.Fatal(err)
		}
		preview, err = service.GetFolderCloseStatus(ctx, e.ID, principal.ID)
		if err != nil || preview.WillCloseExpediente || preview.ExpedienteStatus != FolderCerrado {
			t.Fatalf("closed case: %+v %v", preview, err)
		}
		if _, err := cases.UpdateStatus(ctx, e.ID, UpdateExpedienteStatusInput{Status: FolderAbierto}); err != nil {
			t.Fatal(err)
		}
		preview, err = service.GetFolderCloseStatus(ctx, e.ID, principal.ID)
		if err != nil || preview.WillCloseExpediente {
			t.Fatalf("reopened case with closed folders: %+v %v", preview, err)
		}
		if _, err := service.GetFolderCloseStatus(ctx, e.ID, 99999999); err != ErrFolderNotFound {
			t.Fatalf("missing folder: %v", err)
		}
		if _, err := service.GetFolderCloseStatus(ctx, expediente.ID, principal.ID); err != ErrFolderNotFound {
			t.Fatalf("foreign folder: %v", err)
		}
		if _, err := service.GetFolderCloseStatus(ctx, 99999999, principal.ID); err != ErrExpedienteNotFound {
			t.Fatalf("missing case: %v", err)
		}
	})

	t.Run("folder location and private content snapshot", func(t *testing.T) {
		cases := NewExpedienteService(db)
		e, err := cases.Create(ctx, CreateExpedienteInput{OpeningDate: &Date{Time: time.Now()}, InitType: "DENUNCIA_DE_OFICIO"})
		if err != nil {
			t.Fatal(err)
		}
		folders, err := service.ListFolders(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		folder := folders[0]
		root := models.ExpedienteSection{ExpedienteID: e.ID, ExpedienteFolderID: folder.ID, Name: "Section audit", Type: "OTROS", SortOrder: 1}
		if err := db.Create(&root).Error; err != nil {
			t.Fatal(err)
		}
		child := models.ExpedienteSection{ExpedienteID: e.ID, ExpedienteFolderID: folder.ID, ExpedienteSectionParentID: &root.ID, Name: "Subsection audit", Type: "OTROS", SortOrder: 2}
		if err := db.Create(&child).Error; err != nil {
			t.Fatal(err)
		}
		doc := models.Document{Name: "Original document"}
		if err := db.Create(&doc).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.ExpedienteSectionDocument{ExpedienteSectionID: child.ID, DocumentID: doc.ID}).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := service.UpdateFolderLocation(ctx, e.ID, folder.ID, UpdateFolderLocationInput{Location: UbicacionConsejo}); err != nil {
			t.Fatal(err)
		}
		history, err := cases.ListFolderHistory(ctx, e.ID, folder.ID, 20, 0)
		if err != nil || len(history.Data) != 2 || history.Data[0].Location == nil || *history.Data[0].Location != UbicacionConsejo {
			t.Fatalf("folder location history: %+v %v", history, err)
		}
		eventID := history.Data[0].ID
		var audit struct{ ContentSnapshot string }
		readSnapshot := func() string {
			t.Helper()
			if err := db.Table("expediente_history").Select("content_snapshot::text AS content_snapshot").Where("id = ?", eventID).Take(&audit).Error; err != nil {
				t.Fatal(err)
			}
			return audit.ContentSnapshot
		}
		original := readSnapshot()
		var snapshot struct {
			SchemaVersion int `json:"schema_version"`
			Sections      []struct {
				ID        int64  `json:"id"`
				ParentID  *int64 `json:"parent_id"`
				Name      string `json:"name"`
				Documents []struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
				} `json:"documents"`
			} `json:"sections"`
		}
		if err := json.Unmarshal([]byte(original), &snapshot); err != nil || snapshot.SchemaVersion != 1 || len(snapshot.Sections) != 2 {
			t.Fatalf("content snapshot: %s %v", original, err)
		}
		section := snapshot.Sections[1]
		if section.ID != child.ID || section.ParentID == nil || *section.ParentID != root.ID || len(section.Documents) != 1 || section.Documents[0].ID != doc.ID || section.Documents[0].Name != doc.Name {
			t.Fatalf("lost content relation: %+v", section)
		}
		if err := db.Model(&doc).Update("name", "Renamed document").Error; err != nil {
			t.Fatal(err)
		}
		if readSnapshot() != original {
			t.Fatal("historical snapshot changed")
		}
		encoded, err := json.Marshal(history)
		if err != nil || strings.Contains(string(encoded), "content_snapshot") || strings.Contains(string(encoded), "Original document") || strings.Contains(string(encoded), "active_folders") {
			t.Fatalf("private data exposed: %s %v", encoded, err)
		}
		if _, err := cases.UpdateStatus(ctx, e.ID, UpdateExpedienteStatusInput{Status: FolderCerrado}); err != nil {
			t.Fatal(err)
		}
		latest, err := cases.ListFolderHistory(ctx, e.ID, folder.ID, 1, 0)
		if err != nil || latest.Data[0].Event != "CARPETA_CERRADA" || latest.Data[0].Location == nil || *latest.Data[0].Location != UbicacionConsejo {
			t.Fatalf("cascade history: %+v %v", latest, err)
		}
		if _, err := cases.ListFolderHistory(ctx, e.ID, 99999999, 20, 0); err != ErrFolderNotFound {
			t.Fatalf("missing folder: %v", err)
		}
		if _, err := cases.ListFolderHistory(ctx, expediente.ID, folder.ID, 20, 0); err != ErrFolderNotFound {
			t.Fatalf("foreign folder: %v", err)
		}
	})

	t.Run("closure dates and cascade", func(t *testing.T) {
		cases := NewExpedienteService(db)
		e, err := cases.Create(ctx, CreateExpedienteInput{OpeningDate: &Date{Time: time.Now()}, InitType: "DENUNCIA_DE_OFICIO"})
		if err != nil {
			t.Fatal(err)
		}
		folders, err := service.ListFolders(ctx, e.ID)
		if err != nil || len(folders) != 1 {
			t.Fatalf("folders: %v %v", folders, err)
		}
		principal := folders[0]
		extra, err := service.CreateFolder(ctx, e.ID, CreateFolderInput{FolderTypeID: kind.ID, OpeningDate: &DocumentDate{Time: time.Now()}})
		if err != nil {
			t.Fatal(err)
		}
		closed, err := service.UpdateFolderStatus(ctx, e.ID, principal.ID, UpdateFolderStatusInput{Status: FolderCerrado})
		if err != nil || closed.ClosingDate == nil {
			t.Fatalf("close principal: %+v %v", closed, err)
		}
		old := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		if err := db.Model(&models.ExpedienteFolder{}).Where("id = ?", principal.ID).Update("closing_date", old).Error; err != nil {
			t.Fatal(err)
		}
		var current models.Expediente
		if err := db.First(&current, e.ID).Error; err != nil || current.Status != FolderAbierto {
			t.Fatalf("premature close: %+v %v", current, err)
		}
		if _, err := service.UpdateFolderStatus(ctx, e.ID, extra.ID, UpdateFolderStatusInput{Status: FolderCerrado}); err != nil {
			t.Fatal(err)
		}
		if err := db.First(&current, e.ID).Error; err != nil || current.Status != FolderCerrado || current.ClosingDate == nil {
			t.Fatalf("last folder did not close case: %+v %v", current, err)
		}
		reopened, err := cases.UpdateStatus(ctx, e.ID, UpdateExpedienteStatusInput{Status: FolderAbierto})
		if err != nil || reopened.ClosingDate != nil || reopened.Status != FolderAbierto {
			t.Fatalf("reopen case: %+v %v", reopened, err)
		}
		folders, err = service.ListFolders(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range folders {
			if f.Status != FolderCerrado || f.ClosingDate == nil {
				t.Fatalf("case reopened folder: %+v", f)
			}
		}
		opened, err := service.UpdateFolderStatus(ctx, e.ID, extra.ID, UpdateFolderStatusInput{Status: FolderAbierto})
		if err != nil || opened.ClosingDate != nil {
			t.Fatalf("reopen folder: %+v %v", opened, err)
		}
		result, err := cases.UpdateStatus(ctx, e.ID, UpdateExpedienteStatusInput{Status: FolderCerrado})
		if err != nil || result.ClosingDate == nil {
			t.Fatalf("cascade: %+v %v", result, err)
		}
		folders, err = service.ListFolders(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range folders {
			if f.Status != FolderCerrado || f.ClosingDate == nil {
				t.Fatalf("cascade missed folder: %+v", f)
			}
			if f.ID == principal.ID && !f.ClosingDate.Equal(old) {
				t.Fatalf("overwrote historical date: %+v", f)
			}
		}
		again, err := service.UpdateFolderStatus(ctx, e.ID, principal.ID, UpdateFolderStatusInput{Status: FolderCerrado})
		if err != nil || again.ClosingDate == nil || !again.ClosingDate.Equal(old) {
			t.Fatalf("repeat folder close changed date: %+v %v", again, err)
		}

		repeated, err := cases.UpdateStatus(ctx, e.ID, UpdateExpedienteStatusInput{Status: FolderCerrado})
		if err != nil || repeated.ClosingDate == nil || !repeated.ClosingDate.Equal(result.ClosingDate.Time) {
			t.Fatalf("repeat close: %+v %v", repeated, err)
		}
		for _, id := range []int64{principal.ID, extra.ID} {
			if _, err := service.UpdateFolderStatus(ctx, e.ID, id, UpdateFolderStatusInput{Status: FolderAbierto}); err != nil {
				t.Fatal(err)
			}
		}
		failures := make(chan error, 2)
		for _, id := range []int64{principal.ID, extra.ID} {
			go func(id int64) {
				_, err := service.UpdateFolderStatus(ctx, e.ID, id, UpdateFolderStatusInput{Status: FolderCerrado})
				failures <- err
			}(id)
		}
		for i := 0; i < 2; i++ {
			if err := <-failures; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.First(&current, e.ID).Error; err != nil || current.Status != FolderCerrado || current.ClosingDate == nil {
			t.Fatalf("concurrent last closures: %+v %v", current, err)
		}

		history, err := cases.ListHistory(ctx, e.ID, 100, 0)
		if err != nil || history.Total < 10 {
			t.Fatalf("history: %+v %v", history, err)
		}
		encoded, err := json.Marshal(history)
		if err != nil || strings.Contains(string(encoded), "active_folders") {
			t.Fatalf("audit snapshot exposed: %s %v", encoded, err)
		}
		var audit struct{ ActiveFolders string }
		if err := db.Table("expediente_history").Select("active_folders::text AS active_folders").Where("expediente_id = ? AND folder_id = ? AND event = ?", e.ID, extra.ID, "CARPETA_ABIERTA").Order("id").Take(&audit).Error; err != nil {
			t.Fatal(err)
		}
		var snapshot []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(audit.ActiveFolders), &snapshot); err != nil || len(snapshot) != 2 {
			t.Fatalf("snapshot: %s %v", audit.ActiveFolders, err)
		}
		if snapshot[0].ID != principal.ID || snapshot[0].Name == "" || snapshot[1].ID != extra.ID || snapshot[1].Name == "" {
			t.Fatalf("snapshot identities: %+v", snapshot)
		}
		last := history.Data[0]
		if last.Event != "EXPEDIENTE_CERRADO" || last.FolderID != nil || last.ClosingDate == nil {
			t.Fatalf("latest event: %+v", last)
		}
		if _, err := cases.UpdateStatus(ctx, e.ID, UpdateExpedienteStatusInput{Status: FolderCerrado}); err != nil {
			t.Fatal(err)
		}
		after, err := cases.ListHistory(ctx, e.ID, 1, 0)
		if err != nil || after.Total != history.Total {
			t.Fatalf("noop duplicated history: %+v %v", after, err)
		}
		// A failed history write must roll back both the folder and expediente changes.
		if err := db.Exec(`CREATE FUNCTION reject_history_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test history failure'; END; $$;
 CREATE TRIGGER reject_history_test BEFORE INSERT ON expediente_history FOR EACH ROW EXECUTE FUNCTION reject_history_test()`).Error; err != nil {
			t.Fatal(err)
		}
		_, err = service.UpdateFolderStatus(ctx, e.ID, extra.ID, UpdateFolderStatusInput{Status: FolderAbierto})
		if dropErr := db.Exec("DROP TRIGGER reject_history_test ON expediente_history; DROP FUNCTION reject_history_test()").Error; dropErr != nil {
			t.Fatal(dropErr)
		}
		if err == nil {
			t.Fatal("history failure ignored")
		}
		var unchanged models.ExpedienteFolder
		if err := db.First(&unchanged, extra.ID).Error; err != nil || unchanged.Status != FolderCerrado || unchanged.ClosingDate == nil {
			t.Fatalf("history failure did not rollback: %+v %v", unchanged, err)
		}

	})

}
