package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"etica_back/internal/persistence/models"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSectionInvalid = errors.New("invalid section fields or hierarchy")
var ErrSectionNotFound = errors.New("section not found in expediente")
var ErrSectionHasChildren = errors.New("section has children")
var ErrSectionDocumentNotFound = errors.New("document link not found in section")
var sectionColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type CreateSectionInput struct {
	ParentID     *int64  `json:"expediente_section_parent_id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	DocumentDate *Date   `json:"document_date"`
	SortOrder    int     `json:"sort_order"`
	ColorHex     *string `json:"color_hex"`
}
type UpdateSectionInput struct {
	ParentID     Optional[int64] `json:"expediente_section_parent_id"`
	Name         *string         `json:"name"`
	Type         *string         `json:"type"`
	DocumentDate Optional[Date]  `json:"document_date"`
	SortOrder    *int            `json:"sort_order"`
	ColorHex     *string         `json:"color_hex"`
}
type SectionResponse struct {
	ID           int64     `json:"id"`
	ExpedienteID int64     `json:"expediente_id"`
	ParentID     *int64    `json:"expediente_section_parent_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	DocumentDate *Date     `json:"document_date"`
	SortOrder    int       `json:"sort_order"`
	ColorHex     string    `json:"color_hex"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func sectionResponse(row models.ExpedienteSection) SectionResponse {
	var date *Date
	if row.DocumentDate != nil {
		date = &Date{Time: *row.DocumentDate}
	}
	return SectionResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, ParentID: row.ExpedienteSectionParentID, Name: row.Name, Type: row.Type, DocumentDate: date, SortOrder: row.SortOrder, ColorHex: row.ColorHex, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
func sectionDBError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23514", "23503":
			return ErrSectionInvalid
		case "23001":
			return ErrSectionHasChildren
		}
	}
	return err
}
func getSection(tx *gorm.DB, expedienteID, sectionID int64) (models.ExpedienteSection, error) {
	var row models.ExpedienteSection
	err := tx.Where("expediente_id = ?", expedienteID).First(&row, sectionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrSectionNotFound
	}
	return row, err
}
func validParent(tx *gorm.DB, expedienteID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	if *parentID <= 0 {
		return ErrSectionInvalid
	}
	_, err := getSection(tx, expedienteID, *parentID)
	if errors.Is(err, ErrSectionNotFound) {
		return ErrSectionInvalid
	}
	return err
}
func validateSection(input CreateSectionInput) (models.ExpedienteSection, error) {
	color := "#808080"
	if input.ColorHex != nil {
		color = strings.TrimSpace(*input.ColorHex)
	}
	if !validDocumentText(input.Name) || !validDocumentText(input.Type) || input.SortOrder < 0 || !sectionColor.MatchString(color) || (input.ParentID != nil && *input.ParentID <= 0) {
		return models.ExpedienteSection{}, ErrSectionInvalid
	}
	var date *time.Time
	if input.DocumentDate != nil {
		date = &input.DocumentDate.Time
	}
	return models.ExpedienteSection{Name: strings.TrimSpace(input.Name), Type: strings.TrimSpace(input.Type), ExpedienteSectionParentID: input.ParentID, DocumentDate: date, SortOrder: input.SortOrder, ColorHex: strings.ToUpper(color)}, nil
}
func (s *DocumentService) CreateSection(ctx context.Context, expedienteID int64, input CreateSectionInput) (SectionResponse, error) {
	if expedienteID <= 0 {
		return SectionResponse{}, ErrSectionInvalid
	}
	row, err := validateSection(input)
	if err != nil {
		return SectionResponse{}, err
	}
	row.ExpedienteID = expedienteID
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		if err := validParent(tx, expedienteID, input.ParentID); err != nil {
			return err
		}
		return tx.Create(&row).Error
	})
	return sectionResponse(row), sectionDBError(err)
}
func (s *DocumentService) ListSections(ctx context.Context, expedienteID int64) ([]SectionResponse, error) {
	if expedienteID <= 0 {
		return nil, ErrSectionInvalid
	}
	db := s.db.WithContext(ctx)
	if err := requireExpediente(db, expedienteID, false); err != nil {
		return nil, err
	}
	var rows []models.ExpedienteSection
	err := db.Where("expediente_id = ?", expedienteID).Order("expediente_section_parent_id ASC NULLS FIRST, sort_order ASC, id ASC").Find(&rows).Error
	result := make([]SectionResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, sectionResponse(row))
	}
	return result, err
}
func (s *DocumentService) GetSection(ctx context.Context, expedienteID, id int64) (SectionResponse, error) {
	if expedienteID <= 0 || id <= 0 {
		return SectionResponse{}, ErrSectionInvalid
	}
	db := s.db.WithContext(ctx)
	if err := requireExpediente(db, expedienteID, false); err != nil {
		return SectionResponse{}, err
	}
	row, err := getSection(db, expedienteID, id)
	return sectionResponse(row), err
}
func (s *DocumentService) UpdateSection(ctx context.Context, expedienteID, id int64, input UpdateSectionInput) (SectionResponse, error) {
	if expedienteID <= 0 || id <= 0 || (input.Name == nil && input.Type == nil && !input.ParentID.Set && !input.DocumentDate.Set && input.SortOrder == nil && input.ColorHex == nil) {
		return SectionResponse{}, ErrSectionInvalid
	}
	var result models.ExpedienteSection
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		row, err := getSection(tx, expedienteID, id)
		if err != nil {
			return err
		}
		current := sectionResponse(row)
		values := CreateSectionInput{ParentID: current.ParentID, Name: current.Name, Type: current.Type, DocumentDate: current.DocumentDate, SortOrder: current.SortOrder, ColorHex: &current.ColorHex}
		if input.Name != nil {
			values.Name = *input.Name
		}
		if input.Type != nil {
			values.Type = *input.Type
		}
		if input.ParentID.Set {
			values.ParentID = input.ParentID.Value
		}
		if input.DocumentDate.Set {
			values.DocumentDate = input.DocumentDate.Value
		}
		if input.SortOrder != nil {
			values.SortOrder = *input.SortOrder
		}
		if input.ColorHex != nil {
			values.ColorHex = input.ColorHex
		}
		clean, err := validateSection(values)
		if err != nil {
			return err
		}
		if err := validParent(tx, expedienteID, clean.ExpedienteSectionParentID); err != nil {
			return err
		}
		// Map updates preserve explicit NULLs and zero sort_order.
		updates := map[string]any{"name": clean.Name, "type": clean.Type, "expediente_section_parent_id": clean.ExpedienteSectionParentID, "document_date": clean.DocumentDate, "sort_order": clean.SortOrder, "color_hex": clean.ColorHex}
		return tx.Model(&result).Clauses(clause.Returning{}).Where("id = ? AND expediente_id = ?", id, expedienteID).Updates(updates).Error
	})
	return sectionResponse(result), sectionDBError(err)
}
func (s *DocumentService) DeleteSection(ctx context.Context, expedienteID, id int64) error {
	if expedienteID <= 0 || id <= 0 {
		return ErrSectionInvalid
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		if _, err := getSection(tx, expedienteID, id); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.ExpedienteSection{}).Where("expediente_section_parent_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrSectionHasChildren
		}
		return tx.Delete(&models.ExpedienteSection{}, id).Error
	})
	return sectionDBError(err)
}
func (s *DocumentService) CreateInSection(ctx context.Context, expedienteID, sectionID int64, input CreateDocumentInput) (DocumentResponse, error) {
	if expedienteID <= 0 || sectionID <= 0 {
		return DocumentResponse{}, ErrSectionInvalid
	}
	row, err := cleanDocument(input)
	if err != nil {
		return DocumentResponse{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		if _, err := getSection(tx, expedienteID, sectionID); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Create(&models.ExpedienteSectionDocument{ExpedienteSectionID: sectionID, DocumentID: row.ID}).Error
	})
	return documentResponse(row), err
}
func (s *DocumentService) ListSectionDocuments(ctx context.Context, expedienteID, sectionID int64) ([]DocumentResponse, error) {
	if expedienteID <= 0 || sectionID <= 0 {
		return nil, ErrSectionInvalid
	}
	db := s.db.WithContext(ctx)
	if err := requireExpediente(db, expedienteID, false); err != nil {
		return nil, err
	}
	if _, err := getSection(db, expedienteID, sectionID); err != nil {
		return nil, err
	}
	var rows []models.Document
	err := db.Where("id IN (SELECT document_id FROM expediente_section_documents WHERE expediente_section_id = ?)", sectionID).Order("created_at ASC, id ASC").Find(&rows).Error
	result := make([]DocumentResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, documentResponse(row))
	}
	return result, err
}
func (s *DocumentService) AttachDocument(ctx context.Context, expedienteID, sectionID, documentID int64) error {
	if expedienteID <= 0 || sectionID <= 0 || documentID <= 0 {
		return ErrSectionInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDocument(tx, documentID); err != nil {
			return err
		}
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		if _, err := getSection(tx, expedienteID, sectionID); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ExpedienteSectionDocument{ExpedienteSectionID: sectionID, DocumentID: documentID}).Error
	})
}
func (s *DocumentService) DetachDocument(ctx context.Context, expedienteID, sectionID, documentID int64) error {
	if expedienteID <= 0 || sectionID <= 0 || documentID <= 0 {
		return ErrSectionInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDocument(tx, documentID); err != nil {
			return err
		}
		if err := requireExpediente(tx, expedienteID, true); err != nil {
			return err
		}
		if _, err := getSection(tx, expedienteID, sectionID); err != nil {
			return err
		}
		result := tx.Where("expediente_section_id = ? AND document_id = ?", sectionID, documentID).Delete(&models.ExpedienteSectionDocument{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrSectionDocumentNotFound
		}
		return nil
	})
}
