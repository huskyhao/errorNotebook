package repository

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type SimilarQuestionContent struct {
	ProposalID        string `json:"proposalId"`
	SourceQuestionID  int64  `json:"sourceQuestionId"`
	SourceFingerprint string `json:"sourceFingerprint"`
	Stem              string `json:"stem"`
	QuestionType      string `json:"questionType"`
	Options           []struct {
		Key     string `json:"key"`
		Content string `json:"content"`
	} `json:"options"`
	Answer        string `json:"answer"`
	Analysis      string `json:"analysis"`
	QualityStatus string `json:"qualityStatus"`
}

type AIProposalRepository struct{ db *gorm.DB }

func NewAIProposalRepository(db *gorm.DB) *AIProposalRepository { return &AIProposalRepository{db: db} }

func (r *AIProposalRepository) CreateOrGet(proposal *models.AIProposal) (*models.AIProposal, error) {
	var existing models.AIProposal
	if err := r.db.Where("idempotency_key = ?", proposal.IdempotencyKey).First(&existing).Error; err == nil {
		return &existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find ai proposal: %w", err)
	}
	if err := r.db.Create(proposal).Error; err != nil {
		var duplicate models.AIProposal
		if r.db.Where("idempotency_key = ?", proposal.IdempotencyKey).First(&duplicate).Error == nil {
			return &duplicate, nil
		}
		return nil, fmt.Errorf("create ai proposal: %w", err)
	}
	return proposal, nil
}

func (r *AIProposalRepository) Get(proposalID string) (*models.AIProposal, error) {
	var proposal models.AIProposal
	if err := r.db.Where("proposal_id = ?", proposalID).First(&proposal).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ai proposal: %w", err)
	}
	return &proposal, nil
}

func (r *AIProposalRepository) GetForUser(proposalID string, userID int64) (*models.AIProposal, error) {
	var proposal models.AIProposal
	if err := r.db.Where("proposal_id = ? AND user_id = ?", proposalID, userID).First(&proposal).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ai proposal by owner: %w", err)
	}
	return &proposal, nil
}

func (r *AIProposalRepository) Update(proposal *models.AIProposal) error {
	if err := r.db.Save(proposal).Error; err != nil {
		return fmt.Errorf("update ai proposal: %w", err)
	}
	return nil
}

func (r *AIProposalRepository) ExpireIfNeeded(proposal *models.AIProposal, now time.Time) bool {
	if proposal != nil && proposal.Status == "pending" && !proposal.ExpiresAt.After(now) {
		proposal.Status = "expired"
		_ = r.Update(proposal)
		return true
	}
	return false
}

func (r *AIProposalRepository) ConfirmSimilar(proposalID string, userID int64, sourceFingerprint string) (int64, error) {
	var createdID int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var proposal models.AIProposal
		if err := tx.Where("proposal_id = ? AND user_id = ?", proposalID, userID).First(&proposal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAIProposalNotFound
			}
			return err
		}
		if proposal.Status == "applied" && proposal.CreatedQuestionID != nil {
			createdID = *proposal.CreatedQuestionID
			return nil
		}
		if proposal.Status != "pending" {
			return fmt.Errorf("proposal status is %s", proposal.Status)
		}
		if !proposal.ExpiresAt.After(time.Now()) {
			proposal.Status = "expired"
			_ = tx.Save(&proposal)
			return fmt.Errorf("proposal expired")
		}
		if proposal.SourceFingerprint != sourceFingerprint {
			return fmt.Errorf("source question has changed")
		}
		var source models.Question
		if err := tx.Preload("Options").First(&source, proposal.QuestionID).Error; err != nil {
			return err
		}
		if proposal.SourceFingerprint != proposalQuestionFingerprint(&source) {
			return fmt.Errorf("source question has changed")
		}
		var content SimilarQuestionContent
		if err := json.Unmarshal([]byte(proposal.ContentJSON), &content); err != nil {
			return err
		}
		if content.Stem == "" || !models.IsSupportedQuestionType(content.QuestionType) {
			return fmt.Errorf("invalid similar question content")
		}
		if content.QuestionType == models.QuestionTypeSingleChoice || content.QuestionType == models.QuestionTypeMultipleChoice || content.QuestionType == models.QuestionTypeTrueFalse {
			seen := map[string]bool{}
			for _, option := range content.Options {
				if option.Key == "" || seen[option.Key] {
					return fmt.Errorf("invalid option keys")
				}
				seen[option.Key] = true
			}
			answers := strings.Split(content.Answer, ",")
			if content.Answer == "" {
				return fmt.Errorf("answer is not an option")
			}
			for _, answer := range answers {
				if !seen[strings.TrimSpace(answer)] {
					return fmt.Errorf("answer is not an option")
				}
			}
		}
		q := models.Question{UserID: userID, Stem: content.Stem, QuestionType: content.QuestionType, SourceType: "ai_generated", OCRStatus: "completed", AnalysisStatus: "completed", ParseSource: "ai_proposal", CorrectAnswer: stringPtr(content.Answer)}
		if err := tx.Create(&q).Error; err != nil {
			return err
		}
		for index, option := range content.Options {
			if err := tx.Create(&models.QuestionOption{QuestionID: q.ID, OptionKey: option.Key, Content: option.Content, SortOrder: index + 1}).Error; err != nil {
				return err
			}
		}
		proposal.Status = "applied"
		proposal.CreatedQuestionID = &q.ID
		proposal.ConfirmedBy = &userID
		now := time.Now()
		proposal.ConfirmedAt = &now
		if err := tx.Save(&proposal).Error; err != nil {
			return err
		}
		createdID = q.ID
		_ = source // source existence is part of the confirmation guard
		return nil
	})
	return createdID, err
}

