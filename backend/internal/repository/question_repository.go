package repository

import (
	"errors"
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type QuestionRepository struct {
	db *gorm.DB
}

func NewQuestionRepository(db *gorm.DB) *QuestionRepository {
	return &QuestionRepository{db: db}
}

func (r *QuestionRepository) Create(question *models.Question) error {
	if err := r.db.Create(question).Error; err != nil {
		return fmt.Errorf("create question: %w", err)
	}
	return nil
}

func (r *QuestionRepository) CreateOptions(options []models.QuestionOption) error {
	if len(options) == 0 {
		return nil
	}
	if err := r.db.Create(&options).Error; err != nil {
		return fmt.Errorf("create question options: %w", err)
	}
	return nil
}

func (r *QuestionRepository) ReplaceOptions(questionID int64, options []models.QuestionOption) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", questionID).Delete(&models.QuestionOption{}).Error; err != nil {
			return fmt.Errorf("delete question options: %w", err)
		}
		if len(options) == 0 {
			return nil
		}
		if err := tx.Create(&options).Error; err != nil {
			return fmt.Errorf("recreate question options: %w", err)
		}
		return nil
	})
}

func (r *QuestionRepository) CreateAssets(assets []models.QuestionAsset) error {
	if len(assets) == 0 {
		return nil
	}
	if err := r.db.Create(&assets).Error; err != nil {
		return fmt.Errorf("create question assets: %w", err)
	}
	return nil
}

func (r *QuestionRepository) ReplaceAssets(questionID int64, assets []models.QuestionAsset) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", questionID).Delete(&models.QuestionAsset{}).Error; err != nil {
			return fmt.Errorf("delete question assets: %w", err)
		}
		if len(assets) == 0 {
			return nil
		}
		if err := tx.Create(&assets).Error; err != nil {
			return fmt.Errorf("recreate question assets: %w", err)
		}
		return nil
	})
}

func (r *QuestionRepository) GetByID(id int64) (*models.Question, error) {
	var question models.Question
	if err := r.db.Preload("Options").Preload("Assets").Preload("Tags").Preload("Category").First(&question, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get question by id: %w", err)
	}
	return &question, nil
}

func (r *QuestionRepository) Update(question *models.Question) error {
	if err := r.db.Omit("Options", "Assets", "Tags", "Category").Save(question).Error; err != nil {
		return fmt.Errorf("update question: %w", err)
	}
	return nil
}

func (r *QuestionRepository) Delete(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", id).Delete(&models.QuestionOption{}).Error; err != nil {
			return fmt.Errorf("delete question options: %w", err)
		}
		if err := tx.Where("question_id = ?", id).Delete(&models.QuestionAsset{}).Error; err != nil {
			return fmt.Errorf("delete question assets: %w", err)
		}
		if err := tx.Where("question_id = ?", id).Delete(&models.Analysis{}).Error; err != nil {
			return fmt.Errorf("delete analyses: %w", err)
		}
		if err := tx.Where("question_id = ?", id).Delete(&models.ChatMessage{}).Error; err != nil {
			return fmt.Errorf("delete chat messages: %w", err)
		}
		if err := tx.Where("question_id = ?", id).Delete(&models.Job{}).Error; err != nil {
			return fmt.Errorf("delete jobs: %w", err)
		}
		if err := tx.Where("question_id = ?", id).Delete(&models.QuestionTag{}).Error; err != nil {
			return fmt.Errorf("delete question tags: %w", err)
		}
		if err := tx.Where("question_id = ?", id).Delete(&models.QuestionLearningState{}).Error; err != nil {
			return fmt.Errorf("delete learning state: %w", err)
		}
		if err := tx.Delete(&models.Question{}, id).Error; err != nil {
			return fmt.Errorf("delete question: %w", err)
		}
		return nil
	})
}

func (r *QuestionRepository) List(filters QuestionListFilters) ([]models.Question, error) {
	query := r.db.Model(&models.Question{}).Preload("Options").Preload("Assets").Preload("Tags").Preload("Category").Order("id desc")

	if filters.Uncategorized {
		query = query.Where("category_id IS NULL")
	} else if filters.CategoryID != nil {
		query = query.Where("category_id = ?", *filters.CategoryID)
	}
	if filters.IsFavorited != nil {
		query = query.Where("is_favorited = ?", *filters.IsFavorited)
	}
	if len(filters.TagIDs) > 0 {
		sub := r.db.Table("question_tags").
			Select("question_id").
			Where("tag_id IN ?", filters.TagIDs).
			Group("question_id").
			Having("COUNT(DISTINCT tag_id) = ?", len(filters.TagIDs))
		query = query.Where("id IN (?)", sub)
	}
	if filters.QuestionType != "" {
		query = query.Where("question_type = ?", filters.QuestionType)
	}
	if filters.OCRStatus != "" {
		query = query.Where("ocr_status = ?", filters.OCRStatus)
	}
	if filters.AnalysisStatus != "" {
		query = query.Where("analysis_status = ?", filters.AnalysisStatus)
	}
	if filters.Keyword != "" {
		query = query.Where("stem LIKE ?", "%"+filters.Keyword+"%")
	}

	var questions []models.Question
	if err := query.Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	return questions, nil
}

type QuestionListFilters struct {
	CategoryID     *int64
	Uncategorized  bool
	IsFavorited    *bool
	TagIDs         []int64
	QuestionType   string
	OCRStatus      string
	AnalysisStatus string
	Keyword        string
}

func (r *QuestionRepository) CountByCategory() (map[int64]int64, error) {
	type row struct {
		CategoryID int64
		Count      int64
	}
	var rows []row
	if err := r.db.Model(&models.Question{}).
		Select("category_id, COUNT(*) as count").
		Where("category_id IS NOT NULL").
		Group("category_id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("count by category: %w", err)
	}
	result := make(map[int64]int64, len(rows))
	for _, r := range rows {
		result[r.CategoryID] = r.Count
	}
	return result, nil
}

func (r *QuestionRepository) CountUncategorized() (int64, error) {
	var count int64
	if err := r.db.Model(&models.Question{}).Where("category_id IS NULL").Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count uncategorized: %w", err)
	}
	return count, nil
}

func (r *QuestionRepository) SetTags(questionID int64, tagIDs []int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", questionID).Delete(&models.QuestionTag{}).Error; err != nil {
			return fmt.Errorf("delete question tags: %w", err)
		}
		for _, tagID := range tagIDs {
			if err := tx.Create(&models.QuestionTag{QuestionID: questionID, TagID: tagID}).Error; err != nil {
				return fmt.Errorf("insert question tag: %w", err)
			}
		}
		return nil
	})
}
