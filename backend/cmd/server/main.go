package main

import (
	"context"
	"log"
	"time"

	"erro-notebook/backend/internal/config"
	"erro-notebook/backend/internal/database"
	"erro-notebook/backend/internal/handlers"
	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/repository"
	"erro-notebook/backend/internal/services"
	"erro-notebook/backend/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get sql db failed: %v", err)
	}
	defer sqlDB.Close()

	aiClient := ai.NewClient(
		cfg.AIServiceBaseURL,
		time.Duration(cfg.AIServiceTimeoutSeconds)*time.Second,
	)
	aiClientAsync := ai.NewClient(
		cfg.AIServiceBaseURL,
		time.Duration(cfg.AIServiceAsyncTimeoutSeconds)*time.Second,
	)
	objectStorage, err := storage.NewLocalObjectStorage(cfg.StorageLocalRoot)
	if err != nil {
		log.Fatalf("init object storage failed: %v", err)
	}

	questionRepo := repository.NewQuestionRepository(db)
	jobRepo := repository.NewJobRepository(db)
	analysisRepo := repository.NewAnalysisRepository(db)
	chatRepo := repository.NewChatRepository(db)
	batchRepo := repository.NewBatchRepository(db)
	learningRepo := repository.NewLearningStateRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	tagRepo := repository.NewTagRepository(db)
	proposalRepo := repository.NewAIProposalRepository(db)

	questionService := services.NewQuestionService(
		questionRepo,
		jobRepo,
		analysisRepo,
		chatRepo,
		batchRepo,
		learningRepo,
		categoryRepo,
		tagRepo,
		aiClient,
		aiClientAsync,
		objectStorage,
		proposalRepo,
	)
	jobService := services.NewJobService(jobRepo)
	taxonomyService := services.NewTaxonomyService(categoryRepo, tagRepo)
	questionAIService := services.NewQuestionAIService(aiClient)
	questionHandler := handlers.NewQuestionHandler(questionService, jobService)
	taxonomyHandler := handlers.NewTaxonomyHandler(taxonomyService, questionRepo)
	questionAIHandler := handlers.NewQuestionAIHandler(questionAIService)
	practiceRepo := repository.NewPracticeSessionRepository(db)
	practiceService := services.NewPracticeService(practiceRepo, questionRepo, learningRepo, aiClient, proposalRepo)
	practiceHandler := handlers.NewPracticeHandler(practiceService)
	router := handlers.NewRouter(questionHandler, taxonomyHandler, questionAIHandler, practiceHandler)
	questionService.StartTaskWorkers(
		context.Background(),
		cfg.TaskWorkerCount,
		time.Duration(cfg.TaskPollIntervalSeconds)*time.Second,
		time.Duration(cfg.TaskStaleAfterSeconds)*time.Second,
	)

	addr := ":" + cfg.Port
	log.Printf("backend server listening on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
