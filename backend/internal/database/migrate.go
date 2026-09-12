package database

import (
	"fmt"
	"strconv"
	"strings"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	// Ensure database and all tables use utf8mb4 before anything else.
	// GORM creates tables using the database default charset, so we must
	// fix the database charset first, then convert all existing tables.
	if err := db.Exec("ALTER DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		return fmt.Errorf("set database charset: %w", err)
	}

	// Drop indexes from the former multi-user taxonomy schema when they exist.
	for _, legacyIndex := range []struct {
		table string
		name  string
	}{
		{table: "categories", name: "idx_categories_name"},
		{table: "tags", name: "idx_tags_name"},
	} {
		var count int64
		if err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			legacyIndex.table, legacyIndex.name,
		).Scan(&count).Error; err != nil {
			continue
		}
		if count == 0 {
			continue
		}
		statement := fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`", legacyIndex.table, legacyIndex.name)
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("drop legacy taxonomy index %s.%s: %w", legacyIndex.table, legacyIndex.name, err)
		}
	}

	// Before switching taxonomy names to instance-wide unique indexes, merge
	// rows created by the former per-user schema. Keep the oldest row and
	// remap every question/tag relation to it.
	if err := mergeDuplicateCategories(db); err != nil {
		return fmt.Errorf("merge duplicate categories: %w", err)
	}
	if err := mergeDuplicateTags(db); err != nil {
		return fmt.Errorf("merge duplicate tags: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Category{},
		&models.Tag{},
		&models.Question{},
		&models.QuestionOption{},
		&models.QuestionAsset{},
		&models.Analysis{},
		&models.Job{},
		&models.ChatMessage{},
		&models.QuestionTag{},
		&models.QuestionLearningState{},
		&models.BatchImport{},
		&models.BatchImportItem{},
		&models.PracticeSession{},
		&models.PracticeSessionQuestion{},
		&models.AIProposal{},
	); err != nil {
		return fmt.Errorf("auto migrate tables: %w", err)
	}

	// These are deliberately broad 408 subjects. AI analysis may suggest one
	// of them as the question category; finer concepts belong in instance tags.
	for _, name := range []string{"数据结构", "计算机组成原理", "操作系统", "计算机网络"} {
		category := models.Category{}
		if err := db.Where("name = ?", name).
			FirstOrCreate(&category, &models.Category{Name: name}).Error; err != nil {
			return fmt.Errorf("seed system category %s: %w", name, err)
		}
	}

	// Convert all existing tables to utf8mb4 (harmless if already utf8mb4).
	tables := []string{
		"categories", "tags", "question_tags",
		"questions", "question_options", "question_assets",
		"analyses", "jobs", "chat_messages", "question_learning_states",
		"batch_imports", "batch_import_items",
		"practice_sessions", "practice_session_questions",
		"ai_proposals",
	}
	for _, table := range tables {
		sql := fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", table)
		if err := db.Exec(sql).Error; err != nil {
			// Table may not exist yet (first run before AutoMigrate creates them) — skip.
			continue
		}
	}

	return nil
}

type duplicateTaxonomyRow struct {
	Name string `gorm:"column:name"`
	IDs  string `gorm:"column:ids"`
}

func mergeDuplicateCategories(db *gorm.DB) error {
	exists, err := tableExists(db, "categories")
	if err != nil || !exists {
		return err
	}

	var rows []duplicateTaxonomyRow
	if err := db.Raw("SELECT name, GROUP_CONCAT(id ORDER BY id) AS ids FROM categories GROUP BY name HAVING COUNT(*) > 1").Scan(&rows).Error; err != nil {
		return err
	}
	questionsExist, err := tableExists(db, "questions")
	if err != nil {
		return err
	}

	for _, row := range rows {
		ids := parseIDs(row.IDs)
		if len(ids) < 2 {
			continue
		}
		keepID := ids[0]
		duplicateIDs := ids[1:]
		if err := db.Transaction(func(tx *gorm.DB) error {
			if questionsExist {
				if err := tx.Exec("UPDATE questions SET category_id = ? WHERE category_id IN ?", keepID, duplicateIDs).Error; err != nil {
					return err
				}
			}
			if err := tx.Exec("UPDATE categories SET parent_id = ? WHERE parent_id IN ?", keepID, duplicateIDs).Error; err != nil {
				return err
			}
			return tx.Exec("DELETE FROM categories WHERE id IN ?", duplicateIDs).Error
		}); err != nil {
			return fmt.Errorf("merge category %q: %w", row.Name, err)
		}
	}
	return nil
}

func mergeDuplicateTags(db *gorm.DB) error {
	exists, err := tableExists(db, "tags")
	if err != nil || !exists {
		return err
	}

	var rows []duplicateTaxonomyRow
	if err := db.Raw("SELECT name, GROUP_CONCAT(id ORDER BY id) AS ids FROM tags GROUP BY name HAVING COUNT(*) > 1").Scan(&rows).Error; err != nil {
		return err
	}
	questionTagsExist, err := tableExists(db, "question_tags")
	if err != nil {
		return err
	}

	for _, row := range rows {
		ids := parseIDs(row.IDs)
		if len(ids) < 2 {
			continue
		}
		keepID := ids[0]
		duplicateIDs := ids[1:]
		if err := db.Transaction(func(tx *gorm.DB) error {
			if questionTagsExist {
				for _, duplicateID := range duplicateIDs {
					if err := tx.Exec("INSERT IGNORE INTO question_tags (question_id, tag_id) SELECT question_id, ? FROM question_tags WHERE tag_id = ?", keepID, duplicateID).Error; err != nil {
						return err
					}
					if err := tx.Exec("DELETE FROM question_tags WHERE tag_id = ?", duplicateID).Error; err != nil {
						return err
					}
				}
			}
			return tx.Exec("DELETE FROM tags WHERE id IN ?", duplicateIDs).Error
		}); err != nil {
			return fmt.Errorf("merge tag %q: %w", row.Name, err)
		}
	}
	return nil
}

func tableExists(db *gorm.DB, table string) (bool, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func parseIDs(raw string) []int64 {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
