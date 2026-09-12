package models

import "time"

// AIProposal is an advisory, versioned result. It is never a source of truth
// until a Go-owned confirmation transaction applies it.
type AIProposal struct {
	ID                int64      `gorm:"primaryKey" json:"id"`
	ProposalID        string     `gorm:"column:proposal_id;type:varchar(128);uniqueIndex;not null" json:"proposalId"`
	QuestionID        int64      `gorm:"column:question_id;not null;index" json:"questionId"`
	Action            string     `gorm:"type:varchar(64);not null;index" json:"action"`
	Status            string     `gorm:"type:varchar(32);not null;default:'pending';index" json:"status"`
	ContentJSON       string     `gorm:"column:content_json;type:json;not null" json:"content"`
	SourceFingerprint string     `gorm:"column:source_fingerprint;type:varchar(128);not null" json:"sourceFingerprint"`
	IdempotencyKey    string     `gorm:"column:idempotency_key;type:varchar(255);uniqueIndex;not null" json:"-"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;index;not null" json:"expiresAt"`
	CreatedQuestionID *int64     `gorm:"column:created_question_id" json:"createdQuestionId,omitempty"`
	ConfirmedAt       *time.Time `gorm:"column:confirmed_at" json:"confirmedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (AIProposal) TableName() string { return "ai_proposals" }
