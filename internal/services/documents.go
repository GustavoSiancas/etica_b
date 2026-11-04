package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"etica_back/internal/persistence/models"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDocumentInvalid = errors.New("invalid document")
var ErrDocumentLinked = errors.New("document is linked to sections")
var ErrExpedienteNotFound = errors.New("expediente not found")

// Optional distinguishes an omitted field from an explicit JSON null.
type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

type CreateDocumentInput struct {
	DocumentDate *DocumentDate `json:"document_date"`
	Name         string        `json:"name"`
	FileURL      *string       `json:"file_url"`
}
type UpdateDocumentInput struct {
	DocumentDate Optional[DocumentDate] `json:"document_date"`
	Name         *string                `json:"name"`
	FileURL      Optional[string]       `json:"file_url"`
}
type DocumentResponse struct {
	DocumentDate *time.Time `json:"document_date"`
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	FileURL      *string    `json:"file_url"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
type DocumentPage struct {
	Data   []DocumentResponse `json:"data"`
	Total  int64              `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}
type DocumentService struct{ db *gorm.DB }

func NewDocumentService(db *gorm.DB) *DocumentService { return &DocumentService{db: db} }

func validDocumentText(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
func cleanDocument(input CreateDocumentInput) (models.Document, error) {
	name := strings.TrimSpace(input.Name)
	if !validDocumentText(name) || utf8.RuneCountInString(name) > 255 {
		return models.Document{}, ErrDocumentInvalid
	}
	var fileURL *string
	if input.FileURL != nil && strings.TrimSpace(*input.FileURL) != "" {
		value := strings.TrimSpace(*input.FileURL)
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.ContainsRune(value, 0) {
			return models.Document{}, ErrDocumentInvalid
		}
		fileURL = &value
	}
	var date *time.Time
	if input.DocumentDate != nil {
		value := input.DocumentDate.UTC()
		date = &value
	}
	return models.Document{Name: name, FileURL: fileURL, DocumentDate: date}, nil
}
func documentResponse(d models.Document) DocumentResponse {
	var date *time.Time
	if d.DocumentDate != nil {
		value := d.DocumentDate.UTC()
		date = &value
	}
	return DocumentResponse{ID: d.ID, Name: d.Name, FileURL: d.FileURL, DocumentDate: date, CreatedAt: d.CreatedAt.UTC(), UpdatedAt: d.UpdatedAt.UTC()}
}
func (s *DocumentService) Create(ctx context.Context, input CreateDocumentInput) (DocumentResponse, error) {
	row, err := cleanDocument(input)
	if err != nil {
		return DocumentResponse{}, err
	}
	err = s.db.WithContext(ctx).Create(&row).Error
	return documentResponse(row), err
}
func (s *DocumentService) Get(ctx context.Context, id int64) (DocumentResponse, error) {
	if id <= 0 {
		return DocumentResponse{}, ErrDocumentInvalid
	}
	var row models.Document
	err := s.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrNotFound
	}
	return documentResponse(row), err
}
func (s *DocumentService) List(ctx context.Context, limit, offset int) (DocumentPage, error) {
	page := DocumentPage{Data: []DocumentResponse{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 {
		return page, ErrDocumentInvalid
	}
	db := s.db.WithContext(ctx)
	if err := db.Model(&models.Document{}).Count(&page.Total).Error; err != nil {
		return page, err
	}
	var rows []models.Document
	if err := db.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return page, err
	}
	for _, row := range rows {
		page.Data = append(page.Data, documentResponse(row))
	}
	return page, nil
}
func (s *DocumentService) Update(ctx context.Context, id int64, input UpdateDocumentInput) (DocumentResponse, error) {
	if id <= 0 || (input.Name == nil && !input.FileURL.Set && !input.DocumentDate.Set) {
		return DocumentResponse{}, ErrDocumentInvalid
	}
	updates := map[string]any{}
	if input.DocumentDate.Set {
		var date *time.Time
		if input.DocumentDate.Value != nil {
			value := input.DocumentDate.Value.UTC()
			date = &value
		}
		updates["document_date"] = date
	}
	if input.Name != nil {
		row, err := cleanDocument(CreateDocumentInput{Name: *input.Name})
		if err != nil {
			return DocumentResponse{}, err
		}
		updates["name"] = row.Name
	}
	if input.FileURL.Set {
		row, err := cleanDocument(CreateDocumentInput{Name: "validation", FileURL: input.FileURL.Value})
		if err != nil {
			return DocumentResponse{}, err
		}
		updates["file_url"] = row.FileURL
	}
	var row models.Document
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockWritableDocument(tx, id); err != nil {
			return err
		}
		return tx.Model(&row).Clauses(clause.Returning{}).Where("id = ?", id).Updates(updates).Error
	})
	return documentResponse(row), err
}
func (s *DocumentService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrDocumentInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockWritableDocument(tx, id); err != nil {
			return err
		}
		result := tx.Delete(&models.Document{}, id)
		if result.Error != nil {
			var pg *pgconn.PgError
			if errors.As(result.Error, &pg) && (pg.Code == "23503" || pg.Code == "23001") {
				return ErrDocumentLinked
			}
		}
		return result.Error
	})
}

// Always lock a document before its owner expedientes. Attach/detach uses the
// same order, preventing new owners from slipping past a closed-state check.
func lockDocument(tx *gorm.DB, id int64) (models.Document, error) {
	var row models.Document
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrNotFound
	}
	return row, err
}
func lockWritableDocument(tx *gorm.DB, id int64) (models.Document, error) {
	row, err := lockDocument(tx, id)
	if err != nil {
		return row, err
	}
	var owners []models.Expediente
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(`id IN (
        SELECT s.expediente_id FROM expediente_section s
        JOIN expediente_section_documents l ON l.expediente_section_id=s.id WHERE l.document_id=?)`, id).
		Order("id ASC").Find(&owners).Error
	if err != nil {
		return row, err
	}
	for _, owner := range owners {
		if owner.Status == ExpedienteCerrado {
			return row, ErrExpedienteClosed
		}
	}
	return row, nil
}

func requireExpediente(tx *gorm.DB, id int64, lock bool) error {
	var row models.Expediente
	q := tx.Select("id", "status")
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrExpedienteNotFound
	}
	if err == nil && lock && row.Status == ExpedienteCerrado {
		return ErrExpedienteClosed
	}
	return err
}
func (s *DocumentService) ListByExpediente(ctx context.Context, id int64) ([]DocumentResponse, error) {
	if id <= 0 {
		return nil, ErrDocumentInvalid
	}
	db := s.db.WithContext(ctx)
	if err := requireExpediente(db, id, false); err != nil {
		return nil, err
	}
	rows := []models.Document{}
	err := db.Where(`id IN (SELECT l.document_id FROM expediente_section_documents l
        JOIN expediente_section s ON s.id = l.expediente_section_id WHERE s.expediente_id = ?)`, id).
		Order("created_at ASC, id ASC").Find(&rows).Error
	result := make([]DocumentResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, documentResponse(row))
	}
	return result, err
}

type DocumentTypeEntry struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

func (s *DocumentService) ListTypes(ctx context.Context) ([]DocumentTypeEntry, error) {
	var rows []models.DocumentType
	if err := s.db.WithContext(ctx).Where("is_active = TRUE").Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]DocumentTypeEntry, 0, len(rows))
	for _, r := range rows {
		result = append(result, DocumentTypeEntry{ID: r.ID, Code: r.Code, Name: r.Name, SortOrder: r.SortOrder})
	}
	return result, nil
}
