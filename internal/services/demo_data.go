package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
)

var ErrDemoResetDisabled = errors.New("demo data reset is disabled")

type DemoDataSummary struct {
	Expedientes  int64 `json:"expedientes"`
	Denunciantes int64 `json:"denunciantes"`
	Denunciados  int64 `json:"denunciados"`
	Documents    int64 `json:"documents"`
	Escritos     int64 `json:"escritos"`
}

type DemoDataService struct {
	db      *gorm.DB
	path    string
	enabled bool
}

func NewDemoDataService(db *gorm.DB, path string, enabled bool) *DemoDataService {
	return &DemoDataService{db: db, path: path, enabled: enabled}
}

func (s *DemoDataService) Enabled() bool { return s != nil && s.enabled }

// Reset reloads fixture rows, including internally marked participants.
func (s *DemoDataService) Reset(ctx context.Context) (DemoDataSummary, error) {
	if !s.Enabled() {
		return DemoDataSummary{}, ErrDemoResetDisabled
	}
	sql, err := os.ReadFile(filepath.Clean(s.path))
	if err != nil {
		return DemoDataSummary{}, fmt.Errorf("read demo data seed: %w", err)
	}
	if err := s.db.WithContext(ctx).Exec(string(sql)).Error; err != nil {
		return DemoDataSummary{}, err
	}
	var summary DemoDataSummary
	queries := []struct {
		table, column string
		count         *int64
	}{
		{"expedientes", "description", &summary.Expedientes},
		{"denunciantes", "name", &summary.Denunciantes},
		{"denunciados", "name", &summary.Denunciados},
		{"documents", "name", &summary.Documents},
		{"escritos", "title", &summary.Escritos},
	}
	for _, query := range queries {
		rows := s.db.WithContext(ctx).Table(query.table)
		if query.table == "denunciantes" || query.table == "denunciados" {
			rows = rows.Where("is_demo = ?", true)
		} else {
			rows = rows.Where(query.column+" LIKE ?", "[DEMO]%")
		}
		if err := rows.Count(query.count).Error; err != nil {
			return DemoDataSummary{}, err
		}
	}
	return summary, nil
}
