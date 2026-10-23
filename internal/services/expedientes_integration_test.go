package services

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"etica_back/internal/config"
	platform "etica_back/internal/platform/postgres"
	"github.com/jackc/pgx/v5"
)

// Opt in against local PostgreSQL; all data stays in an isolated disposable schema.
func TestAutomaticCodesPostgres(t *testing.T) {
	if os.Getenv("ETICA_INTEGRATION") != "1" {
		t.Skip("set ETICA_INTEGRATION=1 to test PostgreSQL")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(filepath.Join(cwd, "../..")); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("configuration unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, pool, err := platform.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer pool.Close()
	schema := fmt.Sprintf("etica_code_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if err = admin.Exec("CREATE SCHEMA " + quoted).Error; err != nil {
		t.Fatal("cannot create isolated test schema")
	}
	defer func() {
		if err := admin.Exec("DROP SCHEMA " + quoted + " CASCADE").Error; err != nil {
			t.Error("cannot clean test schema")
		}
	}()
	u, _ := url.Parse(cfg.DatabaseURL)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, testPool, err := platform.Open(ctx, u.String())
	if err != nil {
		t.Fatal(platform.Diagnose(err))
	}
	defer testPool.Close()
	// Reproduce an older installation that left the enum and code functions.
	if err = db.Exec(`CREATE TYPE expediente_init_type_enum AS ENUM (
		'DENUNCIA_DE_OFICIO', 'DENUNCIA_DE_PARTE',
		'COMUNICACIONES_DE_ENTIDADES_PUBLICAS',
		'COMUNICACIONES_DE_SANCION_IMPUESTA_POR_PJ_O_MP',
		'APROBADO_POR_ASAMBLEA_GENERAL'
	)`).Error; err != nil {
		t.Fatal("cannot create existing enum fixture")
	}
	if err = db.Exec(`
		CREATE FUNCTION next_expediente_code(p_year INTEGER) RETURNS VARCHAR(50)
		LANGUAGE plpgsql AS $$ BEGIN RETURN 'obsolete'; END; $$;
		CREATE FUNCTION assign_expediente_code() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$;
	`).Error; err != nil {
		t.Fatal("cannot create existing function fixtures")
	}
	migration, err := os.ReadFile("migrations/001_initial.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(string(migration)).Error; err != nil {
		t.Fatal("initial migration failed")
	}

	var year int
	if err = db.Raw("SELECT EXTRACT(YEAR FROM statement_timestamp() AT TIME ZONE 'America/Lima')::integer").Scan(&year).Error; err != nil {
		t.Fatal("cannot read Lima year")
	}
	service := NewExpedienteService(db)
	first, err := service.Create(ctx, CreateExpedienteInput{Description: "Expediente de prueba", OpeningDate: &Date{Time: time.Now()}, InitType: "DENUNCIA_DE_OFICIO"})
	if err != nil {
		t.Fatal("create failed")
	}
	expectedFirst := fmt.Sprintf("1-%d", year)
	if first.Code != expectedFirst || first.Number != first.Code || first.CreatedAt.IsZero() {
		t.Fatalf("incorrect generated response: %+v", first)
	}
	const count = 12
	results := make(chan ExpedienteResponse, count)
	errs := make(chan error, count)
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Create(ctx, CreateExpedienteInput{Description: "Expediente de prueba", OpeningDate: &Date{Time: time.Now()}, InitType: "DENUNCIA_DE_OFICIO"})
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	close(errs)
	if len(errs) > 0 {
		t.Fatal("concurrent creation failed")
	}
	codes := map[string]bool{first.Code: true}
	for result := range results {
		if codes[result.Code] {
			t.Fatal("duplicate code")
		}
		codes[result.Code] = true
	}
	for _, test := range []struct {
		year int
		want string
	}{{year + 1, fmt.Sprintf("1-%d", year+1)}, {year + 1, fmt.Sprintf("2-%d", year+1)}, {year + 2, fmt.Sprintf("1-%d", year+2)}} {
		var got string
		if err = db.Raw("SELECT next_expediente_code(?)", test.year).Scan(&got).Error; err != nil || got != test.want {
			t.Fatalf("annual sequence: got %s want %s", got, test.want)
		}
	}
	tx := db.Begin()
	var rolledBack string
	if err = tx.Raw("SELECT next_expediente_code(?)", year+3).Scan(&rolledBack).Error; err != nil {
		tx.Rollback()
		t.Fatal("counter allocation failed")
	}
	tx.Rollback()
	var retried string
	if err = db.Raw("SELECT next_expediente_code(?)", year+3).Scan(&retried).Error; err != nil || retried != rolledBack {
		t.Fatal("counter did not roll back")
	}
	list, err := service.List(ctx, 100, 0)
	if err != nil || list.Total != count+1 {
		t.Fatal("list failed")
	}
	if err = db.Exec("UPDATE expedientes SET code=NULL WHERE id=?", first.ID).Error; err == nil {
		t.Fatal("accepted null code")
	}
	if err = db.Exec("UPDATE expedientes SET code=? WHERE id <> ?", first.Code, first.ID).Error; err == nil {
		t.Fatal("accepted duplicate code")
	}
	down, err := os.ReadFile("migrations/001_initial.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(strings.TrimSpace(string(down))).Error; err != nil {
		t.Fatal("down migration failed")
	}
}
