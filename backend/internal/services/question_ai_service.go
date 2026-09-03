package services

import (
	"context"
	"fmt"

	"erro-notebook/backend/internal/integrations/ai"
)

type QuestionAIService struct {
	aiClient *ai.Client
}

func NewQuestionAIService(aiClient *ai.Client) *QuestionAIService {
	return &QuestionAIService{
		aiClient: aiClient,
	}
}

func (s *QuestionAIService) ParseQuestionImage(
	ctx context.Context,
	questionID int64,
	traceID string,
	sourceType string,
	filePath string,
) (*ai.OCRResponse, error) {
	result, err := s.aiClient.ParseQuestionImage(ctx, questionID, traceID, sourceType, filePath)
	if err != nil {
		return nil, fmt.Errorf("parse question image with ai service: %w", err)
	}

	return result, nil
}

func (s *QuestionAIService) AnalyzeQuestion(
	ctx context.Context,
	payload ai.AnalyzeQuestionRequest,
	imagePath string,
) (*ai.AnalyzeQuestionResponse, error) {
	result, err := s.aiClient.AnalyzeQuestion(ctx, payload, imagePath)
	if err != nil {
		return nil, fmt.Errorf("analyze question with ai service: %w", err)
	}

	return result, nil
}
