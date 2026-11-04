package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"etica_back/internal/persistence/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("document not found")
var ErrInvalid = errors.New("invalid upload request")
var ErrStorage = errors.New("cannot verify uploaded object")

const UploadTTL = 5 * time.Minute
const MaxFileSize int64 = 10 * 1024 * 1024

var documentFileTypes = map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp", "image/gif": "gif", "application/pdf": "pdf", "application/msword": "doc", "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx"}
var keyPattern = regexp.MustCompile(`^[a-f0-9]{32}\.(png|jpg|webp|gif|pdf|doc|docx)$`)

type UploadService struct {
	db        *gorm.DB
	client    *s3.Client
	publicURL string
	bucket    string
}
type UploadSignature struct {
	UploadURL    string            `json:"upload_url"`
	FileURL      string            `json:"file_url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	ExpiresIn    int               `json:"expires_in"`
	MaxSizeBytes int64             `json:"max_size_bytes"`
}

func NewUploadService(db *gorm.DB, client *s3.Client, bucket, publicURL string) (*UploadService, error) {
	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("R2_PUBLIC_URL must be the HTTPS public bucket origin")
	}
	return &UploadService{db: db, client: client, bucket: bucket, publicURL: strings.TrimRight(publicURL, "/")}, nil
}

func (s *UploadService) writable(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := lockWritableDocument(tx, id)
		return err
	})
}

func (s *UploadService) Sign(ctx context.Context, id int64, contentType string) (UploadSignature, error) {
	ext, ok := documentFileTypes[contentType]
	if !ok || id <= 0 {
		return UploadSignature{}, ErrInvalid
	}
	if err := s.writable(ctx, id); err != nil {
		return UploadSignature{}, err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return UploadSignature{}, err
	}
	key := fmt.Sprintf("documents/%d/%s.%s", id, hex.EncodeToString(random), ext)
	signed, err := s3.NewPresignClient(s.client).PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType)}, s3.WithPresignExpires(UploadTTL))
	if err != nil {
		return UploadSignature{}, err
	}
	headers := map[string]string{"Content-Type": contentType}
	for k, v := range signed.SignedHeader {
		if !strings.EqualFold(k, "host") {
			headers[k] = strings.Join(v, ",")
		}
	}
	return UploadSignature{UploadURL: signed.URL, FileURL: s.publicURL + "/" + key, Method: "PUT", Headers: headers, ExpiresIn: int(UploadTTL.Seconds()), MaxSizeBytes: MaxFileSize}, nil
}

func (s *UploadService) Confirm(ctx context.Context, id int64, fileURL string) error {
	prefix := fmt.Sprintf("%s/documents/%d/", s.publicURL, id)
	if id <= 0 || !strings.HasPrefix(fileURL, prefix) || !keyPattern.MatchString(strings.TrimPrefix(fileURL, prefix)) {
		return ErrInvalid
	}
	if err := s.writable(ctx, id); err != nil {
		return err
	}
	key := strings.TrimPrefix(fileURL, s.publicURL+"/")
	object, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return ErrStorage
	}
	ext, ok := documentFileTypes[aws.ToString(object.ContentType)]
	if !ok || !strings.HasSuffix(key, "."+ext) || object.ContentLength == nil || *object.ContentLength <= 0 || *object.ContentLength > MaxFileSize {
		return ErrInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockWritableDocument(tx, id); err != nil {
			return err
		}
		return tx.Model(&models.Document{}).Where("id = ?", id).Updates(map[string]any{"file_url": fileURL, "updated_at": time.Now().UTC()}).Error
	})
}
