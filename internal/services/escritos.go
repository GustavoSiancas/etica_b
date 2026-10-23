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
)

var ErrEscritoInvalid = errors.New("invalid escrito")
var ErrEscritoNotFound = errors.New("escrito not found")
var ErrEscritoAlreadyAttended = errors.New("escrito is already attended")

const (
	EscritoNueva    = "NUEVA"
	EscritoAtendida = "ATENDIDA"
)

type CreateEscritoInput struct {
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	ExpedienteNumber string  `json:"expediente_number"`
	DocumentDate     *string `json:"document_date"`
	FileURL          *string `json:"file_url"`
	DenuncianteIDs   []int64 `json:"denunciante_ids"`
	DenunciadoIDs    []int64 `json:"denunciado_ids"`
}
type EscritoResponse struct {
	ID               int64                 `json:"id"`
	Title            string                `json:"title"`
	Type             string                `json:"type"`
	ExpedienteNumber string                `json:"expediente_number"`
	DocumentDate     *string               `json:"document_date"`
	FileURL          *string               `json:"file_url"`
	Status           string                `json:"status"`
	Denunciantes     []ParticipantResponse `json:"denunciantes"`
	Denunciados      []ParticipantResponse `json:"denunciados"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}
type EscritoPage struct {
	Data   []EscritoResponse `json:"data"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}
type EscritoService struct{ db *gorm.DB }

func NewEscritoService(db *gorm.DB) *EscritoService { return &EscritoService{db: db} }

