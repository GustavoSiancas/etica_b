package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"etica_back/internal/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrExpedienteInvalid = errors.New("invalid expediente request")
var ErrExpedienteClosed = errors.New("expediente is closed; reopen it before modifying information")

const (
	ExpedienteAbierto  = "ABIERTO"
	ExpedienteCerrado  = "CERRADO"
	UbicacionDireccion = "DIRECCION DE ETICA"
	UbicacionConsejo   = "CONSEJO DE ETICA"
	UbicacionTribunal  = "TRIBUNAL DE HONOR"
)

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
	DenuncianteIDs []int64 `json:"denunciante_ids"`
	DenunciadoIDs  []int64 `json:"denunciado_ids"`
	Description    string  `json:"description"`
	OpeningDate    *Date   `json:"opening_date"`
	InitType       string  `json:"init_type"`
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
	Description string    `json:"description"`
	OpeningDate *Date     `json:"opening_date"`
	Number      string    `json:"number"`
	InitType    *string   `json:"init_type"`
	Status      string    `json:"status"`
	Location    string    `json:"location"`
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
	{Value: ExpedienteAbierto, Label: "Abierto"},
	{Value: ExpedienteCerrado, Label: "Cerrado"},
}

type ExpedienteLocationResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var expedienteLocations = []ExpedienteLocationResponse{
	{Value: UbicacionDireccion, Label: "Direccion de Etica"},
	{Value: UbicacionConsejo, Label: "Consejo de Etica"},
	{Value: UbicacionTribunal, Label: "Tribunal de Honor"},
}

func validLocation(value string) bool {
	return value == UbicacionDireccion || value == UbicacionConsejo || value == UbicacionTribunal
}

func (s *ExpedienteService) Create(ctx context.Context, input CreateExpedienteInput) (ExpedienteResponse, error) {
	input.Description = strings.TrimSpace(input.Description)
	if !utf8.ValidString(input.Description) || strings.ContainsRune(input.Description, 0) || input.OpeningDate == nil || input.OpeningDate.Year() < 1 || input.OpeningDate.Year() > 9999 || !ValidExpedienteInitType(input.InitType) || !validParticipantIDs(input.DenuncianteIDs) || !validParticipantIDs(input.DenunciadoIDs) {
		return ExpedienteResponse{}, ErrExpedienteInvalid
	}
	initType := input.InitType
	var openingDate *time.Time
	if input.OpeningDate != nil {
		parsed := input.OpeningDate.Time
		openingDate = &parsed
	}
	expediente := models.Expediente{Description: input.Description, OpeningDate: openingDate, Status: ExpedienteAbierto, Location: UbicacionDireccion, InitType: &initType}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(input.DenuncianteIDs) > 0 {
			var count int64
			if err := tx.Model(&models.Denunciante{}).Where("id IN ?", input.DenuncianteIDs).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(input.DenuncianteIDs)) {
				return ErrParticipantNotFound
			}
		}
		if len(input.DenunciadoIDs) > 0 {
			var count int64
			if err := tx.Model(&models.Denunciado{}).Where("id IN ?", input.DenunciadoIDs).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(input.DenunciadoIDs)) {
				return ErrParticipantNotFound
			}
		}
		if err := tx.Create(&expediente).Error; err != nil {
			if duplicate(err) {
				return ErrExpedienteDuplicate
			}
			return err
		}
		if len(input.DenuncianteIDs) > 0 {
			links := make([]models.ExpedienteDenunciante, 0, len(input.DenuncianteIDs))
			for _, id := range input.DenuncianteIDs {
				links = append(links, models.ExpedienteDenunciante{ExpedienteID: expediente.ID, DenuncianteID: id})
			}
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}
		if len(input.DenunciadoIDs) > 0 {
			links := make([]models.ExpedienteDenunciado, 0, len(input.DenunciadoIDs))
			for _, id := range input.DenunciadoIDs {
				links = append(links, models.ExpedienteDenunciado{ExpedienteID: expediente.ID, DenunciadoID: id})
			}
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ExpedienteResponse{}, err
	}

	return expedienteResponse(expediente), nil
}

type ExpedienteFilters struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Status   string `json:"status"`
	Location string `json:"location"`
}

