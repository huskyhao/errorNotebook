package repository

import (
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type BatchRepository struct {
	db *gorm.DB
}

func NewBatchRepository(db *gorm.DB) *BatchRepository {
	return &BatchRepository{db: db}
}

func (r *BatchRepository) CreateBatch(batch *models.BatchImport) error {
	if err := r.db.Create(batch).Error; err != nil {
		return fmt.Errorf("create batch import: %w", err)
	}
	return nil
}

func (r *BatchRepository) CreateItems(items []models.BatchImportItem) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.Create(&items).Error; err != nil {
		return fmt.Errorf("create batch items: %w", err)
	}
	return nil
}

func (r *BatchRepository) GetBatchByID(id int64) (*models.BatchImport, error) {
	var batch models.BatchImport
	if err := r.db.First(&batch, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get batch: %w", err)
	}
	return &batch, nil
}

func (r *BatchRepository) GetItemsByBatchID(batchID int64) ([]models.BatchImportItem, error) {
	var items []models.BatchImportItem
	if err := r.db.Where("batch_id = ?", batchID).Order("file_index asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list batch items: %w", err)
	}
	return items, nil
}

func (r *BatchRepository) UpdateItem(item *models.BatchImportItem) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("update batch item: %w", err)
	}
	return nil
}
