package models

import "time"

type Category struct {
	// ParentID remains for compatibility with legacy rows; new categories are top-level subjects.
	ID        int64     `gorm:"primaryKey" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null;default:0;index:idx_categories_user_name,priority:1" json:"-"`
	Name      string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_categories_user_name,priority:2" json:"name"`
	ParentID  *int64    `gorm:"column:parent_id;index" json:"parentId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (Category) TableName() string {
	return "categories"
}

type Tag struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null;default:0;index:idx_tags_user_name,priority:1" json:"-"`
	Name      string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_tags_user_name,priority:2" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (Tag) TableName() string {
	return "tags"
}

type QuestionTag struct {
	QuestionID int64 `gorm:"primaryKey;column:question_id" json:"questionId"`
	TagID      int64 `gorm:"primaryKey;column:tag_id" json:"tagId"`
}

func (QuestionTag) TableName() string {
	return "question_tags"
}
