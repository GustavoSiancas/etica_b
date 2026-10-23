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
	var substage models.DocumentSubstage
	if err := db.First(&substage).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := documents.Create(ctx, 1, CreateDocumentInput{Title: "Document", Stage: substage.Name, Type: "OTROS"}); err != nil {
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
	listed, _, err := participants.ListDenunciantes(ctx, 100, 0)
	if err != nil || len(listed) != 2 || listed[1].DocumentNumber == nil || *listed[1].DocumentNumber != "00123456" {
		t.Fatalf("complainant identity lost: %+v %v", listed, err)
	}
	rows, err := documents.ListByExpediente(ctx, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("documents: %v %v", rows, err)
	}
	stageCatalog, err := documents.ListStages(ctx)
	if err != nil || len(stageCatalog) == 0 {
		t.Fatalf("stage dictionary: %v", err)
	}
	typeCatalog, err := documents.ListTypes(ctx)
	if err != nil || len(typeCatalog) != 3 || typeCatalog[0].Code != "ESCRITO_DE_OFICIO" || typeCatalog[2].Code != "OTROS" {
		t.Fatalf("type dictionary: %v %v", typeCatalog, err)
	}
	for _, stage := range []string{stageCatalog[0].Name, "OTROS", "Etapa personalizada"} {
		created, err := documents.Create(ctx, 1, CreateDocumentInput{Title: "Free classification", Stage: stage, Type: "Tipo personalizado"})
		if err != nil || created.Stage != stage || created.Type != "Tipo personalizado" {
			t.Fatalf("free classification: %+v %v", created, err)
		}
	}
	if err := db.Exec(`DELETE FROM document_substages; DELETE FROM document_stages; DELETE FROM document_types`).Error; err != nil {
		t.Fatal(err)
	}
	independent, err := documents.ListByExpediente(ctx, 1)
	if err != nil || len(independent) != 4 {
		t.Fatalf("documents must not depend on dictionary: %+v %v", independent, err)
	}
	a, err = participants.ListDenunciantesByExpediente(ctx, 1)
	if err != nil || len(a) != 2 {
		t.Fatalf("all expediente participants must remain visible: %v %v", a, err)
	}
	for i := 0; i < 2; i++ {
		apply("seeds/demo_data.sql")
	}
	if err := NewDatabaseClearService(db, true).Clear(ctx); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.Expediente{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("clear: count=%d err=%v", count, err)
	}
}
