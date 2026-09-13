package models

import "time"

type Question struct {
	// CategoryID is the single stable, top-level subject classification for a question.
	ID                  int64            `gorm:"primaryKey" json:"id"`
	CategoryID          *int64           `gorm:"index" json:"categoryId,omitempty"`
	Category            *Category        `gorm:"foreignKey:CategoryID" json:"-"`
	Stem                string           `gorm:"type:text;not null" json:"stem"`
	QuestionType        string           `gorm:"type:varchar(64);not null" json:"questionType"`
	CorrectAnswer       *string          `gorm:"type:text" json:"correctAnswer,omitempty"`
	UserAnswer          *string          `gorm:"type:text" json:"userAnswer,omitempty"`
	OCRStatus           string           `gorm:"column:ocr_status;type:varchar(64);not null;default:'uploaded'" json:"ocrStatus"`
	AnalysisStatus      string           `gorm:"column:analysis_status;type:varchar(64);not null;default:'queued'" json:"analysisStatus"`
	SourceType          string           `gorm:"type:varchar(64);not null" json:"sourceType"`
	RawOCRText          *string          `gorm:"column:raw_ocr_text;type:longtext" json:"rawOcrText,omitempty"`
	StructureWarnings   *string          `gorm:"column:structure_warnings;type:json" json:"-"`
	StructureConfidence *float64         `gorm:"column:structure_confidence" json:"structureConfidence,omitempty"`
	ParseSource         string           `gorm:"column:parse_source;type:varchar(64);not null;default:'rules'" json:"parseSource"`
	DiagramDescription  *string          `gorm:"column:diagram_description;type:text" json:"diagramDescription,omitempty"`
	HasDiagram          bool             `gorm:"column:has_diagram;not null;default:false" json:"hasDiagram"`
	ImagePath           *string          `gorm:"column:image_path;type:varchar(1024)" json:"imagePath,omitempty"`
	IsFavorited         bool             `gorm:"column:is_favorited;not null;default:false;index" json:"isFavorited"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
	Options             []QuestionOption `gorm:"foreignKey:QuestionID" json:"options,omitempty"`
	Assets              []QuestionAsset  `gorm:"foreignKey:QuestionID" json:"assets,omitempty"`
	Tags                []Tag            `gorm:"many2many:question_tags;foreignKey:ID;joinForeignKey:QuestionID;references:ID;joinReferences:TagID" json:"tags,omitempty"`
}

func (Question) TableName() string {
	return "questions"
}

type QuestionOption struct {
	ID         int64  `gorm:"primaryKey" json:"id"`
	QuestionID int64  `gorm:"not null;index" json:"questionId"`
	OptionKey  string `gorm:"column:option_key;type:varchar(16);not null" json:"key"`
	Content    string `gorm:"type:text;not null" json:"content"`
	SortOrder  int    `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

func (QuestionOption) TableName() string {
	return "question_options"
}

type QuestionAsset struct {
	ID         int64  `gorm:"primaryKey" json:"id"`
	QuestionID int64  `gorm:"not null;index" json:"questionId"`
	AssetType  string `gorm:"column:asset_type;type:varchar(64);not null" json:"assetType"`
	FileURL    string `gorm:"column:file_url;type:varchar(1024);not null" json:"fileUrl"`
	MetaJSON   string `gorm:"column:meta_json;type:json" json:"metaJson"`
}

func (QuestionAsset) TableName() string {
	return "question_assets"
}
