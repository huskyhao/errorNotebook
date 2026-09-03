package services

import (
	"strings"

	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
)

type TaxonomyService struct {
	categoryRepo *repository.CategoryRepository
	tagRepo      *repository.TagRepository
}

func NewTaxonomyService(
	categoryRepo *repository.CategoryRepository,
	tagRepo *repository.TagRepository,
) *TaxonomyService {
	return &TaxonomyService{
		categoryRepo: categoryRepo,
		tagRepo:      tagRepo,
	}
}

func (s *TaxonomyService) ListCategories() ([]models.Category, error) {
	return s.categoryRepo.List()
}

func (s *TaxonomyService) CreateCategory(name string, parentID *int64) (*models.Category, error) {
	category := &models.Category{
		Name:     strings.TrimSpace(name),
		ParentID: parentID,
	}
	if err := s.categoryRepo.Create(category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *TaxonomyService) ListTags() ([]models.Tag, error) {
	return s.tagRepo.List()
}

func (s *TaxonomyService) CreateTag(name string) (*models.Tag, error) {
	tag := &models.Tag{Name: strings.TrimSpace(name)}
	if err := s.tagRepo.Create(tag); err != nil {
		return nil, err
	}
	return tag, nil
}

func (s *TaxonomyService) DeleteTag(id int64) error {
	return s.tagRepo.Delete(id)
}

func (s *TaxonomyService) UpdateCategory(id int64, name string, parentID *int64) (*models.Category, error) {
	category, err := s.categoryRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, nil
	}
	category.Name = strings.TrimSpace(name)
	category.ParentID = parentID
	if err := s.categoryRepo.Update(category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *TaxonomyService) DeleteCategory(id int64) error {
	return s.categoryRepo.Delete(id)
}

type CategoryTreeNode struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	ParentID      *int64 `json:"parentId,omitempty"`
	QuestionCount int64  `json:"questionCount"`
}

func (s *TaxonomyService) GetCategoryTree(counts map[int64]int64) ([]CategoryTreeNode, error) {
	categories, err := s.categoryRepo.List()
	if err != nil {
		return nil, err
	}

	nodes := make([]CategoryTreeNode, 0, len(categories))
	for _, cat := range categories {
		pc := cat.ParentID // copy pointer
		nodes = append(nodes, CategoryTreeNode{
			ID:            cat.ID,
			Name:          cat.Name,
			ParentID:      pc,
			QuestionCount: counts[cat.ID],
		})
	}
	return nodes, nil
}
