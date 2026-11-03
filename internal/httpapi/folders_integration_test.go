package httpapi

import (
	"context"
	platform "etica_back/internal/platform/postgres"
	"etica_back/internal/services"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func verifyFolderMigration(t *testing.T, ctx context.Context, db *gorm.DB) {
	t.Helper()
	var e int64
	if err := db.Raw("SELECT id FROM expedientes ORDER BY id LIMIT 1").Scan(&e).Error; err != nil {
		t.Fatal(err)
	}
	var root, child, doc int64
	if err := db.Raw("INSERT INTO expediente_section(expediente_id,name,type,sort_order) VALUES (?, 'Legacy root','OTROS',0) RETURNING id", e).Scan(&root).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("INSERT INTO expediente_section(expediente_id,expediente_section_parent_id,name,type,sort_order) VALUES (?,?,'Legacy child','OTROS',0) RETURNING id", e, root).Scan(&child).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("INSERT INTO documents(name) VALUES ('Legacy document') RETURNING id").Scan(&doc).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO expediente_section_documents VALUES (?,?)", child, doc).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := platform.ApplyUpgradeMigration(ctx, db, filepath.Join("..", "..", "migrations", "004_expediente_folders.up.sql")); err != nil {
			t.Fatal(err)
		}
	}
	service := services.NewDocumentService(db)
	folders, err := service.ListFolders(ctx, e)
	if err != nil || len(folders) != 1 || folders[0].Name != "EXPEDIENTE PRINCIPAL" {
		t.Fatalf("principal migration: %+v %v", folders, err)
	}
	sections, err := service.ListFolderSections(ctx, e, folders[0].ID)
	if err != nil || len(sections) != 2 {
		t.Fatalf("legacy hierarchy lost: %+v %v", sections, err)
	}
	documents, err := service.ListSectionDocuments(ctx, e, child)
	if err != nil || len(documents) != 1 || documents[0].ID != doc {
		t.Fatalf("legacy document lost: %+v %v", documents, err)
	}
	types, err := service.ListFolderTypes(ctx)
	if err != nil || len(types) != 2 {
		t.Fatalf("catalog: %+v %v", types, err)
	}
	// Use an open expediente even if the retained migration fixture was closed.
	if err := db.Exec("UPDATE expedientes SET status='ABIERTO' WHERE id=?", e).Error; err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateFolder(ctx, e, services.CreateFolderInput{FolderTypeID: types[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.CreateFolder(ctx, e, services.CreateFolderInput{FolderTypeID: types[1].ID})
	if err != nil || again.ID != other.ID {
		t.Fatalf("duplicate folder: %+v %v", again, err)
	}
	if _, err := service.CreateSection(ctx, e, services.CreateSectionInput{Name: "Wrong parent", Type: "OTROS", ParentID: &root, FolderID: &other.ID}); err != services.ErrSectionInvalid {
		t.Fatalf("cross folder parent accepted: %v", err)
	}
	created, err := service.CreateSection(ctx, e, services.CreateSectionInput{Name: "Cautelar", Type: "OTROS", FolderID: &other.ID})
	if err != nil || created.FolderID != other.ID {
		t.Fatalf("folder section: %+v %v", created, err)
	}
	descendant, err := service.CreateSection(ctx, e, services.CreateSectionInput{Name: "Child", Type: "OTROS", ParentID: &created.ID})
	if err != nil || descendant.FolderID != other.ID {
		t.Fatalf("folder inheritance: %+v %v", descendant, err)
	}
	if err := db.Exec("DELETE FROM expediente_section_documents; DELETE FROM expediente_section WHERE expediente_section_parent_id IS NOT NULL; DELETE FROM expediente_section; DELETE FROM documents").Error; err != nil {
		t.Fatal(err)
	}
}
