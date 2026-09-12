package models

import "time"

type PracticeSession struct {
	ID           int64                     `gorm:"primaryKey" json:"id"`
	Name         string                    `gorm:"type:varchar(255);not null" json:"name"`
	Status       string                    `gorm:"type:varchar(32);not null;default:'in_progress'" json:"status"`
	TotalCount   int                       `gorm:"not null;default:0" json:"totalCount"`
	CorrectCount int                       `gorm:"not null;default:0" json:"correctCount"`
	CreatedAt    time.Time                 `json:"createdAt"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
	Questions    []PracticeSessionQuestion `gorm:"foreignKey:SessionID" json:"questions,omitempty"`
}

func (PracticeSession) TableName() string {
	return "practice_sessions"
}

type PracticeSessionQuestion struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	SessionID      int64     `gorm:"not null;index" json:"sessionId"`
	QuestionID     int64     `gorm:"not null;index" json:"questionId"`
	OrderIndex     int       `gorm:"not null" json:"orderIndex"`
	UserAnswer     *string   `gorm:"type:text" json:"userAnswer,omitempty"`
	IsCorrect      *bool     `json:"isCorrect,omitempty"`
	Status         string    `gorm:"type:varchar(32);not null;default:'unanswered'" json:"status"`
	Score          *float64  `gorm:"column:score" json:"score,omitempty"`
	MaxScore       *float64  `gorm:"column:max_score" json:"maxScore,omitempty"`
	GradingStatus  string    `gorm:"column:grading_status;type:varchar(32);not null;default:'ungraded'" json:"gradingStatus"`
	GradingComment *string   `gorm:"column:grading_comment;type:text" json:"gradingComment,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Question       Question  `gorm:"foreignKey:QuestionID" json:"question,omitempty"`
}

func (PracticeSessionQuestion) TableName() string {
	return "practice_session_questions"
}
