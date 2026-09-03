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
	); err != nil {
		return fmt.Errorf("auto migrate tables: %w", err)
	}

	// Convert all existing tables to utf8mb4 (harmless if already utf8mb4).
	tables := []string{
		"categories", "tags", "question_tags",
		"questions", "question_options", "question_assets",
		"analyses", "jobs", "chat_messages", "question_learning_states",
		"batch_imports", "batch_import_items",
		"practice_sessions", "practice_session_questions",
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
