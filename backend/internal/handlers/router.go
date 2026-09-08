package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(
	questionHandler *QuestionHandler,
	taxonomyHandler *TaxonomyHandler,
	questionAIHandler *QuestionAIHandler,
	practiceHandler *PracticeHandler,
) *gin.Engine {
	router := gin.Default()
	router.Use(corsMiddleware())

	router.GET("/health", Health)

	api := router.Group("/api/v1")
	{
		api.GET("/health", Health)

		if questionHandler != nil {
			api.POST("/questions/import", questionHandler.Import)
			api.POST("/questions/batch-import", questionHandler.BatchImport)
			api.GET("/questions", questionHandler.ListQuestions)
			api.GET("/questions/:id", questionHandler.GetQuestion)
			api.PATCH("/questions/:id", questionHandler.UpdateQuestion)
			api.POST("/questions/:id/answer", questionHandler.SubmitAnswer)
			api.POST("/questions/:id/analyze", questionHandler.AnalyzeQuestion)
			api.POST("/questions/:id/ocr/retry", questionHandler.RetryOCR)
			api.GET("/questions/:id/analysis", questionHandler.GetAnalysis)
			api.GET("/questions/:id/learning-state", questionHandler.GetLearningState)
			api.PATCH("/questions/:id/learning-state", questionHandler.UpdateLearningState)
			api.POST("/questions/:id/learning-state/generate", questionHandler.GenerateLearningState)
			api.POST("/questions/:id/taxonomy-suggestion/apply", questionHandler.ApplyTaxonomySuggestion)
			api.POST("/questions/:id/agent-actions", questionHandler.RunAgentAction)
			api.POST("/questions/:id/chat", questionHandler.CreateChatMessage)
			api.GET("/questions/:id/chat", questionHandler.GetChatMessages)
			api.POST("/questions/:id/favorite", questionHandler.ToggleFavorite)
			api.POST("/questions/:id/tags", questionHandler.SetQuestionTags)
			api.DELETE("/questions/:id", questionHandler.DeleteQuestion)
			api.GET("/batch-imports/:id", questionHandler.GetBatchImport)
			api.GET("/jobs/:jobId", questionHandler.GetJob)
			api.POST("/jobs/:jobId/retry", questionHandler.RetryJob)
		}

		if practiceHandler != nil {
			api.GET("/recommendations/practice", practiceHandler.GetPracticeRecommendations)
			sessions := api.Group("/practice-sessions")
			{
				sessions.POST("", practiceHandler.CreateSession)
				sessions.GET("", practiceHandler.ListSessions)
				sessions.GET("/:id", practiceHandler.GetSession)
				sessions.POST("/:id/answer", practiceHandler.AnswerQuestion)
				sessions.POST("/:id/skip", practiceHandler.SkipQuestion)
				sessions.POST("/:id/submit", practiceHandler.SubmitSession)
				sessions.GET("/:id/results", practiceHandler.GetResults)
			}
		}

		if taxonomyHandler != nil {
			api.GET("/categories/tree", taxonomyHandler.GetCategoryTree)
			api.GET("/categories", taxonomyHandler.ListCategories)
			api.POST("/categories", taxonomyHandler.CreateCategory)
			api.PUT("/categories/:id", taxonomyHandler.UpdateCategory)
			api.DELETE("/categories/:id", taxonomyHandler.DeleteCategory)
			api.GET("/tags", taxonomyHandler.ListTags)
			api.POST("/tags", taxonomyHandler.CreateTag)
			api.DELETE("/tags/:id", taxonomyHandler.DeleteTag)
		}
	}

	if questionAIHandler != nil {
		internalAI := api.Group("/internal/ai")
		{
			internalAI.POST("/ocr/parse", questionAIHandler.ParseImage)
			internalAI.POST("/analyze/question", questionAIHandler.AnalyzeQuestion)
		}
	}

	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
