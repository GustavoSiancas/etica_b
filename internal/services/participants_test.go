package services

import (
	"context"
	"errors"
	"testing"
)

func TestParticipantDocumentValidation(t *testing.T) {
	for _, tc := range []struct {
		kind, number string
		ruc, valid   bool
	}{
		{"DNI", "00123456", true, true}, {"RUC", "20123456789", true, true}, {"CE", "000123456", false, true},
		{"RUC", "20123456789", false, false}, {"PASSPORT", "123", true, false}, {"", "123", true, false},
		{"DNI", "", true, false}, {"DNI", "12a34", true, false}, {"DNI", "-123", true, false},
		{"DNI", "123456789012345678901", true, false},
	} {
		if got := validParticipantDocument(tc.kind, tc.number, tc.ruc); got != tc.valid {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}

func TestCreateParticipantsRejectsMissingAndInvalidIdentity(t *testing.T) {
	service := NewParticipantService(nil)
	for _, input := range []CreateDenuncianteInput{
		{Name: "Ana"}, {Name: "Ana", DocumentType: "DNI"}, {Name: "Ana", DocumentType: "RUC", DocumentNumber: "abc"},
		{Name: "", DocumentType: "CE", DocumentNumber: "000123"},
	} {
		if _, err := service.CreateDenunciante(context.Background(), input); !errors.Is(err, ErrParticipantInvalid) {
			t.Fatal(err)
		}
	}
	for _, input := range []CreateDenunciadoInput{
		{Name: "Luis"}, {Name: "Luis", DocumentType: "RUC", DocumentNumber: "20123456789", RegistrationNumber: 123},
		{Name: "Luis", DocumentType: "DNI", DocumentNumber: "00123456"},
		{Name: "Luis", DocumentType: "DNI", DocumentNumber: "00123456", RegistrationNumber: -1},
	} {
		if _, err := service.CreateDenunciado(context.Background(), input); !errors.Is(err, ErrParticipantInvalid) {
			t.Fatal(err)
		}
	}
}
