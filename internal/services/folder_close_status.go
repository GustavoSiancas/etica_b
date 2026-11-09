package services

import "context"

type FolderCloseStatusResponse struct {
	ExpedienteID        int64  `json:"expediente_id"`
	FolderID            int64  `json:"folder_id"`
	FolderStatus        string `json:"folder_status"`
	ExpedienteStatus    string `json:"expediente_status"`
	WillCloseExpediente bool   `json:"will_close_expediente"`
}

// One read-only statement evaluates both statuses and sibling folders at the same snapshot.
func (s *DocumentService) GetFolderCloseStatus(ctx context.Context, expedienteID, folderID int64) (FolderCloseStatusResponse, error) {
	result := FolderCloseStatusResponse{}
	if expedienteID <= 0 || folderID <= 0 {
		return result, ErrFolderInvalid
	}
	var row struct {
		FolderCloseStatusResponse
		FoundFolderID *int64
	}
	query := s.db.WithContext(ctx).Raw(`SELECT e.id AS expediente_id, f.id AS found_folder_id,
 COALESCE(f.id,0) AS folder_id, COALESCE(f.status,'') AS folder_status, e.status AS expediente_status,
 (e.status='ABIERTO' AND COALESCE(f.status='ABIERTO',false) AND NOT EXISTS (
 SELECT 1 FROM expediente_folders other
 WHERE other.expediente_id=e.id AND other.id<>? AND other.status='ABIERTO'
 )) AS will_close_expediente
 FROM expedientes e LEFT JOIN expediente_folders f ON f.expediente_id=e.id AND f.id=?
 WHERE e.id=?`, folderID, folderID, expedienteID).Scan(&row)
	if query.Error != nil {
		return result, query.Error
	}
	if query.RowsAffected == 0 {
		return result, ErrExpedienteNotFound
	}
	if row.FoundFolderID == nil {
		return result, ErrFolderNotFound
	}
	return row.FolderCloseStatusResponse, nil
}
