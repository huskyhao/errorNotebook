package database

import (
	"fmt"

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

	// Older development databases made taxonomy names globally unique. That
	// prevents two anonymous users from creating the same private subject/tag
	// name even though their rows are isolated. Drop those legacy indexes;
	// AutoMigrate below creates the scoped (user_id, name) indexes instead.
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

	if err := db.AutoMigrate(
		&models.User{},
		&models.UserSession{},
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
	// of them as the question category; finer concepts belong in private tags.
	for _, name := range []string{"数据结构", "计算机组成原理", "操作系统", "计算机网络"} {
		category := models.Category{}
		if err := db.Where("user_id = ? AND name = ?", 0, name).
			FirstOrCreate(&category, &models.Category{UserID: 0, Name: name}).Error; err != nil {
			return fmt.Errorf("seed system category %s: %w", name, err)
		}
	}

	// Existing pre-session rows used the development owner 1. Reserve that
	// owner so the first anonymous visitor cannot inherit legacy data.
	var legacy models.User
	if err := db.Where("id = ?", 1).First(&legacy).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("check legacy user: %w", err)
		}
		if err := db.Create(&models.User{ID: 1, Kind: "legacy"}).Error; err != nil {
			return fmt.Errorf("reserve legacy user: %w", err)
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
		"users", "user_sessions",
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
