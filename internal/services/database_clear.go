package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrDatabaseClearDisabled = errors.New("database clear is disabled: ENABLE_DATABASE_CLEAR must be true")

type DatabaseClearService struct {
	db      *gorm.DB
	enabled bool
}

func NewDatabaseClearService(db *gorm.DB, enabled bool) *DatabaseClearService {
	return &DatabaseClearService{db: db, enabled: enabled}
}

// Clear removes all application rows, including catalogs, and resets identities.
func (s *DatabaseClearService) Clear(ctx context.Context) error {
	if s == nil || !s.enabled {
		return ErrDatabaseClearDisabled
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`TRUNCATE TABLE
			escrito_denunciados, escrito_denunciantes, escritos,
			expediente_denunciados, expediente_denunciantes, denunciados, denunciantes,
			documents, document_substages, document_stages, document_types, expediente_year_counters, expedientes
			RESTART IDENTITY RESTRICT`).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("clear application database: %w", err)
	}
	return nil
}
