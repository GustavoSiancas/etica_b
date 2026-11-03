package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	platform "etica_back/internal/platform/postgres"
	"etica_back/internal/services"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
)

// Explicit test URL only; every run owns an isolated disposable schema.
func TestSectionDocumentFlowPostgres(t *testing.T) {
	dsn := os.Getenv("ETICA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ETICA_TEST_DATABASE_URL for PostgreSQL HTTP integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, pool, err := platform.Open(ctx, dsn)
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer pool.Close()
	schema := fmt.Sprintf("etica_sections_http_%d", time.Now().UnixNano())
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
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_initial.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	// Reproduce the installation from the reported log: sections exist but
	// expedientes still has legacy statuses and no location column.
	if err := db.Exec(`ALTER TABLE expedientes DROP CONSTRAINT expedientes_status_check;
        ALTER TABLE expedientes DROP COLUMN location;
        ALTER TABLE expedientes ALTER COLUMN status SET DEFAULT 'OPEN';
        INSERT INTO expedientes(description,status,opening_date) VALUES ('Legacy A','OPEN','2026-01-01'),('Legacy B','IN_PROGRESS','2026-01-01'),('Legacy C','CLOSED','2026-01-01');`).Error; err != nil {
		t.Fatal(err)
	}
	upgradePath := filepath.Join("..", "..", "migrations", "002_expediente_state_location.up.sql")
	for i := 0; i < 2; i++ {
		if err := platform.ApplyUpgradeMigration(ctx, db, upgradePath); err != nil {
			t.Fatal(err)
		}
	}
	var upgraded []struct {
		Status   string
		Location string
		Number   string
	}
	if err := db.Table("expedientes").Order("id").Find(&upgraded).Error; err != nil {
		t.Fatal(err)
	}
	if len(upgraded) != 3 {
		t.Fatal("migration changed record count")
	}
	for i, status := range []string{"ABIERTO", "ABIERTO", "CERRADO"} {
		if upgraded[i].Status != status || upgraded[i].Location != services.UbicacionDireccion || upgraded[i].Number == "" {
			t.Fatalf("legacy upgrade failed: %+v", upgraded[i])
		}
	}
	// Upgrade the old trigger without renumbering previously registered cases.
	if err := db.Exec(`UPDATE expedientes SET number='9-2020', opening_date='2020-12-31', created_at='2035-01-01' WHERE description='Legacy A';
        CREATE OR REPLACE FUNCTION assign_expediente_code() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN NEW.number := next_expediente_code(EXTRACT(YEAR FROM statement_timestamp())::INTEGER); RETURN NEW; END; $$;`).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := platform.ApplyUpgradeMigration(ctx, db, filepath.Join("..", "..", "migrations", "003_expediente_opening_year.up.sql")); err != nil {
			t.Fatal(err)
		}
	}
	var retained struct {
		Number    string
		CreatedAt time.Time
	}
	if err := db.Table("expedientes").Where("description='Legacy A'").Take(&retained).Error; err != nil || retained.Number != "9-2020" || retained.CreatedAt.Year() != 2035 {
		t.Fatal("003 modified existing number or creation date")
	}
	for _, test := range []struct{ date, number string }{{"2020-12-31", "10-2020"}, {"2030-01-01", "1-2030"}, {"2020-01-01", "11-2020"}} {
		parsed, _ := time.Parse("2006-01-02", test.date)
		row, err := services.NewExpedienteService(db).Create(ctx, services.CreateExpedienteInput{OpeningDate: &services.Date{Time: parsed}, InitType: "DENUNCIA_DE_OFICIO"})
		if err != nil || row.Number != test.number {
			t.Fatalf("opening year %s: %s %v", test.date, row.Number, err)
		}
	}
	if err := db.Exec("INSERT INTO expedientes(description) VALUES ('Missing opening date')").Error; err == nil {
		t.Fatal("new expediente without opening date accepted")
	}
	verifyFolderMigration(t, ctx, db)
	if err := db.Exec("DELETE FROM expediente_folders; DELETE FROM expedientes; DELETE FROM expediente_year_counters").Error; err != nil {
		t.Fatal(err)
	}
	documents := services.NewDocumentService(db)
	// Exercise signing/confirmation against a local S3 mock, with no external calls.
	var storedBody []byte
	var storedType string
	var onHead func()
	var storageMu sync.Mutex
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageMu.Lock()
		defer storageMu.Unlock()
		switch r.Method {
		case "PUT":
			storedBody, _ = io.ReadAll(r.Body)
			storedType = r.Header.Get("Content-Type")
			w.WriteHeader(200)
		case "HEAD":
			if onHead != nil {
				callback := onHead
				onHead = nil
				callback()
			}
			if storedBody == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", storedType)
			w.Header().Set("Content-Length", strconv.Itoa(len(storedBody)))
			w.WriteHeader(200)
		default:
			w.WriteHeader(405)
		}
	}))
	defer storage.Close()
	client := s3.NewFromConfig(aws.Config{Region: "auto", Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}, func(o *s3.Options) { o.BaseEndpoint = aws.String(storage.URL); o.UsePathStyle = true })
	upload, err := services.NewUploadService(db, client, "test-bucket", "https://files.example.com")
	if err != nil {
		t.Fatal(err)
	}
	handler := Routes(upload, services.NewExpedienteService(db), documents, services.NewParticipantService(db))
	request := func(method, path, body string, status int, target any) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		if target != nil {
			if err := json.Unmarshal(w.Body.Bytes(), target); err != nil {
				t.Fatal(err)
			}
		}
		if status == 204 && w.Body.Len() != 0 {
			t.Fatal("204 must have no body")
		}
	}
	var e services.ExpedienteResponse
	request("POST", "/expedientes", `{"opening_date":"2026-10-30","init_type":"DENUNCIA_DE_PARTE"}`, 201, &e)
	if e.Status != services.ExpedienteAbierto || e.Location != services.UbicacionDireccion {
		t.Fatalf("unexpected defaults: %+v", e)
	}
	base := fmt.Sprintf("/expedientes/%d/sections", e.ID)
	var root, child services.SectionResponse
	request("POST", base, `{"name":"Denuncia","type":"OTROS"}`, 201, &root)
	request("POST", base, fmt.Sprintf(`{"name":"Anexos","type":"Custom","expediente_section_parent_id":%d,"document_date":"2026-10-30","color_hex":"#aabbcc"}`, root.ID), 201, &child)
	rootPath := fmt.Sprintf("%s/%d", base, root.ID)
	childPath := fmt.Sprintf("%s/%d", base, child.ID)
	if child.ColorHex != "#AABBCC" || child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatal("section fields not persisted")
	}
	var list []services.SectionResponse
	request("GET", base, "", 200, &list)
	if len(list) != 2 {
		t.Fatal("missing sections")
	}
	request("PATCH", rootPath, fmt.Sprintf(`{"expediente_section_parent_id":%d}`, child.ID), 400, nil)
	request("DELETE", rootPath, "", 409, nil)
	request("PATCH", childPath, `{"color_hex":"red"}`, 400, nil)
	request("PATCH", childPath, `{"document_date":"2026-02-30"}`, 400, nil)
	request("PATCH", childPath, `{"unexpected":1}`, 400, nil)
	request("PATCH", childPath, `{}`, 400, nil)
	request("GET", "/expedientes/0/sections", "", 400, nil)
	request("GET", "/expedientes/999999/sections", "", 404, nil)
	request("GET", base+"/999999", "", 404, nil)
	var other services.ExpedienteResponse
	request("POST", "/expedientes", `{"opening_date":"2026-10-30","init_type":"DENUNCIA_DE_OFICIO"}`, 201, &other)
	otherBase := fmt.Sprintf("/expedientes/%d/sections", other.ID)
	request("POST", otherBase, fmt.Sprintf(`{"name":"Invalid","type":"OTROS","expediente_section_parent_id":%d}`, root.ID), 400, nil)
	request("GET", fmt.Sprintf("%s/%d", otherBase, child.ID), "", 404, nil)
	var doc services.DocumentResponse
	request("POST", childPath+"/documents", `{"name":"Evidence","file_url":"https://example.com/evidence.pdf"}`, 201, &doc)
	var signature services.UploadSignature
	request("POST", fmt.Sprintf("/documents/%d/upload-url", doc.ID), `{"content_type":"application/pdf"}`, 200, &signature)
	put, err := http.NewRequest("PUT", signature.UploadURL, bytes.NewBufferString("%PDF-1.7 local fixture"))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range signature.Headers {
		put.Header.Set(k, v)
	}
	uploaded, err := storage.Client().Do(put)
	if err != nil {
		t.Fatal(err)
	}
	uploaded.Body.Close()
	if uploaded.StatusCode != 200 {
		t.Fatal("mock upload failed")
	}
	filePayload, _ := json.Marshal(map[string]string{"file_url": signature.FileURL})
	request("PATCH", fmt.Sprintf("/documents/%d/file-url", doc.ID), string(filePayload), 200, nil)
	request("GET", fmt.Sprintf("/documents/%d", doc.ID), "", 200, &doc)
	if doc.FileURL == nil || *doc.FileURL != signature.FileURL {
		t.Fatal("verified PDF URL not persisted")
	}
	storageMu.Lock()
	storedType = "text/html"
	storageMu.Unlock()
	request("PATCH", fmt.Sprintf("/documents/%d/file-url", doc.ID), string(filePayload), 400, nil)
	request("POST", base+"/999999/documents", `{"name":"Must roll back"}`, 404, nil)
	var page services.DocumentPage
	request("GET", "/documents?limit=10&offset=0", "", 200, &page)
	if page.Total != 1 {
		t.Fatal("failed scoped create left orphan document")
	}
	request("GET", "/documents?limit=101", "", 400, nil)
	docPath := fmt.Sprintf("/documents/%d", doc.ID)
	linkPath := fmt.Sprintf("%s/documents/%d", rootPath, doc.ID)
	request("POST", linkPath, "", 204, nil)
	request("POST", linkPath, "", 204, nil)
	var docs []services.DocumentResponse
	request("GET", fmt.Sprintf("/expedientes/%d/documents", e.ID), "", 200, &docs)
	if len(docs) != 1 {
		t.Fatal("same document appeared twice in expediente")
	}
	request("DELETE", docPath, "", 409, nil)
	request("PATCH", docPath, `{"name":"Renamed","file_url":null}`, 200, &doc)
	if doc.Name != "Renamed" || doc.FileURL != nil {
		t.Fatal("document patch failed")
	}
	request("GET", childPath+"/documents", "", 200, &docs)
	if len(docs) != 1 || docs[0].Name != "Renamed" {
		t.Fatal("shared document update not visible")
	}
	request("PATCH", childPath, `{"expediente_section_parent_id":null,"document_date":null,"sort_order":0}`, 200, &child)
	if child.ParentID != nil || child.DocumentDate != nil || child.SortOrder != 0 {
		t.Fatal("explicit null or zero lost")
	}
	request("DELETE", rootPath, "", 204, nil)
	request("GET", docPath, "", 200, &doc)
	request("DELETE", childPath+fmt.Sprintf("/documents/%d", doc.ID), "", 204, nil)
	request("DELETE", childPath+fmt.Sprintf("/documents/%d", doc.ID), "", 404, nil)
	request("DELETE", docPath, "", 204, nil)
	request("GET", docPath, "", 404, nil)
	request("DELETE", childPath, "", 204, nil)
	for _, path := range []string{"/escritos", "/escritos/1/upload-url", "/document-stages"} {
		request("GET", path, "", 404, nil)
	}
	request("POST", fmt.Sprintf("/expedientes/%d/documents", e.ID), `{"name":"Old path"}`, 405, nil)
	request("POST", "/documents", `{"title":"Old payload","stage":"OTROS","type":"OTROS"}`, 400, nil)
	request("GET", "/document-types", "", 200, nil)

	// Independently created documents can be shared across expedientes.
	request("POST", "/documents", `{"name":"Shared document"}`, 201, &doc)
	request("POST", base, `{"name":"Shared A","type":"OTROS"}`, 201, &root)
	request("POST", otherBase, `{"name":"Shared B","type":"OTROS"}`, 201, &child)
	sharedA := fmt.Sprintf("%s/%d/documents/%d", base, root.ID, doc.ID)
	sharedB := fmt.Sprintf("%s/%d/documents/%d", otherBase, child.ID, doc.ID)
	request("POST", sharedA, "", 204, nil)
	request("POST", sharedB, "", 204, nil)
	request("GET", fmt.Sprintf("/expedientes/%d/documents", other.ID), "", 200, &docs)
	if len(docs) != 1 || docs[0].ID != doc.ID {
		t.Fatal("shared document missing from second expediente")
	}
	request("DELETE", sharedA, "", 204, nil)
	request("DELETE", fmt.Sprintf("/documents/%d", doc.ID), "", 409, nil)
	request("DELETE", sharedB, "", 204, nil)
	request("DELETE", fmt.Sprintf("/documents/%d", doc.ID), "", 204, nil)

	// Concurrent opposing reparent operations must not produce a cycle.
	a, err := documents.CreateSection(ctx, e.ID, services.CreateSectionInput{Name: "A", Type: "OTROS"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := documents.CreateSection(ctx, e.ID, services.CreateSectionInput{Name: "B", Type: "OTROS"})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := make(chan error, 2)
	var wg sync.WaitGroup
	for _, pair := range [][2]int64{{a.ID, b.ID}, {b.ID, a.ID}} {
		wg.Add(1)
		go func(pair [2]int64) {
			defer wg.Done()
			parent := pair[1]
			_, err := documents.UpdateSection(ctx, e.ID, pair[0], services.UpdateSectionInput{ParentID: services.Optional[int64]{Set: true, Value: &parent}})
			outcomes <- err
		}(pair)
	}
	wg.Wait()
	close(outcomes)
	ok, invalid := 0, 0
	for err := range outcomes {
		if err == nil {
			ok++
		} else if errors.Is(err, services.ErrSectionInvalid) {
			invalid++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || invalid != 1 {
		t.Fatalf("concurrent hierarchy: success=%d rejected=%d", ok, invalid)
	}
	// Deep hierarchy stays flat in the API, avoiding recursive JSON depth limits.
	parent := b.ID
	for i := 0; i < 100; i++ {
		section, err := documents.CreateSection(ctx, e.ID, services.CreateSectionInput{Name: fmt.Sprintf("Level %d", i), Type: "OTROS", ParentID: &parent})
		if err != nil {
			t.Fatal(err)
		}
		parent = section.ID
	}
	// Invalid direct SQL is rejected too.
	if err := db.Exec("UPDATE expediente_section SET expediente_section_parent_id = ? WHERE id = ?", parent, b.ID).Error; err == nil {
		t.Fatal("database accepted indirect cycle")
	}
	// Closed expedientes remain readable but all information writes are denied.
	var frozenSection services.SectionResponse
	request("POST", base, `{"name":"Frozen","type":"OTROS"}`, 201, &frozenSection)
	frozenPath := fmt.Sprintf("%s/%d", base, frozenSection.ID)
	request("POST", frozenPath+"/documents", `{"name":"Frozen document"}`, 201, &doc)
	frozenDocPath := fmt.Sprintf("/documents/%d", doc.ID)
	request("POST", frozenDocPath+"/upload-url", `{"content_type":"application/pdf"}`, 200, &signature)
	pendingPayload, _ := json.Marshal(map[string]string{"file_url": signature.FileURL})
	request("POST", fmt.Sprintf("%s/%d/documents/%d", otherBase, child.ID, doc.ID), "", 204, nil)
	locationPath := fmt.Sprintf("/expedientes/%d/location", e.ID)
	statusPath := fmt.Sprintf("/expedientes/%d/status", e.ID)
	request("GET", "/expediente-statuses", "", 200, nil)
	request("GET", "/expediente-locations", "", 200, nil)
	for _, location := range []string{services.UbicacionConsejo, services.UbicacionTribunal, services.UbicacionDireccion} {
		payload, _ := json.Marshal(map[string]string{"location": location})
		request("PATCH", locationPath, string(payload), 200, &e)
		if e.Location != location {
			t.Fatal("location update lost")
		}
	}
	request("PATCH", locationPath, `{"location":"OTHER"}`, 400, nil)
	for _, status := range []string{"OPEN", "CLOSED", "IN_PROGRESS", "", "OTHER"} {
		payload, _ := json.Marshal(map[string]string{"status": status})
		request("PATCH", statusPath, string(payload), 400, nil)
	}
	request("PATCH", statusPath, `{"status":"CERRADO"}`, 200, &e)
	if e.Status != services.ExpedienteCerrado || e.Location != services.UbicacionDireccion {
		t.Fatal("status and location must be independent")
	}
	var expedientePage services.ExpedienteList
	request("GET", "/expedientes?status=CERRADO&location=DIRECCION%20DE%20ETICA&limit=1&offset=0", "", 200, &expedientePage)
	if expedientePage.Total != 1 || len(expedientePage.Data) != 1 || expedientePage.Data[0].ID != e.ID {
		t.Fatalf("filtered total/data: %+v", expedientePage)
	}
	request("GET", "/expedientes?status=CERRADO&location=DIRECCION%20DE%20ETICA&limit=1&offset=1", "", 200, &expedientePage)
	if expedientePage.Total != 1 || len(expedientePage.Data) != 0 {
		t.Fatal("pagination changed filtered total")
	}
	request("GET", "/expedientes?status=ABIERTO&limit=1", "", 200, &expedientePage)
	if expedientePage.Total != 1 || len(expedientePage.Data) != 1 || expedientePage.Data[0].ID != other.ID {
		t.Fatal("open filter failed")
	}
	request("GET", "/expedientes?location=CONSEJO%20DE%20ETICA", "", 200, &expedientePage)
	if expedientePage.Total != 0 || len(expedientePage.Data) != 0 {
		t.Fatal("empty location filter failed")
	}

	for _, test := range []struct{ method, path, body string }{
		{"POST", base, `{"name":"Blocked","type":"OTROS"}`},
		{"PATCH", frozenPath, `{"name":"Blocked"}`},
		{"DELETE", frozenPath, ""},
		{"POST", frozenPath + "/documents", `{"name":"Blocked"}`},
		{"POST", frozenPath + fmt.Sprintf("/documents/%d", doc.ID), ""},
		{"DELETE", frozenPath + fmt.Sprintf("/documents/%d", doc.ID), ""},
		{"PATCH", frozenDocPath, `{"name":"Blocked shared update"}`},
		{"DELETE", frozenDocPath, ""},
		{"POST", frozenDocPath + "/upload-url", `{"content_type":"application/pdf"}`},
		{"PATCH", frozenDocPath + "/file-url", string(pendingPayload)},
	} {
		request(test.method, test.path, test.body, 409, nil)
	}
	request("PATCH", locationPath, `{"location":"CONSEJO DE ETICA"}`, 200, &e)
	if e.Status != services.ExpedienteCerrado || e.Location != services.UbicacionConsejo {
		t.Fatal("closed location update must preserve CERRADO")
	}
	request("POST", base, `{"name":"Still blocked","type":"OTROS"}`, 409, nil)
	request("GET", base, "", 200, nil)
	request("GET", frozenPath, "", 200, nil)
	request("GET", frozenPath+"/documents", "", 200, &docs)
	if len(docs) != 1 || docs[0].Name != "Frozen document" {
		t.Fatal("closed content was modified")
	}
	request("GET", frozenDocPath, "", 200, nil)
	request("GET", fmt.Sprintf("/expedientes/%d/denunciantes", e.ID), "", 200, nil)
	participants := services.NewParticipantService(db)
	person, err := participants.CreateDenunciante(ctx, services.CreateDenuncianteInput{Name: "Frozen complainant", DocumentType: "DNI", DocumentNumber: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := participants.AttachDenunciante(ctx, e.ID, person.ID); !errors.Is(err, services.ErrExpedienteClosed) {
		t.Fatalf("closed participant mutation: %v", err)
	}
	request("PATCH", statusPath, `{"status":"ABIERTO"}`, 200, &e)
	request("PATCH", frozenPath, `{"name":"Reopened section"}`, 200, nil)
	request("PATCH", frozenDocPath, `{"name":"Reopened document"}`, 200, nil)
	// Close during storage verification: the final DB update must recheck state.
	storageMu.Lock()
	storedType = "application/pdf"
	onHead = func() {
		if _, err := services.NewExpedienteService(db).UpdateStatus(ctx, e.ID, services.UpdateExpedienteStatusInput{Status: services.ExpedienteCerrado}); err != nil {
			t.Error(err)
		}
	}
	storageMu.Unlock()
	request("PATCH", frozenDocPath+"/file-url", string(pendingPayload), 409, nil)
	request("GET", frozenDocPath, "", 200, &doc)
	if doc.FileURL != nil {
		t.Fatal("upload was confirmed after expediente closed")
	}
	request("PATCH", statusPath, `{"status":"ABIERTO"}`, 200, nil)
	request("PATCH", frozenDocPath+"/file-url", string(pendingPayload), 200, nil)

}
