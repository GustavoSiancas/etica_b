package services

import (
	"context"
	"errors"
	"etica_back/internal/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type FolderTypeResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type FolderResponse struct {
	ID           int64     `json:"id"`
	ExpedienteID int64     `json:"expediente_id"`
	FolderTypeID int64     `json:"folder_type_id"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type CreateFolderInput struct {
	FolderTypeID int64 `json:"folder_type_id"`
}

func (s *DocumentService) ListFolderTypes(ctx context.Context) ([]FolderTypeResponse, error) {
	rows := []FolderTypeResponse{}
	err := s.db.WithContext(ctx).Model(&models.FolderType{}).Order("id").Find(&rows).Error
	return rows, err
}
func (s *DocumentService) ListFolders(ctx context.Context, expedienteID int64) ([]FolderResponse, error) {
	if expedienteID <= 0 {
		return nil, ErrSectionInvalid
	}
	db := s.db.WithContext(ctx)
	if err := requireExpediente(db, expedienteID, false); err != nil {
		return nil, err
	}
	rows := []FolderResponse{}
	err := db.Table("expediente_folders f").Select("f.*, t.name").Joins("JOIN folder_types t ON t.id=f.folder_type_id").Where("f.expediente_id = ?", expedienteID).Order("f.id").Scan(&rows).Error
	for i := range rows {
		rows[i].CreatedAt = rows[i].CreatedAt.UTC()
		rows[i].UpdatedAt = rows[i].UpdatedAt.UTC()
	}
	return rows, err
}

// Adding an existing type returns its folder; each expediente has one folder per type.
func (s *DocumentService) CreateFolder(ctx context.Context, expedienteID int64, input CreateFolderInput) (FolderResponse, error) {
	if expedienteID <= 0 || input.FolderTypeID <= 0 {
		return FolderResponse{}, ErrSectionInvalid
	}
	var row models.ExpedienteFolder
	var kind models.FolderType
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		if err := tx.First(&kind, input.FolderTypeID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSectionInvalid
			}
			return err
		}
		row = models.ExpedienteFolder{ExpedienteID: expedienteID, FolderTypeID: input.FolderTypeID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Where("expediente_id = ? AND folder_type_id = ?", expedienteID, input.FolderTypeID).First(&row).Error
	})
	return FolderResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, FolderTypeID: row.FolderTypeID, Name: kind.Name, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}, sectionDBError(err)
}
func (s *DocumentService) ListFolderSections(ctx context.Context, expedienteID, folderID int64) ([]SectionResponse, error) {
	if expedienteID <= 0 || folderID <= 0 {
		return nil, ErrSectionInvalid
	}
	db := s.db.WithContext(ctx)
	if err := requireExpediente(db, expedienteID, false); err != nil {
		return nil, err
	}
	var folder models.ExpedienteFolder
	if err := db.Where("expediente_id = ?", expedienteID).First(&folder, folderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSectionNotFound
		}
		return nil, err
	}
	var rows []models.ExpedienteSection
	err := db.Where("expediente_id = ? AND expediente_folder_id = ?", expedienteID, folderID).Order("expediente_section_parent_id ASC NULLS FIRST, sort_order, id").Find(&rows).Error
	result := make([]SectionResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, sectionResponse(row))
	}
	return result, err
}
