package models

import "time"

type Analysis struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	QuestionID  int64     `gorm:"not null;index" json:"questionId"`
	Provider    string    `gorm:"type:varchar(128);not null" json:"provider"`
	Answer      *string   `gorm:"type:varchar(255)" json:"answer,omitempty"`
	ContentJSON string    `gorm:"column:content_json;type:json;not null" json:"contentJson"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (Analysis) TableName() string {
	return "analyses"
}

type Job struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	JobID        string     `gorm:"column:job_id;type:varchar(64);uniqueIndex;not null" json:"jobId"`
	QuestionID   int64      `gorm:"not null;index" json:"questionId"`
	JobType      string     `gorm:"column:job_type;type:varchar(64);not null" json:"type"`
	Status       string     `gorm:"type:varchar(64);not null" json:"status"`
	ErrorCode    *string    `gorm:"column:error_code;type:varchar(128)" json:"errorCode,omitempty"`
	ErrorMessage *string    `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`
	StartedAt    *time.Time `gorm:"column:started_at" json:"startedAt,omitempty"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finishedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

func (Job) TableName() string {
	return "jobs"
}

type ChatMessage struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	QuestionID     int64     `gorm:"not null;index" json:"questionId"`
	Role           string    `gorm:"type:varchar(32);not null" json:"role"`
	Message        string    `gorm:"type:mediumtext;not null" json:"message"`
	AttachmentJSON *string   `gorm:"column:attachment_json;type:json" json:"attachmentJson,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}
