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
