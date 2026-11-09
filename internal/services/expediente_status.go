package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"etica_back/internal/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UpdateExpedienteStatusInput struct {
	Status string `json:"status"`
}

// All state transitions lock the expediente before its folders.
func (s *ExpedienteService) UpdateStatus(ctx context.Context, id int64, input UpdateExpedienteStatusInput) (ExpedienteResponse, error) {
	value := strings.TrimSpace(input.Status)
	if id <= 0 || !validFolderStatus(value) {
		return ExpedienteResponse{}, ErrExpedienteInvalid
	}
	var row models.Expediente
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExpedienteNotFound
		}
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if value == FolderCerrado {
			var folders []models.ExpedienteFolder
			if err := tx.Where("expediente_id = ? AND status = ?", id, FolderAbierto).Order("id").Find(&folders).Error; err != nil {
				return err
			}
			// Already closed folders retain their own dates and updated_at.
			if err := tx.Model(&models.ExpedienteFolder{}).Where("expediente_id = ? AND status = ?", id, FolderAbierto).
				Updates(map[string]any{"status": FolderCerrado, "closing_date": now}).Error; err != nil {
				return err
			}
			for _, folder := range folders {
				if err := recordExpedienteHistory(tx, id, &folder.ID, "CARPETA_CERRADA", now); err != nil {
					return err
				}
			}
		}
		if row.Status == value {
			return nil
		}
		if err := tx.Model(&row).Updates(expedienteStatusUpdates(value, now)).Error; err != nil {
			return err
		}
		return recordExpedienteHistory(tx, id, nil, statusHistoryEvent(value, false), now)
	})
	return expedienteResponse(row), err
}

func expedienteStatusUpdates(status string, now time.Time) map[string]any {
	updates := map[string]any{"status": status, "closing_date": nil}
	if status == FolderCerrado {
		date, _ := time.Parse("2006-01-02", now.In(time.FixedZone("America/Lima", -5*60*60)).Format("2006-01-02"))
		updates["closing_date"] = date
	}
	return updates
}

// Caller holds the expediente lock, serializing creation and state changes.
func syncExpedienteStatus(tx *gorm.DB, id int64, now time.Time) error {
	var count int64
	if err := tx.Model(&models.ExpedienteFolder{}).Where("expediente_id = ? AND status = ?", id, FolderAbierto).Count(&count).Error; err != nil {
		return err
	}
	status := FolderCerrado
	if count > 0 {
		status = FolderAbierto
	}
	result := tx.Model(&models.Expediente{}).Where("id = ? AND status <> ?", id, status).Updates(expedienteStatusUpdates(status, now))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return recordExpedienteHistory(tx, id, nil, statusHistoryEvent(status, false), now)
}

func statusHistoryEvent(status string, folder bool) string {
	if folder {
		if status == FolderCerrado {
			return "CARPETA_CERRADA"
		}
		return "CARPETA_ABIERTA"
	}
	if status == FolderCerrado {
		return "EXPEDIENTE_CERRADO"
	}
	return "EXPEDIENTE_ABIERTO"
}
