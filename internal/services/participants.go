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
var ErrMembershipNotFound = errors.New("participant membership is not active")

type ParticipantResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type ParticipantMembershipResponse struct {
	ID           int64               `json:"id"`
	ExpedienteID int64               `json:"expediente_id"`
	Participant  ParticipantResponse `json:"participant"`
	IsActive     bool                `json:"is_active"`
}
type ParticipantService struct{ db *gorm.DB }

func NewParticipantService(db *gorm.DB) *ParticipantService { return &ParticipantService{db: db} }

func (s *ParticipantService) CreateDenunciante(ctx context.Context, name string) (ParticipantResponse, error) {
	name, err := validateParticipantName(name)
	if err != nil {
		return ParticipantResponse{}, err
	}
	row := models.Denunciante{Name: name}
	if err = s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return ParticipantResponse{}, err
	}
	return ParticipantResponse{ID: row.ID, Name: row.Name}, nil
}
func (s *ParticipantService) CreateDenunciado(ctx context.Context, name string) (ParticipantResponse, error) {
	name, err := validateParticipantName(name)
	if err != nil {
		return ParticipantResponse{}, err
	}
	row := models.Denunciado{Name: name}
	if err = s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return ParticipantResponse{}, err
	}
	return ParticipantResponse{ID: row.ID, Name: row.Name}, nil
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
		result = append(result, ParticipantResponse{ID: row.ID, Name: row.Name})
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
		result = append(result, ParticipantResponse{ID: row.ID, Name: row.Name})
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
		membership := models.ExpedienteDenunciante{ExpedienteID: expedienteID, DenuncianteID: participantID, IsActive: true}
		err := tx.Where("expediente_id = ? AND denunciante_id = ?", expedienteID, participantID).First(&membership).Error
		if err == nil {
			if membership.IsActive {
				return ErrParticipantDuplicate
			}
			if err = tx.Model(&membership).Update("is_active", true).Error; err != nil {
				return err
			}
			membership.IsActive = true
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = tx.Create(&membership).Error; err != nil {
				if duplicate(err) {
					return ErrParticipantDuplicate
				}
				return err
			}
		} else {
			return err
		}
		history := models.ExpedienteParticipantHistory{ExpedienteID: expedienteID, ParticipantType: "DENUNCIANTE", ParticipantID: participantID, Action: "ADDED", NewName: &participant.Name}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}
		result = ParticipantMembershipResponse{ID: membership.ID, ExpedienteID: expedienteID, Participant: ParticipantResponse{ID: participant.ID, Name: participant.Name}, IsActive: true}
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
		membership := models.ExpedienteDenunciado{ExpedienteID: expedienteID, DenunciadoID: participantID, IsActive: true}
		err := tx.Where("expediente_id = ? AND denunciado_id = ?", expedienteID, participantID).First(&membership).Error
		if err == nil {
			if membership.IsActive {
				return ErrParticipantDuplicate
			}
			if err = tx.Model(&membership).Update("is_active", true).Error; err != nil {
				return err
			}
			membership.IsActive = true
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = tx.Create(&membership).Error; err != nil {
				if duplicate(err) {
					return ErrParticipantDuplicate
				}
				return err
			}
		} else {
			return err
		}
		history := models.ExpedienteParticipantHistory{ExpedienteID: expedienteID, ParticipantType: "DENUNCIADO", ParticipantID: participantID, Action: "ADDED", NewName: &participant.Name}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}
		result = ParticipantMembershipResponse{ID: membership.ID, ExpedienteID: expedienteID, Participant: ParticipantResponse{ID: participant.ID, Name: participant.Name}, IsActive: true}
		return nil
	})
	return result, err
}

func (s *ParticipantService) RemoveDenunciante(ctx context.Context, expedienteID, participantID int64) error {
	if expedienteID <= 0 || participantID <= 0 {
		return ErrParticipantInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var membership models.ExpedienteDenunciante
		err := tx.Where("expediente_id = ? AND denunciante_id = ? AND is_active = TRUE", expedienteID, participantID).First(&membership).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMembershipNotFound
		}
		if err != nil {
			return err
		}
		var person models.Denunciante
		if err = tx.First(&person, participantID).Error; err != nil {
			return err
		}
		if err = tx.Model(&membership).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Create(&models.ExpedienteParticipantHistory{ExpedienteID: expedienteID, ParticipantType: "DENUNCIANTE", ParticipantID: participantID, Action: "REMOVED", PreviousName: &person.Name}).Error
	})
}
func (s *ParticipantService) RemoveDenunciado(ctx context.Context, expedienteID, participantID int64) error {
	if expedienteID <= 0 || participantID <= 0 {
		return ErrParticipantInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var membership models.ExpedienteDenunciado
		err := tx.Where("expediente_id = ? AND denunciado_id = ? AND is_active = TRUE", expedienteID, participantID).First(&membership).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMembershipNotFound
		}
		if err != nil {
			return err
		}
		var person models.Denunciado
		if err = tx.First(&person, participantID).Error; err != nil {
			return err
		}
		if err = tx.Model(&membership).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Create(&models.ExpedienteParticipantHistory{ExpedienteID: expedienteID, ParticipantType: "DENUNCIADO", ParticipantID: participantID, Action: "REMOVED", PreviousName: &person.Name}).Error
	})
}

func duplicate(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &pgErr) && pgErr.SQLState() == "23505")
}

// ListDenunciantesByExpediente returns only active memberships, ordered by membership ID.
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
	if err := db.Where("expediente_id = ? AND is_active = ?", expedienteID, true).Preload("Denunciante").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ParticipantMembershipResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, ParticipantMembershipResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, Participant: ParticipantResponse{ID: row.Denunciante.ID, Name: row.Denunciante.Name}, IsActive: row.IsActive})
	}
	return result, nil
}

// ListDenunciadosByExpediente returns only active memberships, ordered by membership ID.
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
	if err := db.Where("expediente_id = ? AND is_active = ?", expedienteID, true).Preload("Denunciado").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ParticipantMembershipResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, ParticipantMembershipResponse{ID: row.ID, ExpedienteID: row.ExpedienteID, Participant: ParticipantResponse{ID: row.Denunciado.ID, Name: row.Denunciado.Name}, IsActive: row.IsActive})
	}
	return result, nil
}
