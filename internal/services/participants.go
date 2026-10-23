package services

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"etica_back/internal/persistence/models"
	"gorm.io/gorm"
)

var ErrParticipantInvalid = errors.New("invalid participant")
var ErrParticipantNotFound = errors.New("participant not found")
var ErrParticipantDuplicate = errors.New("participant already belongs to expediente")

type CreateDenuncianteInput struct {
	Name           string `json:"name"`
	DocumentType   string `json:"document_type"`
	DocumentNumber string `json:"document_number"`
}
type CreateDenunciadoInput struct {
	Name               string `json:"name"`
	DocumentType       string `json:"document_type"`
	DocumentNumber     string `json:"document_number"`
	RegistrationNumber int64  `json:"registration_number"`
}
type ParticipantResponse struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	DocumentType       *string `json:"document_type"`
	DocumentNumber     *string `json:"document_number"`
	RegistrationNumber *int64  `json:"registration_number,omitempty"`
}

func denuncianteResponse(row models.Denunciante) ParticipantResponse {
	return ParticipantResponse{ID: row.ID, Name: row.Name, DocumentType: row.DocumentType, DocumentNumber: row.DocumentNumber}
}
func denunciadoResponse(row models.Denunciado) ParticipantResponse {
	return ParticipantResponse{ID: row.ID, Name: row.Name, DocumentType: row.DocumentType, DocumentNumber: row.DocumentNumber, RegistrationNumber: row.RegistrationNumber}
}

type ParticipantMembershipResponse struct {
	ID           int64               `json:"id"`
	ExpedienteID int64               `json:"expediente_id"`
	Participant  ParticipantResponse `json:"participant"`
}
type ParticipantService struct{ db *gorm.DB }

func NewParticipantService(db *gorm.DB) *ParticipantService { return &ParticipantService{db: db} }

func (s *ParticipantService) CreateDenunciante(ctx context.Context, input CreateDenuncianteInput) (ParticipantResponse, error) {
	name, err := validateParticipantName(input.Name)
	docType := strings.ToUpper(strings.TrimSpace(input.DocumentType))
	docNumber := strings.TrimSpace(input.DocumentNumber)
	if err != nil || !validParticipantDocument(docType, docNumber, true) {
		return ParticipantResponse{}, ErrParticipantInvalid
	}
	row := models.Denunciante{Name: name, DocumentType: &docType, DocumentNumber: &docNumber}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return ParticipantResponse{}, err
	}
	return denuncianteResponse(row), nil
}

func (s *ParticipantService) CreateDenunciado(ctx context.Context, input CreateDenunciadoInput) (ParticipantResponse, error) {
	name, err := validateParticipantName(input.Name)
	docType := strings.ToUpper(strings.TrimSpace(input.DocumentType))
	docNumber := strings.TrimSpace(input.DocumentNumber)
	if err != nil || !validParticipantDocument(docType, docNumber, false) || input.RegistrationNumber <= 0 {
		return ParticipantResponse{}, ErrParticipantInvalid
	}
	row := models.Denunciado{Name: name, DocumentType: &docType, DocumentNumber: &docNumber, RegistrationNumber: &input.RegistrationNumber}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return ParticipantResponse{}, err
	}
	return denunciadoResponse(row), nil
}

func validateParticipantName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.ContainsRune(name, 0) {
		return "", ErrParticipantInvalid
	}
	return name, nil
}

func (s *ParticipantService) ListDenunciantes(ctx context.Context, limit, offset int) ([]ParticipantResponse, int64, error) {
	var rows []models.Denunciante
	var total int64
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, 0, ErrParticipantInvalid
	}
	q := s.db.WithContext(ctx)
	if err := q.Model(&models.Denunciante{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]ParticipantResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, denuncianteResponse(row))
	}
	return result, total, nil
}
func (s *ParticipantService) ListDenunciados(ctx context.Context, limit, offset int) ([]ParticipantResponse, int64, error) {
	var rows []models.Denunciado
	var total int64
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, 0, ErrParticipantInvalid
	}
	q := s.db.WithContext(ctx)
	if err := q.Model(&models.Denunciado{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]ParticipantResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, denunciadoResponse(row))
	}
	return result, total, nil
}

