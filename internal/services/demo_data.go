package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"etica_back/internal/persistence/models"
	"gorm.io/gorm"
)

var ErrDemoResetDisabled = errors.New("participant seed reload is disabled")

type DemoDataSummary struct {
	Denunciados  int64 `json:"denunciados"`
	Denunciantes int64 `json:"denunciantes"`
}
type DemoDataService struct {
	db      *gorm.DB
	paths   []string
	enabled bool
}

func NewDemoDataService(db *gorm.DB, path string, enabled bool, additionalPaths ...string) *DemoDataService {
	return &DemoDataService{db: db, paths: append([]string{path}, additionalPaths...), enabled: enabled}
}
func (s *DemoDataService) Enabled() bool { return s != nil && s.enabled }

// Load runs the insert-only participant seed on every startup. It never resets data.
// The SQL locks the destination table and skips existing CALs, making restarts safe.
func (s *DemoDataService) Load(ctx context.Context) (DemoDataSummary, error) {
	for _, path := range s.paths {
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return DemoDataSummary{}, fmt.Errorf("read participant seed: %w", err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return DemoDataSummary{}, errors.New("participant seed is empty")
		}
		if err := s.db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
			if err := conn.Exec(string(data)).Error; err != nil {
				// A script containing BEGIN can fail before COMMIT. Do not return an
				// aborted transaction to the pool when a manually edited seed fails.
				conn.Exec("ROLLBACK")
				return err
			}
			return nil
		}); err != nil {
			return DemoDataSummary{}, err
		}
	}
	var summary DemoDataSummary
	err := s.db.WithContext(ctx).Model(&models.Denunciado{}).Count(&summary.Denunciados).Error
	if err == nil {
		err = s.db.WithContext(ctx).Model(&models.Denunciante{}).Count(&summary.Denunciantes).Error
	}
	return summary, err
}

// Reset retains the development route for compatibility, but now only inserts
// missing participants; it never deletes or updates existing information.
func (s *DemoDataService) Reset(ctx context.Context) (DemoDataSummary, error) {
	if !s.Enabled() {
		return DemoDataSummary{}, ErrDemoResetDisabled
	}
	return s.Load(ctx)
}