func (s *EscritoService) Create(ctx context.Context, input CreateEscritoInput) (EscritoResponse, error) {
	title := strings.TrimSpace(input.Title)
	kind := strings.TrimSpace(input.Type)
	expedienteNumber := strings.TrimSpace(input.ExpedienteNumber)
	if title == "" || !utf8.ValidString(title) || utf8.RuneCountInString(title) > 255 || strings.ContainsRune(title, 0) || kind == "" || !utf8.ValidString(kind) || utf8.RuneCountInString(kind) > 100 || strings.ContainsRune(kind, 0) || expedienteNumber == "" || len(expedienteNumber) > 50 || !validParticipantIDs(input.DenuncianteIDs) || !validParticipantIDs(input.DenunciadoIDs) || (len(input.DenuncianteIDs) == 0 && len(input.DenunciadoIDs) == 0) {
		return EscritoResponse{}, ErrEscritoInvalid
	}
	var date *time.Time
	if input.DocumentDate != nil && *input.DocumentDate != "" {
		parsed, err := time.Parse("2006-01-02", *input.DocumentDate)
		if err != nil {
			return EscritoResponse{}, ErrEscritoInvalid
		}
		date = &parsed
	}
	var fileURL *string
	if input.FileURL != nil && strings.TrimSpace(*input.FileURL) != "" {
		value := strings.TrimSpace(*input.FileURL)
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return EscritoResponse{}, ErrEscritoInvalid
		}
		fileURL = &value
	}
	var result EscritoResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		complainants := make([]models.Denunciante, 0, len(input.DenuncianteIDs))
		if len(input.DenuncianteIDs) > 0 {
			if err := tx.Where("id IN ?", input.DenuncianteIDs).Order("id ASC").Find(&complainants).Error; err != nil {
				return err
			}
			if len(complainants) != len(input.DenuncianteIDs) {
				return ErrParticipantNotFound
			}
		}
		respondents := make([]models.Denunciado, 0, len(input.DenunciadoIDs))
		if len(input.DenunciadoIDs) > 0 {
			if err := tx.Where("id IN ?", input.DenunciadoIDs).Order("id ASC").Find(&respondents).Error; err != nil {
				return err
			}
			if len(respondents) != len(input.DenunciadoIDs) {
				return ErrParticipantNotFound
			}
		}
		row := models.Escrito{Title: title, Type: kind, ExpedienteNumber: expedienteNumber, DocumentDate: date, FileURL: fileURL, Status: EscritoNueva}
		if err := tx.Omit("Denunciantes", "Denunciados").Create(&row).Error; err != nil {
			return err
		}
		complainantResponses := make([]ParticipantResponse, 0, len(complainants))
		for _, person := range complainants {
			link := models.EscritoDenunciante{EscritoID: row.ID, DenuncianteID: person.ID}
			if err := tx.Omit("Escrito", "Denunciante").Create(&link).Error; err != nil {
				return err
			}
			complainantResponses = append(complainantResponses, denuncianteResponse(person))
		}
		defendantResponses := make([]ParticipantResponse, 0, len(respondents))
		for _, person := range respondents {
			link := models.EscritoDenunciado{EscritoID: row.ID, DenunciadoID: person.ID}
			if err := tx.Omit("Escrito", "Denunciado").Create(&link).Error; err != nil {
				return err
			}
			defendantResponses = append(defendantResponses, denunciadoResponse(person))
		}
		result = escritoResponse(row, complainantResponses, defendantResponses)
		return nil
	})
	return result, err
}
func (s *EscritoService) List(ctx context.Context, status string, limit, offset int) (EscritoPage, error) {
	page := EscritoPage{Data: make([]EscritoResponse, 0), Limit: limit, Offset: offset}
	status = strings.ToUpper(strings.TrimSpace(status))
	if limit < 1 || limit > 100 || offset < 0 || (status != "" && status != EscritoNueva && status != EscritoAtendida) {
		return page, ErrEscritoInvalid
	}
	q := s.db.WithContext(ctx).Model(&models.Escrito{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&page.Total).Error; err != nil {
		return page, err
	}
	var rows []models.Escrito
	listQuery := s.db.WithContext(ctx)
	if status != "" {
		listQuery = listQuery.Where("status = ?", status)
	}
	if err := listQuery.Preload("Denunciantes.Denunciante").Preload("Denunciados.Denunciado").Order("created_at ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return page, err
	}
	for _, row := range rows {
		complainants := make([]ParticipantResponse, 0, len(row.Denunciantes))
		for _, link := range row.Denunciantes {
			complainants = append(complainants, denuncianteResponse(link.Denunciante))
		}
		respondents := make([]ParticipantResponse, 0, len(row.Denunciados))
		for _, link := range row.Denunciados {
			respondents = append(respondents, denunciadoResponse(link.Denunciado))
		}
		page.Data = append(page.Data, escritoResponse(row, complainants, respondents))
	}
	return page, nil
}
func (s *EscritoService) UpdateStatus(ctx context.Context, id int64, status string) (EscritoResponse, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if id <= 0 || status != EscritoAtendida {
		return EscritoResponse{}, ErrEscritoInvalid
	}
	now := time.Now()
	result := s.db.WithContext(ctx).Model(&models.Escrito{}).
		Where("id = ? AND status = ?", id, EscritoNueva).
		Updates(map[string]any{"status": EscritoAtendida, "updated_at": now})
	if result.Error != nil {
		return EscritoResponse{}, result.Error
	}
	if result.RowsAffected == 0 {
		var exists int64
		if err := s.db.WithContext(ctx).Model(&models.Escrito{}).Where("id = ?", id).Count(&exists).Error; err != nil {
			return EscritoResponse{}, err
		}
		if exists == 0 {
			return EscritoResponse{}, ErrEscritoNotFound
		}
		return EscritoResponse{}, ErrEscritoAlreadyAttended
	}
	var row models.Escrito
	if err := s.db.WithContext(ctx).Preload("Denunciantes.Denunciante").Preload("Denunciados.Denunciado").First(&row, id).Error; err != nil {
		return EscritoResponse{}, err
	}
	complainants := make([]ParticipantResponse, 0, len(row.Denunciantes))
	for _, link := range row.Denunciantes {
		complainants = append(complainants, denuncianteResponse(link.Denunciante))
	}
	respondents := make([]ParticipantResponse, 0, len(row.Denunciados))
	for _, link := range row.Denunciados {
		respondents = append(respondents, denunciadoResponse(link.Denunciado))
	}
	return escritoResponse(row, complainants, respondents), nil
}

func escritoResponse(row models.Escrito, complainants, respondents []ParticipantResponse) EscritoResponse {
	var date *string
	if row.DocumentDate != nil {
		value := row.DocumentDate.Format("2006-01-02")
		date = &value
	}
	return EscritoResponse{ID: row.ID, Title: row.Title, Type: row.Type, ExpedienteNumber: row.ExpedienteNumber, DocumentDate: date, FileURL: row.FileURL, Status: row.Status, Denunciantes: complainants, Denunciados: respondents, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
