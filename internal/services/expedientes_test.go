package services

import (
	"context"
	"errors"
	"testing"
)

func TestExpedienteValidationBeforeDatabase(t *testing.T) {
	service := NewExpedienteService(nil)
	for _, page := range [][2]int{{0, 0}, {101, 0}, {20, -1}} {
		if _, err := service.List(context.Background(), page[0], page[1]); !errors.Is(err, ErrExpedienteInvalid) {
			t.Errorf("invalid pagination accepted")
		}
	}
}

func TestUpdateDescriptionValidation(t *testing.T) {
	service := NewExpedienteService(nil)
	text := "Text"
	invalid := "bad\x00text"
	for _, tc := range []struct {
		id          int64
		description *string
	}{{0, &text}, {-1, &text}, {1, nil}, {1, &invalid}} {
		if _, err := service.UpdateDescription(context.Background(), tc.id, UpdateExpedienteDescriptionInput{Description: tc.description}); !errors.Is(err, ErrExpedienteInvalid) {
			t.Fatalf("invalid description accepted: %v", err)
		}
	}
}
