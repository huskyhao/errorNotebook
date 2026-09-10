package models

import "time"

type QuestionLearningState struct {
	ID              int64      `gorm:"primaryKey" json:"id"`
	UserID          int64      `gorm:"column:user_id;not null;default:1;index" json:"-"`
	QuestionID      int64      `gorm:"column:question_id;not null;uniqueIndex" json:"questionId"`
	Question        Question   `gorm:"foreignKey:QuestionID" json:"-"`
	MasteryLevel    int        `gorm:"column:mastery_level;not null;default:0" json:"masteryLevel"`
	WrongCount      int        `gorm:"column:wrong_count;not null;default:0" json:"wrongCount"`
	CorrectStreak   int        `gorm:"column:correct_streak;not null;default:0" json:"correctStreak"`
	LastPracticedAt *time.Time `gorm:"column:last_practiced_at" json:"lastPracticedAt,omitempty"`
	NextReviewAt    *time.Time `gorm:"column:next_review_at;index" json:"nextReviewAt,omitempty"`
	MistakeReason   *string    `gorm:"column:mistake_reason;type:text" json:"mistakeReason,omitempty"`
	WeaknessTags    string     `gorm:"column:weakness_tags;type:json" json:"-"`
	ReviewAdvice    string     `gorm:"column:review_advice;type:json" json:"-"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (QuestionLearningState) TableName() string {
	return "question_learning_states"
}
