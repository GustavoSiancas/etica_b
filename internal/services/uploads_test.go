package services

import (
	"context"
	"errors"
	"testing"
)

func TestRejectInvalidPublicOrigins(t *testing.T) {
	for _, origin := range []string{"", "http://files.example.com", "https://files.example.com/path", "https://user:secret@files.example.com", "https://files.example.com?token=secret"} {
		if _, err := NewUploadService(nil, nil, "bucket", origin); err == nil {
			t.Errorf("accepted invalid origin %q", origin)
		}
	}
}

func TestConfirmRejectsUnrelatedURLsBeforeAccessingStorage(t *testing.T) {
	s, err := NewUploadService(nil, nil, "bucket", "https://files.example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, fileURL := range []string{
		"https://other.example.com/documents/1/0123456789abcdef0123456789abcdef.png",
		"https://files.example.com/documents/2/0123456789abcdef0123456789abcdef.png",
		"https://files.example.com/documents/1/../../secret.png",
		"https://files.example.com/documents/1/0123456789abcdef0123456789abcdef.png?signature=expired",
	} {
		if err := s.Confirm(context.Background(), 1, fileURL); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted unrelated URL: %v", err)
		}
	}
}

func TestSignRejectsUnsupportedImagesBeforeDatabaseAccess(t *testing.T) {
	s := &UploadService{}
	for _, contentType := range []string{"image/svg+xml", "text/html", "application/octet-stream", ""} {
		if _, err := s.Sign(context.Background(), 1, contentType); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted %q", contentType)
		}
	}
}
