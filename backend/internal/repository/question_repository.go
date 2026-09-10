package repository

import (
	"errors"
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

var ErrTagNotFound = errors.New("one or more tag ids do not exist")

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

func (r *QuestionRepository) GetByIDForUser(id, userID int64) (*models.Question, error) {
	var question models.Question
	if err := r.db.Where("user_id = ?", userID).Preload("Options").Preload("Assets", "user_id = ?", userID).Preload("Tags").Preload("Category", "user_id = 0 OR user_id = ?", userID).First(&question, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get question by owner: %w", err)
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
		// A question can be referenced by one or more practice sessions. These
		// links must be removed before deleting the question because the existing
		// foreign key intentionally prevents deleting a referenced question.
		if err := tx.Where("question_id = ?", id).Delete(&models.PracticeSessionQuestion{}).Error; err != nil {
			return fmt.Errorf("delete practice session questions: %w", err)
		}
		if err := tx.Delete(&models.Question{}, id).Error; err != nil {
			return fmt.Errorf("delete question: %w", err)
		}
		return nil
	})
}

func (r *QuestionRepository) List(filters QuestionListFilters) ([]models.Question, error) {
	query := r.db.Model(&models.Question{}).Preload("Options").Preload("Tags").Order("id desc")
	if filters.UserID != nil {
		userID := *filters.UserID
		query = query.Where("questions.user_id = ?", userID).Preload("Assets", "user_id = ?", userID).Preload("Category", "user_id = 0 OR user_id = ?", userID)
	} else {
		query = query.Preload("Assets").Preload("Category")
	}

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

func (r *QuestionRepository) ListForUser(filters QuestionListFilters, userID int64) ([]models.Question, error) {
	filters.UserID = &userID
	return r.List(filters)
}

type QuestionListFilters struct {
	UserID         *int64
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
	return r.CountByCategoryForUser(0)
}

func (r *QuestionRepository) CountByCategoryForUser(userID int64) (map[int64]int64, error) {
	type row struct {
		CategoryID int64
		Count      int64
	}
	var rows []row
	query := r.db.Model(&models.Question{}).
		Select("category_id, COUNT(*) as count").
		Where("category_id IS NOT NULL").
		Group("category_id")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("count by category: %w", err)
	}
	result := make(map[int64]int64, len(rows))
	for _, r := range rows {
		result[r.CategoryID] = r.Count
	}
	return result, nil
}

func (r *QuestionRepository) CountUncategorized() (int64, error) {
	return r.CountUncategorizedForUser(0)
}

func (r *QuestionRepository) CountUncategorizedForUser(userID int64) (int64, error) {
	var count int64
	query := r.db.Model(&models.Question{}).Where("category_id IS NULL")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count uncategorized: %w", err)
	}
	return count, nil
}

func (r *QuestionRepository) DeleteForUser(id, userID int64) error {
	var question models.Question
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return r.Delete(id)
}

func (r *QuestionRepository) SetTags(questionID int64, tagIDs []int64, userIDs ...int64) error {
	userID := int64(0)
	if len(userIDs) > 0 {
		userID = userIDs[0]
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		uniqueTagIDs := make([]int64, 0, len(tagIDs))
		seen := make(map[int64]struct{}, len(tagIDs))
		for _, tagID := range tagIDs {
			if tagID <= 0 {
				return ErrTagNotFound
			}
			if _, exists := seen[tagID]; exists {
				continue
			}
			seen[tagID] = struct{}{}
			uniqueTagIDs = append(uniqueTagIDs, tagID)
		}
		if len(uniqueTagIDs) > 0 {
			query := tx.Model(&models.Tag{}).Where("id IN ?", uniqueTagIDs)
			if userID > 0 {
				query = query.Where("user_id = 0 OR user_id = ?", userID)
			}
			var count int64
			if err := query.Count(&count).Error; err != nil {
				return fmt.Errorf("validate question tags: %w", err)
			}
			if count != int64(len(uniqueTagIDs)) {
				return ErrTagNotFound
			}
		}
		if err := tx.Where("question_id = ?", questionID).Delete(&models.QuestionTag{}).Error; err != nil {
			return fmt.Errorf("delete question tags: %w", err)
		}
		for _, tagID := range uniqueTagIDs {
			if err := tx.Create(&models.QuestionTag{QuestionID: questionID, TagID: tagID}).Error; err != nil {
				return fmt.Errorf("insert question tag: %w", err)
			}
		}
		return nil
	})
}

// ApplyTaxonomy atomically validates and applies an existing top-level category
// and existing tags. It never creates taxonomy rows from an AI suggestion.
func (r *QuestionRepository) ApplyTaxonomy(questionID int64, categoryID *int64, tagIDs []int64, userIDs ...int64) error {
	userID := int64(0)
	if len(userIDs) > 0 {
		userID = userIDs[0]
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if categoryID != nil {
			var category models.Category
			categoryQuery := tx.Where("id = ? AND parent_id IS NULL", *categoryID)
			if userID > 0 {
				categoryQuery = categoryQuery.Where("user_id = 0 OR user_id = ?", userID)
			}
			if err := categoryQuery.First(&category).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("category not found or not top-level")
				}
				return fmt.Errorf("validate category: %w", err)
			}
		}
		uniqueTagIDs := make([]int64, 0, len(tagIDs))
		seen := make(map[int64]struct{}, len(tagIDs))
		for _, tagID := range tagIDs {
			if tagID <= 0 {
				return ErrTagNotFound
			}
			if _, exists := seen[tagID]; exists {
				continue
			}
			seen[tagID] = struct{}{}
			uniqueTagIDs = append(uniqueTagIDs, tagID)
		}
		if len(uniqueTagIDs) > 0 {
			tagQuery := tx.Model(&models.Tag{}).Where("id IN ?", uniqueTagIDs)
			if userID > 0 {
				tagQuery = tagQuery.Where("user_id = 0 OR user_id = ?", userID)
			}
			var count int64
			if err := tagQuery.Count(&count).Error; err != nil {
				return fmt.Errorf("validate tags: %w", err)
			}
			if count != int64(len(uniqueTagIDs)) {
				return ErrTagNotFound
			}
		}
		questionQuery := tx.Model(&models.Question{}).Where("id = ?", questionID)
		if userID > 0 {
			questionQuery = questionQuery.Where("user_id = ?", userID)
		}
		if err := questionQuery.Update("category_id", categoryID).Error; err != nil {
			return fmt.Errorf("apply category: %w", err)
		}
		if err := tx.Where("question_id = ?", questionID).Delete(&models.QuestionTag{}).Error; err != nil {
			return fmt.Errorf("replace question tags: %w", err)
		}
		for _, tagID := range uniqueTagIDs {
			if err := tx.Create(&models.QuestionTag{QuestionID: questionID, TagID: tagID}).Error; err != nil {
				return fmt.Errorf("insert question tag: %w", err)
			}
		}
		return nil
	})
}
