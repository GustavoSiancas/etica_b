package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"etica_back/internal/config"
	"etica_back/internal/persistence/models"
	platform "etica_back/internal/platform/postgres"
	"github.com/jackc/pgx/v5"
)

func TestFixedParticipantsPostgres(t *testing.T) {
	if os.Getenv("ETICA_INTEGRATION") != "1" {
		t.Skip("set ETICA_INTEGRATION=1 to test PostgreSQL")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("configuration unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, pool, err := platform.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer pool.Close()
	schema := fmt.Sprintf("etica_fixed_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if err := admin.Exec("CREATE SCHEMA " + quoted).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.Exec("DROP SCHEMA " + quoted + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal("invalid database URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, testPool, err := platform.Open(ctx, u.String())
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer testPool.Close()
	apply := func(path string) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(data)).Error; err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	apply("migrations/001_initial.up.sql")
	for _, migration := range []string{"004_expediente_folders.up.sql", "005_document_date.up.sql", "006_folder_opening_date.up.sql", "007_folder_type_abbreviation.up.sql", "008_folder_state_location.up.sql"} {
		apply("migrations/" + migration)
	}
	if err := db.Exec(`INSERT INTO expedientes(description,opening_date,init_type) VALUES ('Flow fixture','2026-01-01','DENUNCIA_DE_OFICIO'); INSERT INTO denunciantes(name,document_type,document_number) VALUES ('Ana','DNI','00000001'); INSERT INTO denunciados(name,document_type,document_number,registration_number) VALUES ('Luis','DNI','00000002',1); INSERT INTO expediente_denunciantes(expediente_id,denunciante_id) VALUES (1,1); INSERT INTO expediente_denunciados(expediente_id,denunciado_id) VALUES (1,1);`).Error; err != nil {
		t.Fatal(err)
	}

	participants := NewParticipantService(db)
	a, err := participants.ListDenunciantesByExpediente(ctx, 1)
	if err != nil || len(a) != 1 {
		t.Fatalf("missing expediente complainant: %v %v", a, err)
	}
	b, err := participants.ListDenunciadosByExpediente(ctx, 1)
	if err != nil || len(b) != 1 {
		t.Fatalf("missing expediente respondent: %v %v", b, err)
	}
	if _, err := participants.AttachDenunciante(ctx, 1, 1); !errors.Is(err, ErrParticipantDuplicate) {
		t.Fatalf("duplicate complainant: %v", err)
	}
	if _, err := participants.AttachDenunciado(ctx, 1, 1); !errors.Is(err, ErrParticipantDuplicate) {
		t.Fatalf("duplicate respondent: %v", err)
	}
	opening := &Date{Time: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)}
	description := strings.Repeat("Long description ", 100)
	createdExp, err := NewExpedienteService(db).Create(ctx, CreateExpedienteInput{Description: description, OpeningDate: opening, InitType: "DENUNCIA_DE_OFICIO", DenuncianteIDs: []int64{1}, DenunciadoIDs: []int64{1}})
	if err != nil || createdExp.Description != strings.TrimSpace(description) {
		t.Fatalf("long description: %+v %v", createdExp, err)
	}
	attached, err := participants.ListDenunciantesByExpediente(ctx, createdExp.ID)
	if err != nil || len(attached) != 1 || attached[0].Participant.ID != 1 {
		t.Fatalf("initial participants not attached: %+v %v", attached, err)
	}
	before, err := NewExpedienteService(db).List(ctx, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewExpedienteService(db).Create(ctx, CreateExpedienteInput{Description: "Invalid participants", OpeningDate: opening, InitType: "DENUNCIA_DE_PARTE", DenuncianteIDs: []int64{1}, DenunciadoIDs: []int64{999999}})
	if !errors.Is(err, ErrParticipantNotFound) {
		t.Fatalf("missing participant: %v", err)
	}
	after, err := NewExpedienteService(db).List(ctx, 100, 0)
	if err != nil || after.Total != before.Total {
		t.Fatal("partial expediente created")
	}
	documents := NewDocumentService(db)
	section, err := documents.CreateSection(ctx, 1, CreateSectionInput{Name: "Denuncia", Type: "OTROS"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := documents.CreateInSection(ctx, 1, section.ID, CreateDocumentInput{Name: "Document"}); err != nil {
		t.Fatal(err)
	}
	person, err := participants.CreateDenunciante(ctx, CreateDenuncianteInput{Name: "Second participant", DocumentType: "DNI", DocumentNumber: "00123456"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := participants.AttachDenunciante(ctx, 1, person.ID); err != nil {
		t.Fatal(err)
	}
	respondent, err := participants.CreateDenunciado(ctx, CreateDenunciadoInput{Name: "New respondent", DocumentType: " ce ", DocumentNumber: " 000123456 ", RegistrationNumber: 12345})
	if err != nil {
		t.Fatal(err)
	}
	if respondent.DocumentType == nil || *respondent.DocumentType != "CE" || respondent.DocumentNumber == nil || *respondent.DocumentNumber != "000123456" || respondent.RegistrationNumber == nil || *respondent.RegistrationNumber != 12345 {
		t.Fatalf("respondent identity: %+v", respondent)
	}
	if _, err := participants.AttachDenunciado(ctx, 1, respondent.ID); err != nil {
		t.Fatal(err)
	}
	memberships, err := participants.ListDenunciadosByExpediente(ctx, 1)
	if err != nil || len(memberships) != 2 || memberships[1].Participant.RegistrationNumber == nil || *memberships[1].Participant.RegistrationNumber != 12345 {
		t.Fatalf("membership identity lost: %+v %v", memberships, err)
	}
	listed, _, err := participants.ListDenunciantes(ctx, 100, 0, ParticipantFilters{})
	if err != nil || len(listed) != 2 || listed[1].DocumentNumber == nil || *listed[1].DocumentNumber != "00123456" {
		t.Fatalf("complainant identity lost: %+v %v", listed, err)
	}
	rows, err := documents.ListByExpediente(ctx, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("documents: %v %v", rows, err)
	}
	types, err := documents.ListTypes(ctx)
	if err != nil || len(types) != 3 {
		t.Fatalf("types: %v %v", types, err)
	}
	if err := db.Exec("DELETE FROM document_types").Error; err != nil {
		t.Fatal(err)
	}
	independent, err := documents.ListByExpediente(ctx, 1)
	if err != nil || len(independent) != 1 {
		t.Fatalf("dictionary independence: %v %v", independent, err)
	}
	a, err = participants.ListDenunciantesByExpediente(ctx, 1)
	if err != nil || len(a) != 2 {
		t.Fatalf("all expediente participants must remain visible: %v %v", a, err)
	}
	var beforeSeed, afterSeed int64
	if err := db.Model(&models.Denunciado{}).Count(&beforeSeed).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		// Startup import is unconditional; the dev reload flag is independent.
		summary, err := NewDemoDataService(db, "seeds/demo_data.sql", false).Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			afterSeed = summary.Denunciados
		}
		if summary.Denunciados <= beforeSeed || summary.Denunciados != afterSeed {
			t.Fatalf("non-idempotent seed: %+v", summary)
		}
		var count int64
		if err := db.Model(&models.Expediente{}).Count(&count).Error; err != nil || count != after.Total {
			t.Fatalf("seed changed expedientes: %d %v", count, err)
		}
		if err := db.Model(&models.Document{}).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("seed changed documents: %d %v", count, err)
		}
		var preserved models.Denunciado
		if err := db.First(&preserved, respondent.ID).Error; err != nil || preserved.Name != respondent.Name {
			t.Fatal("seed modified existing participant")
		}
	}
	if _, err := NewDemoDataService(db, "seeds/demo_data.sql", false).Reset(ctx); !errors.Is(err, ErrDemoResetDisabled) {
		t.Fatal("development reload flag ignored")
	}
	// Concurrent restarts serialize safely and only insert missing CALs.
	loadResults := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := NewDemoDataService(db, "seeds/demo_data.sql", false).Load(ctx); loadResults <- err }()
	}
	for i := 0; i < 2; i++ {
		if err := <-loadResults; err != nil {
			t.Fatal(err)
		}
	}
	var finalParticipants int64
	if err := db.Model(&models.Denunciado{}).Count(&finalParticipants).Error; err != nil || finalParticipants != afterSeed {
		t.Fatal("concurrent import duplicated participants")
	}
	// Upgrade a previously loaded name-only demo record in place.
	if err := db.Exec("INSERT INTO denunciantes(name,is_demo) VALUES ('Interbank',TRUE)").Error; err != nil {
		t.Fatal(err)
	}
	// The extra seed inserts missing demo participants without duplicating them.
	for i := 0; i < 2; i++ {
		summary, err := NewDemoDataService(db, "seeds/demo_data_denuncianes.sql", false).Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if summary.Denunciantes != 52 || summary.Denunciados != afterSeed {
			t.Fatalf("complainant seed: %+v", summary)
		}
	}
	var demoCount int64
	if err := db.Model(&models.Denunciante{}).Where("is_demo = TRUE AND document_type IS NOT NULL AND document_number IS NOT NULL").Count(&demoCount).Error; err != nil || demoCount != 50 {
		t.Fatalf("demo complainants with fictional documents: %d %v", demoCount, err)
	}
	for kind, want := range map[string]int64{"RUC": 25, "DNI": 20, "CE": 5} {
		var got int64
		if err := db.Model(&models.Denunciante{}).Where("is_demo=TRUE AND document_type=?", kind).Count(&got).Error; err != nil || got != want {
			t.Fatalf("demo type %s: %d %v", kind, got, err)
		}
	}
	// Existing populated documents must not be overwritten by later reloads.
	if err := db.Exec("UPDATE denunciantes SET document_number='20123456789' WHERE name='Interbank' AND is_demo=TRUE").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewDemoDataService(db, "seeds/demo_data_denuncianes.sql", false).Load(ctx); err != nil {
		t.Fatal(err)
	}
	var preservedNumber string
	if err := db.Table("denunciantes").Select("document_number").Where("name='Interbank' AND is_demo=TRUE").Scan(&preservedNumber).Error; err != nil || preservedNumber != "20123456789" {
		t.Fatal("seed overwrote an existing document")
	}
	// The same name matches with accents, N/n, case changes and decomposed text.
	for _, name := range []string{"__Busqueda__ Jos\u00e9 Mu\u00f1oz \u00c1lvarez", "__Busqueda__ Jose\u0301 Mun\u0303oz A\u0301lvarez", "__Busqueda__ Jose Munoz Alvarez"} {
		if err := db.Exec("INSERT INTO denunciantes(name,document_type,document_number) VALUES (?,'DNI','76543210');", name).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO denunciados(name,document_type,document_number,registration_number) VALUES (?,'DNI','76543210',99900001);", name).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, term := range []string{"__busqueda__ jose munoz alvarez", "__BUSQUEDA__ JOS\u00c9 MU\u00d1OZ \u00c1LVAREZ", "__Busqueda__ Jose\u0301 Mun\u0303oz A\u0301lvarez"} {
		for _, filter := range []ParticipantFilters{{Name: term}, {Q: term}} {
			for _, kind := range []string{"denunciante", "denunciado"} {
				var rows []ParticipantResponse
				var total int64
				var err error
				if kind == "denunciante" {
					rows, total, err = participants.ListDenunciantes(ctx, 1, 1, filter)
				} else {
					rows, total, err = participants.ListDenunciados(ctx, 1, 1, filter)
				}
				if err != nil || total != 3 || len(rows) != 1 {
					t.Fatalf("normalized %s search: total=%d rows=%d error=%v", kind, total, len(rows), err)
				}
				if !strings.Contains(rows[0].Name, "__Busqueda__") {
					t.Fatal("stored name unexpectedly changed")
				}
			}
		}
	}
	// Participant-scoped lists combine membership, state/location and paging.
	scopedComplainant, err := participants.CreateDenunciante(ctx, CreateDenuncianteInput{Name: "Scoped complainant", DocumentType: "DNI", DocumentNumber: "88000001"})
	if err != nil {
		t.Fatal(err)
	}
	scopedRespondent, err := participants.CreateDenunciado(ctx, CreateDenunciadoInput{Name: "Scoped respondent", DocumentType: "DNI", DocumentNumber: "88000002", RegistrationNumber: 99990001})
	if err != nil {
		t.Fatal(err)
	}
	cases := NewExpedienteService(db)
	var scopedIDs []int64
	for i := 0; i < 2; i++ {
		caseDate := &Date{Time: opening.AddDate(0, 0, i-1)}
		row, err := cases.Create(ctx, CreateExpedienteInput{OpeningDate: caseDate, InitType: "DENUNCIA_DE_PARTE", DenuncianteIDs: []int64{scopedComplainant.ID}, DenunciadoIDs: []int64{scopedRespondent.ID}})
		if err != nil {
			t.Fatal(err)
		}
		scopedIDs = append(scopedIDs, row.ID)
	}
	scopedFolders, err := documents.ListFolders(ctx, scopedIDs[1])
	if err != nil || len(scopedFolders) != 1 {
		t.Fatal("missing principal folder")
	}
	if _, err := documents.UpdateFolderLocation(ctx, scopedIDs[1], scopedFolders[0].ID, UpdateFolderLocationInput{Location: UbicacionConsejo}); err != nil {
		t.Fatal(err)
	}
	if _, err := documents.UpdateFolderStatus(ctx, scopedIDs[1], scopedFolders[0].ID, UpdateFolderStatusInput{Status: FolderCerrado}); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"denunciante", "denunciado"} {
		id := scopedComplainant.ID
		list := cases.ListByDenunciante
		if role == "denunciado" {
			id = scopedRespondent.ID
			list = cases.ListByDenunciado
		}
		for offset := 0; offset < 3; offset++ {
			page, err := list(ctx, id, 1, offset, ExpedienteFilters{})
			want := 1
			if offset == 2 {
				want = 0
			}
			if err != nil || page.Total != 2 || len(page.Data) != want {
				t.Fatalf("scoped %s offset %d: %+v %v", role, offset, page, err)
			}
			if offset < 2 && page.Data[0].ID != scopedIDs[1-offset] {
				t.Fatal("scoped order or membership incorrect")
			}
		}
		page, err := list(ctx, id, 20, 0, ExpedienteFilters{Status: FolderCerrado, Location: UbicacionConsejo})
		if err != nil || page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != scopedIDs[1] {
			t.Fatalf("combined scoped filters: %+v %v", page, err)
		}
		page, err = list(ctx, id, 20, 0, ExpedienteFilters{Status: FolderAbierto, Location: UbicacionConsejo})
		if err != nil || page.Total != 0 || len(page.Data) != 0 || page.Data == nil {
			t.Fatal("empty scoped list must be [] with total 0")
		}
		if _, err := list(ctx, 9223372036854775807, 20, 0, ExpedienteFilters{}); !errors.Is(err, ErrParticipantNotFound) {
			t.Fatalf("missing participant: %v", err)
		}
	}
	// All three lists filter opening_date inclusively before counting/paging.
	for _, test := range []struct {
		filter ExpedienteFilters
		want   int64
	}{
		{ExpedienteFilters{From: "2026-10-21", To: "2026-10-22"}, 2},
		{ExpedienteFilters{From: "2026-10-21", To: "2026-10-21"}, 1},
		{ExpedienteFilters{From: "2026-10-22"}, 1},
		{ExpedienteFilters{To: "2026-10-21"}, 1},
		{ExpedienteFilters{From: "2026-10-23"}, 0},
		{ExpedienteFilters{From: "2026-10-22", To: "2026-10-22", Status: FolderCerrado, Location: UbicacionConsejo}, 1},
	} {
		for _, role := range []string{"denunciante", "denunciado"} {
			id := scopedComplainant.ID
			list := cases.ListByDenunciante
			if role == "denunciado" {
				id = scopedRespondent.ID
				list = cases.ListByDenunciado
			}
			page, err := list(ctx, id, 1, 0, test.filter)
			wantRows := int(test.want)
			if wantRows > 1 {
				wantRows = 1
			}
			if err != nil || page.Total != test.want || len(page.Data) != wantRows {
				t.Fatalf("date range %s %+v: %+v %v", role, test.filter, page, err)
			}
			page, err = list(ctx, id, 1, 2, test.filter)
			if err != nil || page.Total != test.want || len(page.Data) != 0 {
				t.Fatal("date total changed with offset")
			}
		}
	}
	// Older records with a NULL opening date are excluded by any date bound.
	if err := db.Exec("INSERT INTO expedientes(description,opening_date) VALUES ('No opening date','2026-01-01'); UPDATE expedientes SET opening_date=NULL WHERE description='No opening date'").Error; err != nil {
		t.Fatal(err)
	}
	generalPage, err := cases.List(ctx, 20, 0, ExpedienteFilters{From: "2026-10-21", To: "2026-10-21"})
	if err != nil || generalPage.Total != 1 || len(generalPage.Data) != 1 || generalPage.Data[0].ID != scopedIDs[0] {
		t.Fatalf("general opening range: %+v %v", generalPage, err)
	}
	emptyPerson, err := participants.CreateDenunciante(ctx, CreateDenuncianteInput{Name: "No cases", DocumentType: "DNI", DocumentNumber: "88000003"})
	if err != nil {
		t.Fatal(err)
	}
	emptyPage, err := cases.ListByDenunciante(ctx, emptyPerson.ID, 20, 0, ExpedienteFilters{})
	if err != nil || emptyPage.Total != 0 || emptyPage.Data == nil {
		t.Fatal("existing participant with no cases must return an empty page")
	}
	if err := NewDatabaseClearService(db, true).Clear(ctx); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.Expediente{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("clear: count=%d err=%v", count, err)
	}
}
