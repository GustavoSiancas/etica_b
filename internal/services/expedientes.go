package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"etica_back/internal/persistence/models"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var ErrExpedienteInvalid = errors.New("invalid expediente request")
var ErrExpedienteDuplicate = errors.New("expediente number already exists")

type Date struct{ time.Time }

func (d *Date) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return fmt.Errorf("date must use YYYY-MM-DD")
	}
	d.Time = parsed
	return nil
}
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.Format("2006-01-02")) }

type CreateExpedienteInput struct {
	Name        string `json:"name"`
	OpeningDate *Date  `json:"opening_date"`
	InitType    string `json:"init_type"`
}
type ExpedienteInitTypeResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type ExpedienteStatusResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type ExpedienteResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	OpeningDate *Date     `json:"opening_date"`
	Number      string    `json:"number"`
	Code        string    `json:"code"`
	InitType    *string   `json:"init_type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type ExpedienteList struct {
	Data   []ExpedienteResponse `json:"data"`
	Total  int64                `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}
type ExpedienteService struct{ db *gorm.DB }

func NewExpedienteService(db *gorm.DB) *ExpedienteService { return &ExpedienteService{db: db} }

var expedienteInitTypes = map[string]string{
	"DENUNCIA_DE_OFICIO":                             "DENUNCIA DE OFICIO",
	"DENUNCIA_DE_PARTE":                              "DENUNCIA DE PARTE",
	"COMUNICACIONES_DE_ENTIDADES_PUBLICAS":           "COMUNICACIONES DE ENTIDADES PUBLICAS",
	"COMUNICACIONES_DE_SANCION_IMPUESTA_POR_PJ_O_MP": "COMUNICACIONES DE SANCION IMPUESTA POR PJ O MP",
	"APROBADO_POR_ASAMBLEA_GENERAL":                  "APROBADO POR ASAMBLEA GENERAL",
}

func ValidExpedienteInitType(value string) bool { _, ok := expedienteInitTypes[value]; return ok }

var expedienteStatuses = []ExpedienteStatusResponse{
	{Value: "OPEN", Label: "Abierto"},
	{Value: "IN_PROGRESS", Label: "En proceso"},
	{Value: "CLOSED", Label: "Cerrado"},
}

func (s *ExpedienteService) Create(ctx context.Context, input CreateExpedienteInput) (ExpedienteResponse, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 255 || input.OpeningDate == nil || !ValidExpedienteInitType(input.InitType) {
		return ExpedienteResponse{}, ErrExpedienteInvalid
	}
	initType := input.InitType
	var openingDate *time.Time
	if input.OpeningDate != nil {
		parsed := input.OpeningDate.Time
		openingDate = &parsed
	}
	expediente := models.Expediente{Name: input.Name, OpeningDate: openingDate, Status: "OPEN", InitType: &initType}
	if err := s.db.WithContext(ctx).Create(&expediente).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &pgErr) && pgErr.Code == "23505") {
			return ExpedienteResponse{}, ErrExpedienteDuplicate
		}
		return ExpedienteResponse{}, err
	}
	return expedienteResponse(expediente), nil
}

func (s *ExpedienteService) List(ctx context.Context, limit, offset int) (ExpedienteList, error) {
	result := ExpedienteList{Data: make([]ExpedienteResponse, 0), Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 {
		return result, ErrExpedienteInvalid
	}
	if err := s.db.WithContext(ctx).Model(&models.Expediente{}).Count(&result.Total).Error; err != nil {
		return result, err
	}
	var rows []models.Expediente
	if err := s.db.WithContext(ctx).Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Data = append(result.Data, expedienteResponse(row))
	}
	return result, nil
}

type UpdateExpedienteStatusInput struct {
	Status string `json:"status"`
}

func (s *ExpedienteService) UpdateStatus(ctx context.Context, id int64, input UpdateExpedienteStatusInput) (ExpedienteResponse, error) {
	status := strings.TrimSpace(input.Status)
	if id <= 0 || status == "" || len([]rune(status)) > 30 || strings.ContainsRune(status, 0) {
		return ExpedienteResponse{}, ErrExpedienteInvalid
	}
	var expediente models.Expediente
	err := s.db.WithContext(ctx).First(&expediente, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ExpedienteResponse{}, ErrExpedienteNotFound
	}
	if err != nil {
		return ExpedienteResponse{}, err
	}
	if err := s.db.WithContext(ctx).Model(&expediente).Update("status", status).Error; err != nil {
		return ExpedienteResponse{}, err
	}
	expediente.Status = status
	return expedienteResponse(expediente), nil
}

func (s *ExpedienteService) ListInitTypes(ctx context.Context) ([]ExpedienteInitTypeResponse, error) {
	result := make([]ExpedienteInitTypeResponse, 0, len(expedienteInitTypes))
	for value, label := range expedienteInitTypes {
		result = append(result, ExpedienteInitTypeResponse{Value: value, Label: label})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Value < result[j].Value })
	return result, nil
}

func (s *ExpedienteService) ListStatuses(context.Context) ([]ExpedienteStatusResponse, error) {
	return append([]ExpedienteStatusResponse(nil), expedienteStatuses...), nil
}

func expedienteResponse(e models.Expediente) ExpedienteResponse {
	var openingDate *Date
	if e.OpeningDate != nil {
		openingDate = &Date{Time: *e.OpeningDate}
	}
	return ExpedienteResponse{ID: e.ID, Name: e.Name, OpeningDate: openingDate, Number: e.Number, Code: e.Code, InitType: e.InitType, Status: e.Status, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}
