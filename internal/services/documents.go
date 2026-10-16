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
var ErrSubstageNotFound = errors.New("document substage not found")

type CreateDocumentInput struct {
	Title        string  `json:"title"`
	SubstageID   int64   `json:"substage_id"`
	DocumentDate *string `json:"document_date"`
	FileURL      *string `json:"file_url"`
}

type StageSummary struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}
type SubstageSummary struct {
	ID        int64        `json:"id"`
	Code      string       `json:"code"`
	Name      string       `json:"name"`
	SortOrder int          `json:"sort_order"`
	Stage     StageSummary `json:"stage"`
}
type DocumentResponse struct {
	ID           int64                 `json:"id"`
	ExpedienteID int64                 `json:"expediente_id"`
	Title        string                `json:"title"`
	FileURL      *string               `json:"file_url"`
	DocumentDate *string               `json:"document_date"`
	Substage     SubstageSummary       `json:"substage"`
	Status       string                `json:"status"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	Denunciantes []ParticipantResponse `json:"denunciantes"`
	Denunciados  []ParticipantResponse `json:"denunciados"`
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
	if expedienteID <= 0 || name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.ContainsRune(name, 0) || input.SubstageID <= 0 {
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
		var substage models.DocumentSubstage
		if err := tx.Preload("Stage").Where("id = ? AND is_active = TRUE", input.SubstageID).First(&substage).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSubstageNotFound
		} else if err != nil {
			return err
		}
		var complainants []models.ExpedienteDenunciante
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("expediente_id = ? AND is_active = TRUE", expedienteID).Order("id ASC").Find(&complainants).Error; err != nil {
			return err
		}
		var respondents []models.ExpedienteDenunciado
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("expediente_id = ? AND is_active = TRUE", expedienteID).Order("id ASC").Find(&respondents).Error; err != nil {
			return err
		}
		document := models.Document{ExpedienteID: expedienteID, Name: name, FileURL: fileURL, DocumentDate: date, SubstageID: substage.ID, Status: "DRAFT"}
		if err := tx.Omit(clause.Associations).Create(&document).Error; err != nil {
			return err
		}
		if len(complainants) > 0 {
			links := make([]models.DocumentDenunciante, 0, len(complainants))
			for _, member := range complainants {
				links = append(links, models.DocumentDenunciante{DocumentID: document.ID, ExpedienteDenuncianteID: member.ID, ExpedienteID: expedienteID})
			}
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}
		if len(respondents) > 0 {
			links := make([]models.DocumentDenunciado, 0, len(respondents))
			for _, member := range respondents {
				links = append(links, models.DocumentDenunciado{DocumentID: document.ID, ExpedienteDenunciadoID: member.ID, ExpedienteID: expedienteID})
			}
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}
		var saved models.Document
		if err := tx.Preload("Substage").Preload("Substage.Stage").Preload("Denunciantes.Participant.Denunciante").Preload("Denunciados.Participant.Denunciado").First(&saved, document.ID).Error; err != nil {
			return err
		}
		result = documentResponse(saved)
		return nil
	})
	return result, err
}

func validParticipantIDs(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
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
	err := s.db.WithContext(ctx).Preload("Substage").Preload("Substage.Stage").Preload("Denunciantes.Participant.Denunciante").Preload("Denunciados.Participant.Denunciado").Where("expediente_id = ?", expedienteID).
		Joins("JOIN document_substages ON document_substages.id = documents.substage_id").
		Joins("JOIN document_stages ON document_stages.id = document_substages.stage_id").
		Order("document_stages.sort_order ASC, document_substages.sort_order ASC, documents.document_date ASC NULLS LAST, documents.created_at ASC, documents.id ASC").Find(&rows).Error
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
	substage := document.Substage
	result := DocumentResponse{ID: document.ID, ExpedienteID: document.ExpedienteID, Title: document.Name, FileURL: document.FileURL, DocumentDate: date,
		Substage: SubstageSummary{ID: substage.ID, Code: substage.Code, Name: substage.Name, SortOrder: substage.SortOrder, Stage: StageSummary{ID: substage.Stage.ID, Code: substage.Stage.Code, Name: substage.Stage.Name, SortOrder: substage.Stage.SortOrder}},
		Status:   document.Status, CreatedAt: document.CreatedAt, UpdatedAt: document.UpdatedAt}
	result.Denunciantes = make([]ParticipantResponse, 0, len(document.Denunciantes))
	for _, link := range document.Denunciantes {
		person := link.Participant.Denunciante
		result.Denunciantes = append(result.Denunciantes, ParticipantResponse{ID: person.ID, Name: person.Name})
	}
	result.Denunciados = make([]ParticipantResponse, 0, len(document.Denunciados))
	for _, link := range document.Denunciados {
		person := link.Participant.Denunciado
		result.Denunciados = append(result.Denunciados, ParticipantResponse{ID: person.ID, Name: person.Name})
	}
	return result
}
