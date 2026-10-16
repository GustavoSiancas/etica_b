package models

import "time"

type Audit struct {
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

type Expediente struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	Name        string     `gorm:"type:varchar(255);not null;default:(-)"`
	OpeningDate *time.Time `gorm:"type:date"`
	Number      string     `gorm:"type:varchar(50);not null;uniqueIndex;default:(-)"`
	Code        string     `gorm:"type:varchar(50);not null;uniqueIndex;default:(-)"`
	Status      string     `gorm:"type:varchar(30);not null;default:OPEN"`
	InitType    *string    `gorm:"type:expediente_init_type_enum"`
	Audit
	Documents          []Document                     `gorm:"foreignKey:ExpedienteID"`
	Denunciantes       []ExpedienteDenunciante        `gorm:"foreignKey:ExpedienteID"`
	Denunciados        []ExpedienteDenunciado         `gorm:"foreignKey:ExpedienteID"`
	ParticipantHistory []ExpedienteParticipantHistory `gorm:"foreignKey:ExpedienteID"`
}

type DocumentStage struct {
	ID        int64              `gorm:"primaryKey;autoIncrement"`
	Code      string             `gorm:"type:varchar(60);not null;uniqueIndex"`
	Name      string             `gorm:"type:varchar(120);not null"`
	SortOrder int                `gorm:"not null"`
	IsActive  bool               `gorm:"not null;default:true"`
	Substages []DocumentSubstage `gorm:"foreignKey:StageID"`
}

type DocumentSubstage struct {
	ID        int64         `gorm:"primaryKey;autoIncrement"`
	StageID   int64         `gorm:"not null;index"`
	Code      string        `gorm:"type:varchar(70);not null;uniqueIndex"`
	Name      string        `gorm:"type:varchar(150);not null"`
	SortOrder int           `gorm:"not null"`
	IsActive  bool          `gorm:"not null;default:true"`
	Stage     DocumentStage `gorm:"foreignKey:StageID"`
	Documents []Document    `gorm:"foreignKey:SubstageID"`
}

type Document struct {
	ID           int64      `gorm:"primaryKey;autoIncrement;uniqueIndex:uq_documents_expediente,priority:1"`
	ExpedienteID int64      `gorm:"not null;index;uniqueIndex:uq_documents_expediente,priority:2"`
	Name         string     `gorm:"type:varchar(255);not null"`
	FileURL      *string    `gorm:"type:text"`
	DocumentDate *time.Time `gorm:"type:date"`
	SubstageID   int64      `gorm:"not null;index"`
	Status       string     `gorm:"type:varchar(30);not null;default:DRAFT"`
	Audit
	Expediente   Expediente            `gorm:"foreignKey:ExpedienteID"`
	Substage     DocumentSubstage      `gorm:"foreignKey:SubstageID"`
	Denunciantes []DocumentDenunciante `gorm:"foreignKey:DocumentID,ExpedienteID;references:ID,ExpedienteID"`
	Denunciados  []DocumentDenunciado  `gorm:"foreignKey:DocumentID,ExpedienteID;references:ID,ExpedienteID"`
}

type Escrito struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	Title            string     `gorm:"type:varchar(255);not null"`
	Type             string     `gorm:"type:varchar(100);not null"`
	ExpedienteNumber string     `gorm:"type:varchar(50);not null;index"`
	DocumentDate     *time.Time `gorm:"type:date"`
	FileURL          *string    `gorm:"type:text"`
	Status           string     `gorm:"type:varchar(20);not null;default:NUEVA"`
	Audit
	Denunciantes []EscritoDenunciante `gorm:"foreignKey:EscritoID"`
	Denunciados  []EscritoDenunciado  `gorm:"foreignKey:EscritoID"`
}

type EscritoDenunciante struct {
	EscritoID     int64       `gorm:"primaryKey;autoIncrement:false"`
	DenuncianteID int64       `gorm:"primaryKey;autoIncrement:false"`
	Escrito       Escrito     `gorm:"foreignKey:EscritoID"`
	Denunciante   Denunciante `gorm:"foreignKey:DenuncianteID"`
}

type EscritoDenunciado struct {
	EscritoID    int64      `gorm:"primaryKey;autoIncrement:false"`
	DenunciadoID int64      `gorm:"primaryKey;autoIncrement:false"`
	Escrito      Escrito    `gorm:"foreignKey:EscritoID"`
	Denunciado   Denunciado `gorm:"foreignKey:DenunciadoID"`
}

