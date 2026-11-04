package services

import (
	"context"
	"errors"
	"etica_back/internal/persistence/models"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type FolderTypeResponse struct {
	Abbreviation *string `json:"abbreviation"`
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
}
type FolderResponse struct {
	Status       string     `json:"status"`
	Location     string     `json:"location"`
	Abbreviation *string    `json:"abbreviation"`
	OpeningDate  *time.Time `json:"opening_date"`
	ID           int64      `json:"id"`
	ExpedienteID int64      `json:"expediente_id"`
	FolderTypeID int64      `json:"folder_type_id"`
	Name         string     `json:"name"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
type CreateFolderInput struct {
	Status       *string       `json:"status"`
	Location     *string       `json:"location"`
	FolderTypeID int64         `json:"folder_type_id"`
	OpeningDate  *DocumentDate `json:"opening_date"`
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
	err := db.Table("expediente_folders f").Select("f.*, t.name, t.abbreviation").Joins("JOIN folder_types t ON t.id=f.folder_type_id").Where("f.expediente_id = ?", expedienteID).Order("f.id").Scan(&rows).Error
	for i := range rows {
		rows[i].OpeningDate = utcFolderDate(rows[i].OpeningDate)
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
	status, location := FolderAbierto, UbicacionDireccion
	if input.Status != nil {
		status = strings.TrimSpace(*input.Status)
	}
	if input.Location != nil {
		location = strings.TrimSpace(*input.Location)
	}
	if !validFolderStatus(status) || !validLocation(location) {
		return FolderResponse{}, ErrFolderInvalid
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
		// Preserve idempotency: an existing folder keeps its original opening date.
		err := tx.Where("expediente_id = ? AND folder_type_id = ?", expedienteID, input.FolderTypeID).First(&row).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var openingDate *time.Time
		if kind.Name == "EXPEDIENTE PRINCIPAL" {
			var expediente models.Expediente
			if err := tx.First(&expediente, expedienteID).Error; err != nil {
				return err
			}
			if expediente.OpeningDate != nil {
				date, err := time.ParseInLocation("2006-01-02", expediente.OpeningDate.Format("2006-01-02"), time.FixedZone("America/Lima", -5*60*60))
				if err != nil {
					return err
				}
				openingDate = utcFolderDate(&date)
			}
		} else {
			if input.OpeningDate == nil {
				return ErrSectionInvalid
			}
			openingDate = utcFolderDate(&input.OpeningDate.Time)
		}
		row = models.ExpedienteFolder{ExpedienteID: expedienteID, FolderTypeID: input.FolderTypeID, OpeningDate: openingDate, Status: status, Location: location}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Where("expediente_id = ? AND folder_type_id = ?", expedienteID, input.FolderTypeID).First(&row).Error
	})
	return FolderResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, FolderTypeID: row.FolderTypeID, Name: kind.Name, Status: row.Status, Location: row.Location, Abbreviation: kind.Abbreviation, OpeningDate: utcFolderDate(row.OpeningDate), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}, sectionDBError(err)
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

func utcFolderDate(date *time.Time) *time.Time {
	if date == nil {
		return nil
	}
	value := date.UTC()
	return &value
}

var ErrFolderTypeInvalid = errors.New("invalid folder type name or abbreviation")
var ErrFolderTypeDuplicate = errors.New("folder type already exists")

type CreateFolderTypeInput struct {
	Abbreviation string `json:"abbreviation"`
	Name         string `json:"name"`
}

func (s *DocumentService) CreateFolderType(ctx context.Context, input CreateFolderTypeInput) (FolderTypeResponse, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || utf8.RuneCountInString(name) > 255 {
		return FolderTypeResponse{}, ErrFolderTypeInvalid
	}
	abbreviation, err := normalizeFolderAbbreviation(input.Abbreviation)
	if err != nil {
		return FolderTypeResponse{}, err
	}
	row := models.FolderType{Name: strings.ToUpper(name), Abbreviation: &abbreviation}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return FolderTypeResponse{}, ErrFolderTypeDuplicate
		}
		return FolderTypeResponse{}, err
	}
	return FolderTypeResponse{ID: row.ID, Name: row.Name, Abbreviation: row.Abbreviation}, nil
}

func normalizeFolderAbbreviation(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 20 {
		return "", ErrFolderTypeInvalid
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return "", ErrFolderTypeInvalid
		}
	}
	return strings.ToUpper(value), nil
}

var ErrFolderInvalid = errors.New("invalid folder fields")
var ErrFolderNotFound = errors.New("folder not found in expediente")

type UpdateFolderStatusInput struct {
	Status string `json:"status"`
}
type UpdateFolderLocationInput struct {
	Location string `json:"location"`
}

func validFolderStatus(value string) bool {
	return value == FolderAbierto || value == FolderCerrado
}

// Locking the folder serializes closing it against all content mutations.
func lockFolder(tx *gorm.DB, expedienteID, folderID int64, writable bool) (models.ExpedienteFolder, error) {
	var row models.ExpedienteFolder
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("expediente_id = ?", expedienteID).First(&row, folderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrFolderNotFound
	}
	if err == nil && writable && row.Status == FolderCerrado {
		return row, ErrFolderClosed
	}
	return row, err
}

func (s *DocumentService) UpdateFolderStatus(ctx context.Context, expedienteID, folderID int64, input UpdateFolderStatusInput) (FolderResponse, error) {
	value := strings.TrimSpace(input.Status)
	if expedienteID <= 0 || folderID <= 0 || !validFolderStatus(value) {
		return FolderResponse{}, ErrFolderInvalid
	}
	return s.updateFolderField(ctx, expedienteID, folderID, "status", value)
}
func (s *DocumentService) UpdateFolderLocation(ctx context.Context, expedienteID, folderID int64, input UpdateFolderLocationInput) (FolderResponse, error) {
	value := strings.TrimSpace(input.Location)
	if expedienteID <= 0 || folderID <= 0 || !validLocation(value) {
		return FolderResponse{}, ErrFolderInvalid
	}
	return s.updateFolderField(ctx, expedienteID, folderID, "location", value)
}
func (s *DocumentService) updateFolderField(ctx context.Context, expedienteID, folderID int64, field, value string) (FolderResponse, error) {
	var row models.ExpedienteFolder
	var kind models.FolderType
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireExpediente(tx, expedienteID, false); err != nil {
			return err
		}
		var err error
		row, err = lockFolder(tx, expedienteID, folderID, false)
		if err != nil {
			return err
		}
		if err := tx.Model(&row).Update(field, value).Error; err != nil {
			return err
		}
		return tx.First(&kind, row.FolderTypeID).Error
	})
	return FolderResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, FolderTypeID: row.FolderTypeID, Name: kind.Name, Abbreviation: kind.Abbreviation, OpeningDate: utcFolderDate(row.OpeningDate), Status: row.Status, Location: row.Location, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}, err
}

func requireWritableSection(tx *gorm.DB, expedienteID, sectionID int64) error {
	row, err := getSection(tx, expedienteID, sectionID)
	if err != nil {
		return err
	}
	_, err = lockFolder(tx, expedienteID, row.ExpedienteFolderID, true)
	return err
}

var ErrFolderClosed = errors.New("folder is closed; reopen it before modifying its content")

const (
	FolderAbierto      = "ABIERTO"
	FolderCerrado      = "CERRADO"
	UbicacionDireccion = "DIRECCION DE ETICA"
	UbicacionConsejo   = "CONSEJO DE ETICA"
	UbicacionTribunal  = "TRIBUNAL DE HONOR"
)

type FolderStatusResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var folderStatuses = []FolderStatusResponse{
	{Value: FolderAbierto, Label: "Abierto"},
	{Value: FolderCerrado, Label: "Cerrado"},
}

type FolderLocationResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var folderLocations = []FolderLocationResponse{
	{Value: UbicacionDireccion, Label: "Direccion de Etica"},
	{Value: UbicacionConsejo, Label: "Consejo de Etica"},
	{Value: UbicacionTribunal, Label: "Tribunal de Honor"},
}

func validLocation(value string) bool {
	return value == UbicacionDireccion || value == UbicacionConsejo || value == UbicacionTribunal
}

func (s *DocumentService) ListLocations(context.Context) ([]FolderLocationResponse, error) {
	return append([]FolderLocationResponse(nil), folderLocations...), nil
}

func (s *DocumentService) ListStatuses(context.Context) ([]FolderStatusResponse, error) {
	return append([]FolderStatusResponse(nil), folderStatuses...), nil
}
