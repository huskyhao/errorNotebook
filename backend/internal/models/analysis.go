package models

import "time"

type Analysis struct {
	ID                            int64     `gorm:"primaryKey" json:"id"`
	UserID                        int64     `gorm:"column:user_id;not null;default:1;index" json:"-"`
	QuestionID                    int64     `gorm:"not null;index" json:"questionId"`
	JobID                         *string   `gorm:"column:job_id;type:varchar(64);uniqueIndex" json:"-"`
	Provider                      string    `gorm:"type:varchar(128);not null" json:"provider"`
	Answer                        *string   `gorm:"type:varchar(255)" json:"answer,omitempty"`
	ContentJSON                   string    `gorm:"column:content_json;type:json;not null" json:"contentJson"`
	SourceQuestionFingerprint     string    `gorm:"column:source_question_fingerprint;type:varchar(128)" json:"sourceQuestionFingerprint,omitempty"`
	TaxonomyCandidateSnapshotJSON string    `gorm:"column:taxonomy_candidate_snapshot_json;type:json" json:"-"`
	GeneratedAt                   time.Time `gorm:"column:generated_at" json:"generatedAt"`
	CreatedAt                     time.Time `json:"createdAt"`
}

func (Analysis) TableName() string {
	return "analyses"
}

type Job struct {
	ID              int64      `gorm:"primaryKey" json:"id"`
	UserID          int64      `gorm:"column:user_id;not null;default:1;index" json:"-"`
	JobID           string     `gorm:"column:job_id;type:varchar(64);uniqueIndex;not null" json:"jobId"`
	QuestionID      int64      `gorm:"not null;index" json:"questionId"`
	JobType         string     `gorm:"column:job_type;type:varchar(64);not null;index" json:"type"`
	Status          string     `gorm:"type:varchar(64);not null;index" json:"status"`
	Attempts        int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts     int        `gorm:"column:max_attempts;not null;default:3" json:"maxAttempts"`
	NextRunAt       *time.Time `gorm:"column:next_run_at;index" json:"nextRunAt,omitempty"`
	LockedAt        *time.Time `gorm:"column:locked_at;index" json:"lockedAt,omitempty"`
	ProcessingStage string     `gorm:"column:processing_stage;type:varchar(64)" json:"processingStage,omitempty"`
	ErrorCode       *string    `gorm:"column:error_code;type:varchar(128)" json:"errorCode,omitempty"`
	ErrorMessage    *string    `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`
	StartedAt       *time.Time `gorm:"column:started_at" json:"startedAt,omitempty"`
	FinishedAt      *time.Time `gorm:"column:finished_at" json:"finishedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (Job) TableName() string {
	return "jobs"
}

type ChatMessage struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	UserID         int64     `gorm:"column:user_id;not null;default:1;index" json:"-"`
	QuestionID     int64     `gorm:"not null;index" json:"questionId"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;type:varchar(128);uniqueIndex" json:"-"`
	Role           string    `gorm:"type:varchar(32);not null" json:"role"`
	Message        string    `gorm:"type:mediumtext;not null" json:"message"`
	AttachmentJSON *string   `gorm:"column:attachment_json;type:json" json:"attachmentJson,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}
