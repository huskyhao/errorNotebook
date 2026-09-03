package repository

import (
	"errors"
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) Create(category *models.Category) error {
	if err := r.db.Create(category).Error; err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) List() ([]models.Category, error) {
	var categories []models.Category
	if err := r.db.Order("id desc").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return categories, nil
}

func (r *CategoryRepository) GetByID(id int64) (*models.Category, error) {
	var category models.Category
	if err := r.db.First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get category by id: %w", err)
	}
	return &category, nil
}

func (r *CategoryRepository) FindOrCreate(name string) (*models.Category, error) {
	var category models.Category
	err := r.db.Where("name = ?", name).First(&category).Error
	if err == nil {
		return &category, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find category: %w", err)
	}
	category = models.Category{Name: name}
	if err := r.db.Create(&category).Error; err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	return &category, nil
}

func (r *CategoryRepository) Update(category *models.Category) error {
	if err := r.db.Save(category).Error; err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) Delete(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var children []models.Category
		if err := tx.Where("parent_id = ?", id).Find(&children).Error; err != nil {
			return fmt.Errorf("find children: %w", err)
		}
		var parentCategory models.Category
		parentFound := tx.First(&parentCategory, id).Error == nil

		var newParentID *int64
		if parentFound {
			newParentID = parentCategory.ParentID
		}
		for _, child := range children {
			child.ParentID = newParentID
			if err := tx.Save(&child).Error; err != nil {
				return fmt.Errorf("reparent child category: %w", err)
			}
		}
		if err := tx.Model(&models.Question{}).Where("category_id = ?", id).Update("category_id", nil).Error; err != nil {
			return fmt.Errorf("nullify question categories: %w", err)
		}
		if err := tx.Delete(&models.Category{}, id).Error; err != nil {
			return fmt.Errorf("delete category: %w", err)
		}
		return nil
	})
}

type TagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) Create(tag *models.Tag) error {
	if err := r.db.Create(tag).Error; err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	return nil
}

func (r *TagRepository) List() ([]models.Tag, error) {
	var tags []models.Tag
	if err := r.db.Order("id desc").Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

func (r *TagRepository) FindOrCreate(name string) (*models.Tag, error) {
	var tag models.Tag
	err := r.db.Where("name = ?", name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find tag: %w", err)
	}
	tag = models.Tag{Name: name}
	if err := r.db.Create(&tag).Error; err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &tag, nil
}

func (r *TagRepository) Delete(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&models.QuestionTag{}).Error; err != nil {
			return fmt.Errorf("delete question tags: %w", err)
		}
		if err := tx.Delete(&models.Tag{}, id).Error; err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
		return nil
	})
}
