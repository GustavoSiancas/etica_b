package services

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Explicit public projection: audit snapshots never leave the internal service.
type ExpedienteHistoryEntry struct {
	ID           int64      `json:"id"`
	ExpedienteID int64      `json:"expediente_id"`
	FolderID     *int64     `json:"folder_id"`
	Location     *string    `json:"location"`
	FolderName   *string    `json:"folder_name"`
	Event        string     `json:"event"`
	Status       string     `json:"status"`
	OpeningDate  *time.Time `json:"opening_date"`
	ClosingDate  *time.Time `json:"closing_date"`
	ChangedAt    time.Time  `json:"changed_at"`
}

type ExpedienteHistoryPage struct {
	Data   []ExpedienteHistoryEntry `json:"data"`
	Total  int64                    `json:"total"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
}

// The caller holds the expediente lock and owns the surrounding transaction.
func recordExpedienteHistory(tx *gorm.DB, expedienteID int64, folderID *int64, event string, now time.Time) error {
	source := `SELECT e.id, NULL::bigint, NULL::text, ?, e.status,
 e.opening_date::timestamp AT TIME ZONE 'America/Lima',
 e.closing_date::timestamp AT TIME ZONE 'America/Lima', ?,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',f.id,'name',t.name) ORDER BY f.id)
 FROM expediente_folders f JOIN folder_types t ON t.id=f.folder_type_id
 WHERE f.expediente_id=e.id AND f.status='ABIERTO'), '[]'::jsonb), NULL::varchar, NULL::jsonb
 FROM expedientes e WHERE e.id=?`
	args := []any{event, now, expedienteID}
	if folderID != nil {
		source = `SELECT f.expediente_id, f.id, t.name, ?, f.status, f.opening_date, f.closing_date, ?,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',a.id,'name',k.name) ORDER BY a.id)
 FROM expediente_folders a JOIN folder_types k ON k.id=a.folder_type_id
 WHERE a.expediente_id=f.expediente_id AND a.status='ABIERTO'), '[]'::jsonb), f.location,
 jsonb_build_object('schema_version',1,'sections',COALESCE((
 SELECT jsonb_agg(jsonb_build_object(
 'id',s.id,'name',s.name,'parent_id',s.expediente_section_parent_id,
 'type',s.type,'document_date',s.document_date,'sort_order',s.sort_order,
 'color_hex',s.color_hex,'created_at',s.created_at,'updated_at',s.updated_at,
 'documents',COALESCE((SELECT jsonb_agg(jsonb_build_object(
 'id',d.id,'name',d.name,'document_date',d.document_date,
 'created_at',d.created_at,'updated_at',d.updated_at) ORDER BY d.id)
 FROM expediente_section_documents sd JOIN documents d ON d.id=sd.document_id
 WHERE sd.expediente_section_id=s.id),'[]'::jsonb)
 ) ORDER BY s.sort_order,s.id)
 FROM expediente_section s WHERE s.expediente_id=f.expediente_id AND s.expediente_folder_id=f.id
 ),'[]'::jsonb))
 FROM expediente_folders f JOIN folder_types t ON t.id=f.folder_type_id
 WHERE f.expediente_id=? AND f.id=?`
		args = append(args, *folderID)
	}
	return tx.Exec(`INSERT INTO expediente_history
 (expediente_id,folder_id,folder_name,event,status,opening_date,closing_date,changed_at,active_folders,location,content_snapshot) `+source, args...).Error
}

func (s *ExpedienteService) ListHistory(ctx context.Context, id int64, limit, offset int) (ExpedienteHistoryPage, error) {
	return s.listHistory(ctx, id, nil, limit, offset)
}

func (s *ExpedienteService) ListFolderHistory(ctx context.Context, id, folderID int64, limit, offset int) (ExpedienteHistoryPage, error) {
	if folderID <= 0 {
		return ExpedienteHistoryPage{}, ErrExpedienteInvalid
	}
	return s.listHistory(ctx, id, &folderID, limit, offset)
}

func (s *ExpedienteService) listHistory(ctx context.Context, id int64, folderID *int64, limit, offset int) (ExpedienteHistoryPage, error) {
	page := ExpedienteHistoryPage{Data: []ExpedienteHistoryEntry{}, Limit: limit, Offset: offset}
	if id <= 0 || limit < 1 || limit > 100 || offset < 0 {
		return page, ErrExpedienteInvalid
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, id, false); err != nil {
			return err
		}
		if folderID != nil {
			var count int64
			if err := tx.Table("expediente_folders").Where("expediente_id = ? AND id = ?", id, *folderID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrFolderNotFound
			}
		}
		query := tx.Table("expediente_history").Where("expediente_id = ?", id)
		if folderID != nil {
			query = query.Where("folder_id = ?", *folderID)
		}
		if err := query.Count(&page.Total).Error; err != nil {
			return err
		}
		return query.Select("id, expediente_id, folder_id, folder_name, event, status, location, opening_date, closing_date, changed_at").Order("changed_at DESC, id DESC").Limit(limit).Offset(offset).Scan(&page.Data).Error
	})
	return page, err
}