func (s *ParticipantService) AttachDenunciante(ctx context.Context, expedienteID, participantID int64) (ParticipantMembershipResponse, error) {
	if expedienteID <= 0 || participantID <= 0 {
		return ParticipantMembershipResponse{}, ErrParticipantInvalid
	}
	var result ParticipantMembershipResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var expediente models.Expediente
		if err := tx.Select("id").First(&expediente, expedienteID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExpedienteNotFound
		} else if err != nil {
			return err
		}
		var participant models.Denunciante
		if err := tx.First(&participant, participantID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrParticipantNotFound
		} else if err != nil {
			return err
		}
		membership := models.ExpedienteDenunciante{ExpedienteID: expedienteID, DenuncianteID: participantID}
		if err := tx.Create(&membership).Error; err != nil {
			if duplicate(err) {
				return ErrParticipantDuplicate
			}
			return err
		}
		result = ParticipantMembershipResponse{ID: membership.ID, ExpedienteID: expedienteID, Participant: denuncianteResponse(participant)}
		return nil
	})
	return result, err
}
func (s *ParticipantService) AttachDenunciado(ctx context.Context, expedienteID, participantID int64) (ParticipantMembershipResponse, error) {
	if expedienteID <= 0 || participantID <= 0 {
		return ParticipantMembershipResponse{}, ErrParticipantInvalid
	}
	var result ParticipantMembershipResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var expediente models.Expediente
		if err := tx.Select("id").First(&expediente, expedienteID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExpedienteNotFound
		} else if err != nil {
			return err
		}
		var participant models.Denunciado
		if err := tx.First(&participant, participantID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrParticipantNotFound
		} else if err != nil {
			return err
		}
		membership := models.ExpedienteDenunciado{ExpedienteID: expedienteID, DenunciadoID: participantID}
		if err := tx.Create(&membership).Error; err != nil {
			if duplicate(err) {
				return ErrParticipantDuplicate
			}
			return err
		}
		result = ParticipantMembershipResponse{ID: membership.ID, ExpedienteID: expedienteID, Participant: denunciadoResponse(participant)}
		return nil
	})
	return result, err
}

func duplicate(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &pgErr) && pgErr.SQLState() == "23505")
}

// ListDenunciantesByExpediente returns fixed memberships, ordered by membership ID.
func (s *ParticipantService) ListDenunciantesByExpediente(ctx context.Context, expedienteID int64) ([]ParticipantMembershipResponse, error) {
	if expedienteID <= 0 {
		return nil, ErrParticipantInvalid
	}
	db := s.db.WithContext(ctx)
	var expediente models.Expediente
	if err := db.Select("id").First(&expediente, expedienteID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExpedienteNotFound
	} else if err != nil {
		return nil, err
	}
	var rows []models.ExpedienteDenunciante
	if err := db.Where("expediente_id = ?", expedienteID).Preload("Denunciante").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ParticipantMembershipResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, ParticipantMembershipResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, Participant: denuncianteResponse(row.Denunciante)})
	}
	return result, nil
}

// ListDenunciadosByExpediente returns fixed memberships, ordered by membership ID.
func (s *ParticipantService) ListDenunciadosByExpediente(ctx context.Context, expedienteID int64) ([]ParticipantMembershipResponse, error) {
	if expedienteID <= 0 {
		return nil, ErrParticipantInvalid
	}
	db := s.db.WithContext(ctx)
	var expediente models.Expediente
	if err := db.Select("id").First(&expediente, expedienteID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExpedienteNotFound
	} else if err != nil {
		return nil, err
	}
	var rows []models.ExpedienteDenunciado
	if err := db.Where("expediente_id = ?", expedienteID).Preload("Denunciado").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ParticipantMembershipResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, ParticipantMembershipResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, Participant: denunciadoResponse(row.Denunciado)})
	}
	return result, nil
}

func validParticipantDocument(kind, number string, allowRUC bool) bool {
	if kind != "DNI" && kind != "CE" && !(allowRUC && kind == "RUC") {
		return false
	}
	if len(number) == 0 || len(number) > 20 {
		return false
	}
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
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