type Denunciante struct {
	ID   int64  `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"type:varchar(255);not null"`
	Audit
}

type Denunciado struct {
	ID   int64  `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"type:varchar(255);not null"`
	Audit
}

type ExpedienteDenunciante struct {
	ID            int64       `gorm:"primaryKey;autoIncrement;uniqueIndex:uq_edte_scope,priority:1"`
	ExpedienteID  int64       `gorm:"not null;uniqueIndex:uq_edte_member,priority:1;uniqueIndex:uq_edte_scope,priority:2"`
	DenuncianteID int64       `gorm:"not null;index;uniqueIndex:uq_edte_member,priority:2"`
	IsActive      bool        `gorm:"not null;default:true"`
	Expediente    Expediente  `gorm:"foreignKey:ExpedienteID"`
	Denunciante   Denunciante `gorm:"foreignKey:DenuncianteID"`
}

type ExpedienteDenunciado struct {
	ID           int64      `gorm:"primaryKey;autoIncrement;uniqueIndex:uq_eddo_scope,priority:1"`
	ExpedienteID int64      `gorm:"not null;uniqueIndex:uq_eddo_member,priority:1;uniqueIndex:uq_eddo_scope,priority:2"`
	DenunciadoID int64      `gorm:"not null;index;uniqueIndex:uq_eddo_member,priority:2"`
	IsActive     bool       `gorm:"not null;default:true"`
	Expediente   Expediente `gorm:"foreignKey:ExpedienteID"`
	Denunciado   Denunciado `gorm:"foreignKey:DenunciadoID"`
}

type DocumentDenunciante struct {
	DocumentID              int64                 `gorm:"primaryKey;autoIncrement:false"`
	ExpedienteDenuncianteID int64                 `gorm:"primaryKey;autoIncrement:false;index"`
	ExpedienteID            int64                 `gorm:"not null;index"`
	Document                Document              `gorm:"foreignKey:DocumentID,ExpedienteID;references:ID,ExpedienteID"`
	Participant             ExpedienteDenunciante `gorm:"foreignKey:ExpedienteDenuncianteID,ExpedienteID;references:ID,ExpedienteID"`
}

type DocumentDenunciado struct {
	DocumentID             int64                `gorm:"primaryKey;autoIncrement:false"`
	ExpedienteDenunciadoID int64                `gorm:"primaryKey;autoIncrement:false;index"`
	ExpedienteID           int64                `gorm:"not null;index"`
	Document               Document             `gorm:"foreignKey:DocumentID,ExpedienteID;references:ID,ExpedienteID"`
	Participant            ExpedienteDenunciado `gorm:"foreignKey:ExpedienteDenunciadoID,ExpedienteID;references:ID,ExpedienteID"`
}

type ExpedienteParticipantHistory struct {
	ID              int64   `gorm:"primaryKey;autoIncrement"`
	ExpedienteID    int64   `gorm:"not null;index"`
	ParticipantType string  `gorm:"type:varchar(20);not null"`
	ParticipantID   int64   `gorm:"not null;index"`
	Action          string  `gorm:"type:varchar(30);not null"`
	Reason          *string `gorm:"type:text"`
	CreatedBy       *int64
	CreatedAt       time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	PreviousName    *string    `gorm:"type:varchar(255)"`
	NewName         *string    `gorm:"type:varchar(255)"`
	DenuncianteID   *int64     `gorm:"->;type:bigint"`
	DenunciadoID    *int64     `gorm:"->;type:bigint"`
	Expediente      Expediente `gorm:"foreignKey:ExpedienteID"`
}

func (Expediente) TableName() string                   { return "expedientes" }
func (DocumentStage) TableName() string                { return "document_stages" }
func (DocumentSubstage) TableName() string             { return "document_substages" }
func (Document) TableName() string                     { return "documents" }
func (Escrito) TableName() string                      { return "escritos" }
func (EscritoDenunciante) TableName() string           { return "escrito_denunciantes" }
func (EscritoDenunciado) TableName() string            { return "escrito_denunciados" }
func (Denunciante) TableName() string                  { return "denunciantes" }
func (Denunciado) TableName() string                   { return "denunciados" }
func (ExpedienteDenunciante) TableName() string        { return "expediente_denunciantes" }
func (ExpedienteDenunciado) TableName() string         { return "expediente_denunciados" }
func (DocumentDenunciante) TableName() string          { return "document_denunciantes" }
func (DocumentDenunciado) TableName() string           { return "document_denunciados" }
func (ExpedienteParticipantHistory) TableName() string { return "expediente_participant_history" }