func ValidExpedienteFilters(f ExpedienteFilters) bool {
	for _, value := range []string{f.From, f.To} {
		if value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Year() < 1 {
				return false
			}
		}
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		return false
	}
	return (f.Status == "" || f.Status == ExpedienteAbierto || f.Status == ExpedienteCerrado) && (f.Location == "" || validLocation(f.Location))
}
func (s *ExpedienteService) List(ctx context.Context, limit, offset int, filters ...ExpedienteFilters) (ExpedienteList, error) {
	if len(filters) > 1 {
		return ExpedienteList{}, ErrExpedienteInvalid
	}
	var filter ExpedienteFilters
	if len(filters) == 1 {
		filter = filters[0]
	}
	return s.listExpedientes(ctx, limit, offset, filter, "", 0)
}
func (s *ExpedienteService) ListByDenunciante(ctx context.Context, id int64, limit, offset int, filter ExpedienteFilters) (ExpedienteList, error) {
	return s.listExpedientes(ctx, limit, offset, filter, "denunciante", id)
}
func (s *ExpedienteService) ListByDenunciado(ctx context.Context, id int64, limit, offset int, filter ExpedienteFilters) (ExpedienteList, error) {
	return s.listExpedientes(ctx, limit, offset, filter, "denunciado", id)
}
func (s *ExpedienteService) listExpedientes(ctx context.Context, limit, offset int, filter ExpedienteFilters, role string, participantID int64) (ExpedienteList, error) {
	result := ExpedienteList{Data: []ExpedienteResponse{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 || !ValidExpedienteFilters(filter) {
		return result, ErrExpedienteInvalid
	}
	if role != "" && participantID <= 0 {
		return result, ErrParticipantInvalid
	}
	db := s.db.WithContext(ctx)
	query := db.Model(&models.Expediente{})
	if role != "" {
		var model any
		var scope string
		switch role {
		case "denunciante":
			model = &models.Denunciante{}
			scope = "id IN (SELECT expediente_id FROM expediente_denunciantes WHERE denunciante_id = ?)"
		case "denunciado":
			model = &models.Denunciado{}
			scope = "id IN (SELECT expediente_id FROM expediente_denunciados WHERE denunciado_id = ?)"
		default:
			return result, ErrParticipantInvalid
		}
		var count int64
		if err := db.Model(model).Where("id = ?", participantID).Count(&count).Error; err != nil {
			return result, err
		}
		if count == 0 {
			return result, ErrParticipantNotFound
		}
		query = query.Where(scope, participantID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Location != "" {
		query = query.Where("location = ?", filter.Location)
	}
	if filter.From != "" {
		query = query.Where("opening_date >= ?", filter.From)
	}
	if filter.To != "" {
		query = query.Where("opening_date <= ?", filter.To)
	}
	if err := query.Count(&result.Total).Error; err != nil {
		return result, err
	}
	var rows []models.Expediente
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
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
	if id <= 0 || (status != ExpedienteAbierto && status != ExpedienteCerrado) {
		return ExpedienteResponse{}, ErrExpedienteInvalid
	}
	var row models.Expediente
	// A row lock serializes close/reopen against all expediente mutations.
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExpedienteNotFound
		}
		if err != nil {
			return err
		}
		return tx.Model(&row).Update("status", status).Error
	})
	return expedienteResponse(row), err
}

type UpdateExpedienteLocationInput struct {
	Location string `json:"location"`
}

func (s *ExpedienteService) UpdateLocation(ctx context.Context, id int64, input UpdateExpedienteLocationInput) (ExpedienteResponse, error) {
	location := strings.TrimSpace(input.Location)
	if id <= 0 || !validLocation(location) {
		return ExpedienteResponse{}, ErrExpedienteInvalid
	}
	var row models.Expediente
	// Location is administrative and remains editable while closed. Updating
	// only this column preserves status and leaves all content guards intact.
	result := s.db.WithContext(ctx).Model(&row).Clauses(clause.Returning{}).Where("id = ?", id).Update("location", location)
	if result.Error != nil {
		return ExpedienteResponse{}, result.Error
	}
	if result.RowsAffected == 0 {
		return ExpedienteResponse{}, ErrExpedienteNotFound
	}
	return expedienteResponse(row), nil
}

func (s *ExpedienteService) ListLocations(context.Context) ([]ExpedienteLocationResponse, error) {
	return append([]ExpedienteLocationResponse(nil), expedienteLocations...), nil
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
	return ExpedienteResponse{ID: e.ID, Description: e.Description, OpeningDate: openingDate, Number: e.Number, InitType: e.InitType, Status: e.Status, Location: e.Location, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}
