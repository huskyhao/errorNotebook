package models

import "time"

type BatchImport struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	UserID     int64     `gorm:"not null;default:1;index" json:"userId"`
	TotalFiles int       `gorm:"not null" json:"totalFiles"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (BatchImport) TableName() string {
	return "batch_imports"
}

type BatchImportItem struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	BatchID         int64     `gorm:"not null;index" json:"batchId"`
	QuestionID      int64     `gorm:"not null;index" json:"questionId"`
	FileIndex       int       `gorm:"not null" json:"fileIndex"`
	FileName        string    `gorm:"type:varchar(255)" json:"fileName"`
	Status          string    `gorm:"type:varchar(32);not null;default:'pending'" json:"status"`
	ProcessingStage string    `gorm:"column:processing_stage;type:varchar(64);not null;default:'pending'" json:"processingStage"`
	ErrorMsg        *string   `gorm:"type:text" json:"errorMsg,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (BatchImportItem) TableName() string {
	return "batch_import_items"
}
