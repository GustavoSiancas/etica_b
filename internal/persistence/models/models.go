package models

import "time"

type Audit struct {
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

type Expediente struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	Description string     `gorm:"type:text;not null;default:(-)"`
	OpeningDate *time.Time `gorm:"type:date"`
	Number      string     `gorm:"type:varchar(50);not null;uniqueIndex;default:(-)"`
	InitType    *string    `gorm:"type:expediente_init_type_enum"`
	Audit
	Folders      []ExpedienteFolder      `gorm:"foreignKey:ExpedienteID"`
	Sections     []ExpedienteSection     `gorm:"foreignKey:ExpedienteID"`
	Denunciantes []ExpedienteDenunciante `gorm:"foreignKey:ExpedienteID"`
	Denunciados  []ExpedienteDenunciado  `gorm:"foreignKey:ExpedienteID"`
}

type Document struct {
	DocumentDate *time.Time `gorm:"type:timestamptz"`
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	Name         string     `gorm:"type:varchar(255);not null"`
	FileURL      *string    `gorm:"type:text"`
	Audit
}

type FolderType struct {
	Abbreviation *string `gorm:"type:varchar(20)"`
	ID           int64   `gorm:"primaryKey;autoIncrement"`
	Name         string
}

func (FolderType) TableName() string { return "folder_types" }

type ExpedienteFolder struct {
	Status       string     `gorm:"type:varchar(30);not null;default:ABIERTO"`
	Location     string     `gorm:"type:varchar(30);not null;default:DIRECCION DE ETICA"`
	OpeningDate  *time.Time `gorm:"type:timestamptz"`
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	ExpedienteID int64
	FolderTypeID int64
	Audit
}

func (ExpedienteFolder) TableName() string { return "expediente_folders" }

type ExpedienteSection struct {
	ExpedienteFolderID        int64 `gorm:"not null;default:(-)"`
	ID                        int64 `gorm:"primaryKey;autoIncrement"`
	ExpedienteID              int64 `gorm:"not null;index"`
	ExpedienteSectionParentID *int64
	Name                      string     `gorm:"type:text;not null"`
	Type                      string     `gorm:"type:text;not null"`
	DocumentDate              *time.Time `gorm:"type:date"`
	SortOrder                 int        `gorm:"not null"`
	ColorHex                  string     `gorm:"type:varchar(7);not null;default:#808080"`
	Audit
}

type ExpedienteSectionDocument struct {
	ExpedienteSectionID int64 `gorm:"primaryKey;autoIncrement:false"`
	DocumentID          int64 `gorm:"primaryKey;autoIncrement:false"`
}

type ExpedienteYearCounter struct {
	Year       int   `gorm:"primaryKey;autoIncrement:false"`
	LastNumber int64 `gorm:"not null"`
}

type Denunciante struct {
	ID     int64  `gorm:"primaryKey;autoIncrement"`
	Name   string `gorm:"type:varchar(255);not null"`
	IsDemo bool   `gorm:"not null;default:false"`
	Audit
	DocumentType   *string `gorm:"type:varchar(3)"`
	DocumentNumber *string `gorm:"type:varchar(20)"`
}

type Denunciado struct {
	ID     int64  `gorm:"primaryKey;autoIncrement"`
	Name   string `gorm:"type:varchar(255);not null"`
	IsDemo bool   `gorm:"not null;default:false"`
	Audit
	DocumentType       *string `gorm:"type:varchar(3)"`
	DocumentNumber     *string `gorm:"type:varchar(20)"`
	RegistrationNumber *int64  `gorm:"type:bigint"`
}

type ExpedienteDenunciante struct {
	ID            int64       `gorm:"primaryKey;autoIncrement"`
	ExpedienteID  int64       `gorm:"not null;uniqueIndex:uq_edte_member,priority:1"`
	DenuncianteID int64       `gorm:"not null;index;uniqueIndex:uq_edte_member,priority:2"`
	Expediente    Expediente  `gorm:"foreignKey:ExpedienteID"`
	Denunciante   Denunciante `gorm:"foreignKey:DenuncianteID"`
}

type ExpedienteDenunciado struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	ExpedienteID int64      `gorm:"not null;uniqueIndex:uq_eddo_member,priority:1"`
	DenunciadoID int64      `gorm:"not null;index;uniqueIndex:uq_eddo_member,priority:2"`
	Expediente   Expediente `gorm:"foreignKey:ExpedienteID"`
	Denunciado   Denunciado `gorm:"foreignKey:DenunciadoID"`
}

func (Expediente) TableName() string            { return "expedientes" }
func (Document) TableName() string              { return "documents" }
func (Denunciante) TableName() string           { return "denunciantes" }
func (Denunciado) TableName() string            { return "denunciados" }
func (ExpedienteDenunciante) TableName() string { return "expediente_denunciantes" }
func (ExpedienteDenunciado) TableName() string  { return "expediente_denunciados" }

type DocumentType struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Code      string `gorm:"type:varchar(70);not null;uniqueIndex"`
	Name      string `gorm:"type:varchar(150);not null"`
	SortOrder int    `gorm:"not null"`
	IsActive  bool   `gorm:"not null;default:true"`
}

func (DocumentType) TableName() string { return "document_types" }

func (ExpedienteSection) TableName() string         { return "expediente_section" }
func (ExpedienteSectionDocument) TableName() string { return "expediente_section_documents" }
func (ExpedienteYearCounter) TableName() string     { return "expediente_year_counters" }
