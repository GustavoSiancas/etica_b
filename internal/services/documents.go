package services

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"etica_back/internal/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDocumentInvalid = errors.New("invalid document")
var ErrExpedienteNotFound = errors.New("expediente not found")

type CreateDocumentInput struct {
	Title        string  `json:"title"`
	Stage        string  `json:"stage"`
	Type         string  `json:"type"`
	DocumentDate *string `json:"document_date"`
	FileURL      *string `json:"file_url"`
}

type DocumentResponse struct {
	ID           int64     `json:"id"`
	ExpedienteID int64     `json:"expediente_id"`
	Title        string    `json:"title"`
	FileURL      *string   `json:"file_url"`
	DocumentDate *string   `json:"document_date"`
	Stage        string    `json:"stage"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DocumentService struct{ db *gorm.DB }

func NewDocumentService(db *gorm.DB) *DocumentService { return &DocumentService{db: db} }

type DocumentStageCatalog struct {
	ID        int64                   `json:"id"`
	Code      string                  `json:"code"`
	Name      string                  `json:"name"`
	SortOrder int                     `json:"sort_order"`
	Substages []DocumentSubstageEntry `json:"substages"`
}

type DocumentSubstageEntry struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

func (s *DocumentService) ListStages(ctx context.Context) ([]DocumentStageCatalog, error) {
	var stages []models.DocumentStage
	if err := s.db.WithContext(ctx).Where("is_active = TRUE").Order("sort_order ASC, id ASC").Find(&stages).Error; err != nil {
		return nil, err
	}
	result := make([]DocumentStageCatalog, 0, len(stages))
	for _, stage := range stages {
		entry := DocumentStageCatalog{ID: stage.ID, Code: stage.Code, Name: stage.Name, SortOrder: stage.SortOrder, Substages: make([]DocumentSubstageEntry, 0)}
		var substages []models.DocumentSubstage
		if err := s.db.WithContext(ctx).Where("stage_id = ? AND is_active = TRUE", stage.ID).Order("sort_order ASC, id ASC").Find(&substages).Error; err != nil {
			return nil, err
		}
		for _, substage := range substages {
			entry.Substages = append(entry.Substages, DocumentSubstageEntry{ID: substage.ID, Code: substage.Code, Name: substage.Name, SortOrder: substage.SortOrder})
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *DocumentService) Create(ctx context.Context, expedienteID int64, input CreateDocumentInput) (DocumentResponse, error) {
	name := strings.TrimSpace(input.Title)
	if expedienteID <= 0 || name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.ContainsRune(name, 0) || !validDocumentText(input.Stage) || !validDocumentText(input.Type) {
		return DocumentResponse{}, ErrDocumentInvalid
	}
	var date *time.Time
	if input.DocumentDate != nil && *input.DocumentDate != "" {
		parsed, err := time.Parse("2006-01-02", *input.DocumentDate)
		if err != nil {
			return DocumentResponse{}, ErrDocumentInvalid
		}
		date = &parsed
	}
	var fileURL *string
	if input.FileURL != nil && strings.TrimSpace(*input.FileURL) != "" {
		u, err := url.Parse(strings.TrimSpace(*input.FileURL))
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return DocumentResponse{}, ErrDocumentInvalid
		}
		clean := strings.TrimSpace(*input.FileURL)
		fileURL = &clean
	}
	var result DocumentResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var expediente models.Expediente
		if err := tx.Select("id").First(&expediente, expedienteID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExpedienteNotFound
		} else if err != nil {
			return err
		}
		document := models.Document{ExpedienteID: expedienteID, Name: name, FileURL: fileURL, DocumentDate: date, Stage: strings.TrimSpace(input.Stage), Type: strings.TrimSpace(input.Type), Status: "DRAFT"}
		if err := tx.Omit(clause.Associations).Create(&document).Error; err != nil {
			return err
		}
		result = documentResponse(document)
		return nil
	})
	return result, err
}

func (s *DocumentService) ListByExpediente(ctx context.Context, expedienteID int64) ([]DocumentResponse, error) {
	if expedienteID <= 0 {
		return nil, ErrDocumentInvalid
	}
	var expediente models.Expediente
	if err := s.db.WithContext(ctx).Select("id").First(&expediente, expedienteID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExpedienteNotFound
	} else if err != nil {
		return nil, err
	}
	var rows []models.Document
	err := s.db.WithContext(ctx).Where("expediente_id = ?", expedienteID).
		Order("document_date ASC NULLS LAST, created_at ASC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]DocumentResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, documentResponse(row))
	}
	return result, nil
}

func documentResponse(document models.Document) DocumentResponse {
	var date *string
	if document.DocumentDate != nil {
		formatted := document.DocumentDate.Format("2006-01-02")
		date = &formatted
	}
	result := DocumentResponse{ID: document.ID, ExpedienteID: document.ExpedienteID, Title: document.Name, FileURL: document.FileURL, DocumentDate: date,
		Stage: document.Stage, Type: document.Type,
		Status: document.Status, CreatedAt: document.CreatedAt, UpdatedAt: document.UpdatedAt}
	return result
}

func validDocumentText(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
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
	for _, row := range rows {
		result = append(result, DocumentTypeEntry{ID: row.ID, Code: row.Code, Name: row.Name, SortOrder: row.SortOrder})
	}
	return result, nil
}