func (r *AIProposalRepository) Reject(proposalID string, userID int64) error {
	var proposal models.AIProposal
	if err := r.db.Where("proposal_id = ? AND user_id = ?", proposalID, userID).First(&proposal).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAIProposalNotFound
		}
		return err
	}
	if proposal.Status == "rejected" {
		return nil
	}
	if proposal.Status != "pending" {
		return fmt.Errorf("proposal status is %s", proposal.Status)
	}
	proposal.Status = "rejected"
	return r.Update(&proposal)
}

func (r *AIProposalRepository) ConfirmGrade(proposalID string, sessionID int64, orderIndex int, sourceFingerprint string, scoreOverride *float64, feedbackOverride *string, confirmedBy ...int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var proposal models.AIProposal
		proposalQuery := tx.Where("proposal_id = ?", proposalID)
		if len(confirmedBy) > 0 {
			proposalQuery = proposalQuery.Where("user_id = ?", confirmedBy[0])
		}
		if err := proposalQuery.First(&proposal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAIProposalNotFound
			}
			return err
		}
		if proposal.Action != "grade_subjective_answer" {
			return fmt.Errorf("proposal is not a grading suggestion")
		}
		if proposal.Status == "applied" {
			return nil
		}
		if proposal.Status != "pending" {
			return fmt.Errorf("proposal status is %s", proposal.Status)
		}
		if !proposal.ExpiresAt.After(time.Now()) {
			proposal.Status = "expired"
			_ = tx.Save(&proposal)
			return fmt.Errorf("proposal expired")
		}
		if proposal.SourceFingerprint != sourceFingerprint {
			return fmt.Errorf("answer has changed")
		}
		var item models.PracticeSessionQuestion
		if err := tx.Where("session_id = ? AND order_index = ?", sessionID, orderIndex).First(&item).Error; err != nil {
			return err
		}
		if proposal.QuestionID != item.QuestionID {
			return fmt.Errorf("proposal does not belong to session question")
		}
		var content struct {
			SuggestedScore float64 `json:"suggestedScore"`
			MaxScore       float64 `json:"maxScore"`
			Feedback       string  `json:"feedback"`
		}
		if err := json.Unmarshal([]byte(proposal.ContentJSON), &content); err != nil {
			return err
		}
		score := content.SuggestedScore
		if scoreOverride != nil {
			score = *scoreOverride
		}
		if content.MaxScore <= 0 || score < 0 || score > content.MaxScore {
			return fmt.Errorf("score is outside maxScore")
		}
		feedback := content.Feedback
		if feedbackOverride != nil {
			feedback = *feedbackOverride
		}
		item.Score = &score
		item.MaxScore = &content.MaxScore
		item.GradingComment = &feedback
		item.GradingStatus = "manual_required"
		item.Status = "ungraded"
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		now := time.Now()
		proposal.Status = "applied"
		if len(confirmedBy) > 0 {
			proposal.ConfirmedBy = &confirmedBy[0]
		}
		proposal.ConfirmedAt = &now
		return tx.Save(&proposal).Error
	})
}

var ErrAIProposalNotFound = errors.New("ai proposal not found")

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func proposalQuestionFingerprint(question *models.Question) string {
	payload := struct {
		Stem          string
		QuestionType  string
		CorrectAnswer string
		UserAnswer    string
		Options       []models.QuestionOption
	}{question.Stem, question.QuestionType, derefProposalString(question.CorrectAnswer), derefProposalString(question.UserAnswer), question.Options}
	data, _ := json.Marshal(payload)
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func derefProposalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
