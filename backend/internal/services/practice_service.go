package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
)

type PracticeService struct {
	practiceRepo *repository.PracticeSessionRepository
	questionRepo *repository.QuestionRepository
	learningRepo *repository.LearningStateRepository
	aiClient     *ai.Client
	proposalRepo *repository.AIProposalRepository
}

func NewPracticeService(
	practiceRepo *repository.PracticeSessionRepository,
	questionRepo *repository.QuestionRepository,
	learningRepo *repository.LearningStateRepository,
	clients ...any,
) *PracticeService {
	var aiClient *ai.Client
	var proposalRepo *repository.AIProposalRepository
	for _, item := range clients {
		switch value := item.(type) {
		case *ai.Client:
			aiClient = value
		case *repository.AIProposalRepository:
			proposalRepo = value
		}
	}
	return &PracticeService{
		practiceRepo: practiceRepo,
		questionRepo: questionRepo,
		learningRepo: learningRepo,
		aiClient:     aiClient, proposalRepo: proposalRepo,
	}
}

type CreateSessionInput struct {
	Name        string  `json:"name"`
	QuestionIDs []int64 `json:"questionIds"`
}

type PracticeSessionListItem struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	TotalCount   int       `json:"totalCount"`
	CorrectCount int       `json:"correctCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

type PracticeSessionDetail struct {
	ID           int64                    `json:"id"`
	Name         string                   `json:"name"`
	Status       string                   `json:"status"`
	TotalCount   int                      `json:"totalCount"`
	CorrectCount int                      `json:"correctCount"`
	CreatedAt    time.Time                `json:"createdAt"`
	Questions    []PracticeSessionQDetail `json:"questions"`
}

type PracticeSessionQDetail struct {
	ID             int64          `json:"id"`
	OrderIndex     int            `json:"orderIndex"`
	UserAnswer     *string        `json:"userAnswer,omitempty"`
	IsCorrect      *bool          `json:"isCorrect,omitempty"`
	Status         string         `json:"status"`
	Score          *float64       `json:"score,omitempty"`
	MaxScore       *float64       `json:"maxScore,omitempty"`
	GradingStatus  string         `json:"gradingStatus"`
	GradingComment *string        `json:"gradingComment,omitempty"`
	Question       *QuestionBrief `json:"question"`
}

type QuestionBrief struct {
	ID           int64                   `json:"id"`
	Stem         string                  `json:"stem"`
	QuestionType string                  `json:"questionType"`
	Options      []models.QuestionOption `json:"options"`
}

type PracticeSessionResult struct {
	ID           int64                    `json:"id"`
	Name         string                   `json:"name"`
	Status       string                   `json:"status"`
	TotalCount   int                      `json:"totalCount"`
	CorrectCount int                      `json:"correctCount"`
	ScorePercent float64                  `json:"scorePercent"`
	CreatedAt    time.Time                `json:"createdAt"`
	Questions    []PracticeSessionQResult `json:"questions"`
}

type PracticeRecommendationGroup struct {
	Key         string  `json:"key"`
	Title       string  `json:"title"`
	Reason      string  `json:"reason"`
	QuestionIDs []int64 `json:"questionIds"`
	Count       int     `json:"count"`
}

type PracticeSessionQResult struct {
	ID             int64         `json:"id"`
	OrderIndex     int           `json:"orderIndex"`
	UserAnswer     *string       `json:"userAnswer,omitempty"`
	CorrectAnswer  *string       `json:"correctAnswer,omitempty"`
	IsCorrect      *bool         `json:"isCorrect,omitempty"`
	Status         string        `json:"status"`
	Score          *float64      `json:"score,omitempty"`
	MaxScore       *float64      `json:"maxScore,omitempty"`
	GradingStatus  string        `json:"gradingStatus"`
	GradingComment *string       `json:"gradingComment,omitempty"`
	Question       *QuestionFull `json:"question"`
}

type QuestionFull struct {
	ID            int64                   `json:"id"`
	Stem          string                  `json:"stem"`
	QuestionType  string                  `json:"questionType"`
	CorrectAnswer *string                 `json:"correctAnswer,omitempty"`
	Options       []models.QuestionOption `json:"options"`
}

func (s *PracticeService) CreateSession(userID int64, input CreateSessionInput) (*PracticeSessionDetail, error) {
	if len(input.QuestionIDs) == 0 {
		return nil, fmt.Errorf("questionIds must not be empty")
	}

	for _, qid := range input.QuestionIDs {
		var q *models.Question
		var err error
		if userID > 0 {
			q, err = s.questionRepo.GetByIDForUser(qid, userID)
		} else {
			q, err = s.questionRepo.GetByID(qid)
		}
		if err != nil {
			return nil, fmt.Errorf("get question %d: %w", qid, err)
		}
		if q == nil {
			return nil, fmt.Errorf("question %d not found", qid)
		}
		questionType := normalizeQuestionType(q.QuestionType)
		if !models.IsSupportedQuestionType(questionType) {
			return nil, fmt.Errorf("question %d has unsupported question type %s", qid, q.QuestionType)
		}
		if models.RequiresOptions(questionType) && len(q.Options) == 0 {
			return nil, fmt.Errorf("question %d has no options for %s", qid, questionType)
		}
	}

	name := input.Name
	if name == "" {
		name = time.Now().Format("2006-01-02 15:04") + " 练习"
	}

	session := &models.PracticeSession{
		UserID:     userID,
		Name:       name,
		Status:     "in_progress",
		TotalCount: len(input.QuestionIDs),
	}

	if err := s.practiceRepo.Create(session); err != nil {
		return nil, err
	}

	items := make([]models.PracticeSessionQuestion, len(input.QuestionIDs))
	for i, qid := range input.QuestionIDs {
		items[i] = models.PracticeSessionQuestion{
			UserID:        userID,
			SessionID:     session.ID,
			QuestionID:    qid,
			OrderIndex:    i,
			Status:        "unanswered",
			GradingStatus: "ungraded",
		}
	}

	if err := s.practiceRepo.CreateQuestions(items); err != nil {
		return nil, err
	}

	return s.buildDetail(session.ID, false)
}

func (s *PracticeService) AuthorizeSession(userID, sessionID int64) error {
	session, err := s.practiceRepo.GetByIDForUser(sessionID, userID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}
	return nil
}

func (s *PracticeService) GetPracticeRecommendationsForUser(userID int64, now time.Time) ([]PracticeRecommendationGroup, error) {
	if userID <= 0 {
		return s.GetPracticeRecommendations(now)
	}
	limit := 8
	due, err := s.learningRepo.DueForReviewForUser(userID, now, limit)
	if err != nil {
		return nil, err
	}
	recentWrong, err := s.learningRepo.RecentWrongForUser(userID, limit)
	if err != nil {
		return nil, err
	}
	weakStates, err := s.learningRepo.WithWeaknessTagsForUser(userID, limit)
	if err != nil {
		return nil, err
	}
	newQuestions, err := s.learningRepo.NewQuestionsForUser(userID, limit)
	if err != nil {
		return nil, err
	}
	mixedQuestions, err := s.learningRepo.MixedQuestionsForUser(userID, limit)
	if err != nil {
		return nil, err
	}
	weakTag := topWeaknessTag(weakStates)
	weakReason := "按薄弱标签聚合，优先复盘高频出错点"
	if weakTag != "" {
		weakReason = "围绕「" + weakTag + "」集中练习"
	}
	return []PracticeRecommendationGroup{
		buildRecommendationGroup("today_review", "今日复习", "下次复习时间已到的题目", idsFromStates(due)),
		buildRecommendationGroup("recent_wrong", "最近错题", "按最近练习记录优先回看答错题", idsFromStates(recentWrong)),
		buildRecommendationGroup("weak_points", "薄弱专项", weakReason, idsFromStates(weakStates)),
		buildRecommendationGroup("new_questions", "新题巩固", "尚未练习或没有掌握记录的题目", idsFromQuestions(newQuestions)),
		buildRecommendationGroup("mixed_random", "随机混合", "混合抽取题库题目，适合快速自测", idsFromQuestions(mixedQuestions)),
	}, nil
}

func (s *PracticeService) GetSession(sessionID int64) (*PracticeSessionDetail, error) {
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	includeAnswers := session.Status == "submitted"
	return s.buildDetail(sessionID, includeAnswers)
}

func (s *PracticeService) ListSessions(userID int64) ([]PracticeSessionListItem, error) {
	sessions, err := s.practiceRepo.ListByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]PracticeSessionListItem, len(sessions))
	for i, s := range sessions {
		result[i] = PracticeSessionListItem{
			ID:           s.ID,
			Name:         s.Name,
			Status:       s.Status,
			TotalCount:   s.TotalCount,
			CorrectCount: s.CorrectCount,
			CreatedAt:    s.CreatedAt,
		}
	}
	return result, nil
}

func (s *PracticeService) AnswerQuestion(sessionID int64, orderIndex int, userAnswer string) error {
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}
	if session.Status != "in_progress" {
		return fmt.Errorf("session is not in progress")
	}
	if orderIndex < 0 || orderIndex >= session.TotalCount {
		return fmt.Errorf("invalid order index: %d", orderIndex)
	}

	item, err := s.practiceRepo.FindQuestion(sessionID, orderIndex)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("question not found in session")
	}

	item.UserAnswer = &userAnswer
	item.Status = "answered"
	return s.practiceRepo.UpdateQuestion(item)
}

func (s *PracticeService) SkipQuestion(sessionID int64, orderIndex int) error {
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}
	if session.Status != "in_progress" {
		return fmt.Errorf("session is not in progress")
	}

	item, err := s.practiceRepo.FindQuestion(sessionID, orderIndex)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("question not found in session")
	}

	item.Status = "skipped"
	item.UserAnswer = nil
	return s.practiceRepo.UpdateQuestion(item)
}

func (s *PracticeService) SubmitSession(sessionID int64) (*PracticeSessionResult, error) {
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if session.Status != "in_progress" {
		return nil, fmt.Errorf("session already submitted")
	}

	items, err := s.practiceRepo.ListQuestionsBySession(sessionID)
	if err != nil {
		return nil, err
	}

	correctCount := 0
	for i := range items {
		item := &items[i]
		grade := gradePracticeAnswer(item.Question, item.UserAnswer, item.Status)
		item.IsCorrect = grade.IsCorrect
		item.Status = grade.Status
		item.Score = grade.Score
		item.MaxScore = grade.MaxScore
		item.GradingStatus = grade.GradingStatus
		item.GradingComment = grade.Comment
		if grade.IsCorrect != nil && *grade.IsCorrect {
			correctCount++
		}
		if err := s.practiceRepo.UpdateQuestion(item); err != nil {
			return nil, err
		}
		if s.learningRepo != nil {
			_ = s.updateLearningAfterPractice(session.UserID, item.Question.ID, grade, time.Now())
		}
	}

	session.Status = "submitted"
	session.CorrectCount = correctCount
	if err := s.practiceRepo.Update(session); err != nil {
		return nil, err
	}

	return s.buildResult(session)
}

func (s *PracticeService) GetPracticeRecommendations(now time.Time) ([]PracticeRecommendationGroup, error) {
	const limit = 8
	if s.learningRepo == nil {
		return nil, fmt.Errorf("learning state repository unavailable")
	}
	due, err := s.learningRepo.DueForReview(now, limit)
	if err != nil {
		return nil, err
	}
	recentWrong, err := s.learningRepo.RecentWrong(limit)
	if err != nil {
		return nil, err
	}
	weakStates, err := s.learningRepo.WithWeaknessTags(limit)
	if err != nil {
		return nil, err
	}
	newQuestions, err := s.learningRepo.NewQuestions(limit)
	if err != nil {
		return nil, err
	}
	mixedQuestions, err := s.learningRepo.MixedQuestions(limit)
	if err != nil {
		return nil, err
	}

	weakTag := topWeaknessTag(weakStates)
	weakReason := "按薄弱标签聚合，优先复盘高频出错点"
	if weakTag != "" {
		weakReason = "围绕「" + weakTag + "」集中练习"
	}

	return []PracticeRecommendationGroup{
		buildRecommendationGroup("today_review", "今日复习", "下次复习时间已到的题目", idsFromStates(due)),
		buildRecommendationGroup("recent_wrong", "最近错题", "按最近练习记录优先回看答错题", idsFromStates(recentWrong)),
		buildRecommendationGroup("weak_points", "薄弱专项", weakReason, idsFromStates(weakStates)),
		buildRecommendationGroup("new_questions", "新题巩固", "尚未练习或没有掌握记录的题目", idsFromQuestions(newQuestions)),
		buildRecommendationGroup("mixed_random", "随机混合", "混合抽取题库题目，适合快速自测", idsFromQuestions(mixedQuestions)),
	}, nil
}

func (s *PracticeService) updateLearningAfterPractice(userID, questionID int64, grade practiceGrade, now time.Time) error {
	state, err := s.learningRepo.EnsureForUser(questionID, userID)
	if err != nil {
		return err
	}
	applyPracticeGradeToLearningState(state, grade, now)
	return s.learningRepo.Save(state)
}

func (s *PracticeService) GetSessionResults(sessionID int64) (*PracticeSessionResult, error) {
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	if session.Status != "submitted" {
		return nil, fmt.Errorf("session not yet submitted")
	}

	return s.buildResult(session)
}

// GenerateGradeSuggestion asks Python for an advisory score tied to this
// exact practice answer. It never mutates the session or learning state.
func (s *PracticeService) GenerateGradeSuggestion(ctx context.Context, sessionID int64, orderIndex int, params map[string]any) (*ai.AgentActionResponse, error) {
	if s.aiClient == nil {
		return nil, fmt.Errorf("ai service unavailable")
	}
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	item, err := s.practiceRepo.FindQuestion(sessionID, orderIndex)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("question not found in session")
	}
	if item.UserAnswer == nil || strings.TrimSpace(*item.UserAnswer) == "" {
		return nil, fmt.Errorf("user answer is required")
	}
	var question *models.Question
	if item.Question.ID == 0 {
		question, err = s.questionRepo.GetByID(item.QuestionID)
		if err != nil || question == nil {
			return nil, fmt.Errorf("question not found")
		}
	} else {
		question = &item.Question
	}
	if models.IsObjectiveQuestionType(normalizeQuestionType(question.QuestionType)) {
		return nil, fmt.Errorf("objective questions do not use AI grading")
	}
	requestParams := map[string]any{}
	for key, value := range params {
		requestParams[key] = value
	}
	if _, ok := requestParams["maxScore"]; !ok {
		requestParams["maxScore"] = 10
	}
	if _, ok := requestParams["standardAnswer"]; !ok && question.CorrectAnswer != nil {
		requestParams["standardAnswer"] = *question.CorrectAnswer
	}
	fingerprint := practiceAnswerFingerprint(question, *item.UserAnswer)
	resp, err := s.aiClient.AgentAction(ctx, ai.AgentActionRequest{TraceID: fmt.Sprintf("trace_grade_%d", time.Now().UnixNano()), QuestionID: question.ID, Action: "grade_subjective_answer", Context: ai.AgentContext{Question: toAIStructuredQuestion(question, parseWarnings(question.StructureWarnings)), Warnings: parseWarnings(question.StructureWarnings), ReferenceAnswer: derefString(question.CorrectAnswer), ReferenceAnswerSource: "question.correctAnswer", LatestAnswer: *item.UserAnswer, ContentFingerprint: fingerprint, Version: fingerprint}, Params: requestParams})
	if err != nil {
		return nil, err
	}
	if s.proposalRepo != nil && resp != nil && len(resp.Result) > 0 && (resp.Status == "completed" || resp.Status == "needs_review") {
		var envelope struct {
			ProposalID string `json:"proposalId"`
		}
		_ = json.Unmarshal(resp.Result, &envelope)
		proposalID := envelope.ProposalID
		if proposalID == "" {
			proposalID = fmt.Sprintf("grade_%d_%d", sessionID, time.Now().UnixNano())
		}
		key := fmt.Sprintf("grade:%d:%d:%s", sessionID, orderIndex, fingerprint)
		proposal, saveErr := s.proposalRepo.CreateOrGet(&models.AIProposal{ProposalID: proposalID, UserID: session.UserID, QuestionID: question.ID, Action: "grade_subjective_answer", Status: "pending", ContentJSON: string(resp.Result), SourceFingerprint: fingerprint, IdempotencyKey: key, ExpiresAt: time.Now().Add(24 * time.Hour)})
		if saveErr != nil {
			return nil, saveErr
		}
		if proposal != nil {
			proposalID = proposal.ProposalID
		}
		var resultMap map[string]any
		if json.Unmarshal(resp.Result, &resultMap) == nil {
			resultMap["proposalId"] = proposalID
			resp.Result, _ = json.Marshal(resultMap)
		}
	}
	return resp, nil
}

func (s *PracticeService) ConfirmGradeSuggestion(ctx context.Context, sessionID int64, orderIndex int, proposalID string, score *float64, feedback *string) error {
	if s.proposalRepo == nil {
		return fmt.Errorf("proposal repository unavailable")
	}
	item, err := s.practiceRepo.FindQuestion(sessionID, orderIndex)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("question not found in session")
	}
	answer := derefString(item.UserAnswer)
	question, err := s.questionRepo.GetByID(item.QuestionID)
	if err != nil || question == nil {
		return fmt.Errorf("question not found")
	}
	session, sessionErr := s.practiceRepo.GetByID(sessionID)
	if sessionErr != nil || session == nil {
		return fmt.Errorf("practice session not found")
	}
	return s.proposalRepo.ConfirmGrade(proposalID, sessionID, orderIndex, practiceAnswerFingerprint(question, answer), score, feedback, session.UserID)
}

func (s *PracticeService) GetGradeSuggestion(ctx context.Context, proposalID string, questionID int64) (*models.AIProposal, error) {
	if s.proposalRepo == nil {
		return nil, fmt.Errorf("proposal repository unavailable")
	}
	var proposal *models.AIProposal
	var err error
	if userID := currentUserID(ctx); userID > 0 {
		proposal, err = s.proposalRepo.GetForUser(proposalID, userID)
	} else {
		proposal, err = s.proposalRepo.Get(proposalID)
	}
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.QuestionID != questionID || proposal.Action != "grade_subjective_answer" {
		return nil, fmt.Errorf("grading suggestion not found")
	}
	return proposal, nil
}

func (s *PracticeService) GetSessionQuestion(sessionID int64, orderIndex int) (*models.PracticeSessionQuestion, error) {
	return s.practiceRepo.FindQuestion(sessionID, orderIndex)
}

func practiceAnswerFingerprint(question *models.Question, answer string) string {
	data, _ := json.Marshal(struct {
		QuestionID         int64
		Stem, Type, Answer string
	}{question.ID, question.Stem, question.QuestionType, answer})
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func (s *PracticeService) buildDetail(sessionID int64, includeAnswers bool) (*PracticeSessionDetail, error) {
	session, err := s.practiceRepo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	detail := &PracticeSessionDetail{
		ID:           session.ID,
		Name:         session.Name,
		Status:       session.Status,
		TotalCount:   session.TotalCount,
		CorrectCount: session.CorrectCount,
		CreatedAt:    session.CreatedAt,
	}

	for _, item := range session.Questions {
		qd := PracticeSessionQDetail{
			ID:             item.ID,
			OrderIndex:     item.OrderIndex,
			UserAnswer:     item.UserAnswer,
			IsCorrect:      item.IsCorrect,
			Status:         item.Status,
			Score:          item.Score,
			MaxScore:       item.MaxScore,
			GradingStatus:  item.GradingStatus,
			GradingComment: item.GradingComment,
		}

		qd.Question = &QuestionBrief{
			ID:           item.Question.ID,
			Stem:         item.Question.Stem,
			QuestionType: item.Question.QuestionType,
			Options:      item.Question.Options,
		}

		if includeAnswers {
			qd.Question = &QuestionBrief{
				ID:           item.Question.ID,
				Stem:         item.Question.Stem,
				QuestionType: item.Question.QuestionType,
				Options:      item.Question.Options,
			}
		}

		detail.Questions = append(detail.Questions, qd)
	}

	return detail, nil
}

func (s *PracticeService) buildResult(session *models.PracticeSession) (*PracticeSessionResult, error) {
	items, err := s.practiceRepo.ListQuestionsBySession(session.ID)
	if err != nil {
		return nil, err
	}

	scorePercent := float64(0)
	totalAutoScore := float64(0)
	earnedAutoScore := float64(0)
	for _, item := range items {
		if item.Score != nil && item.MaxScore != nil && *item.MaxScore > 0 {
			earnedAutoScore += *item.Score
			totalAutoScore += *item.MaxScore
		}
	}
	if totalAutoScore > 0 {
		scorePercent = earnedAutoScore / totalAutoScore * 100
	}

	result := &PracticeSessionResult{
		ID:           session.ID,
		Name:         session.Name,
		Status:       session.Status,
		TotalCount:   session.TotalCount,
		CorrectCount: session.CorrectCount,
		ScorePercent: scorePercent,
		CreatedAt:    session.CreatedAt,
	}

	for _, item := range items {
		qr := PracticeSessionQResult{
			ID:             item.ID,
			OrderIndex:     item.OrderIndex,
			UserAnswer:     item.UserAnswer,
			CorrectAnswer:  item.Question.CorrectAnswer,
			IsCorrect:      item.IsCorrect,
			Status:         item.Status,
			Score:          item.Score,
			MaxScore:       item.MaxScore,
			GradingStatus:  item.GradingStatus,
			GradingComment: item.GradingComment,
			Question: &QuestionFull{
				ID:            item.Question.ID,
				Stem:          item.Question.Stem,
				QuestionType:  item.Question.QuestionType,
				CorrectAnswer: item.Question.CorrectAnswer,
				Options:       item.Question.Options,
			},
		}
		result.Questions = append(result.Questions, qr)
	}

	return result, nil
}

type practiceGrade struct {
	IsCorrect     *bool
	Status        string
	Score         *float64
	MaxScore      *float64
	GradingStatus string
	Comment       *string
}

func gradePracticeAnswer(question models.Question, userAnswer *string, currentStatus string) practiceGrade {
	questionType := normalizeQuestionType(question.QuestionType)
	if currentStatus == "skipped" || userAnswer == nil || strings.TrimSpace(*userAnswer) == "" {
		f := false
		return practiceGrade{
			IsCorrect:     &f,
			Status:        "skipped",
			Score:         ptrFloat(0),
			MaxScore:      objectiveMaxScore(questionType),
			GradingStatus: "ungraded",
		}
	}

	if !models.IsObjectiveQuestionType(questionType) {
		comment := "主观题已提交，等待人工批改或后续 LLM 辅助评分。"
		return practiceGrade{
			IsCorrect:     nil,
			Status:        "manual_required",
			Score:         nil,
			MaxScore:      nil,
			GradingStatus: "manual_required",
			Comment:       &comment,
		}
	}

	correct := false
	if question.CorrectAnswer != nil {
		correct = compareObjectiveAnswer(questionType, *userAnswer, *question.CorrectAnswer)
	}
	status := "wrong"
	score := float64(0)
	if correct {
		status = "correct"
		score = 1
	}
	return practiceGrade{
		IsCorrect:     &correct,
		Status:        status,
		Score:         &score,
		MaxScore:      ptrFloat(1),
		GradingStatus: "auto_graded",
	}
}

func compareObjectiveAnswer(questionType string, userAnswer string, correctAnswer string) bool {
	switch questionType {
	case models.QuestionTypeSingleChoice:
		return normalizeChoiceAnswer(userAnswer) == normalizeChoiceAnswer(correctAnswer)
	case models.QuestionTypeMultipleChoice:
		return strings.Join(normalizeChoiceSet(userAnswer), ",") == strings.Join(normalizeChoiceSet(correctAnswer), ",")
	case models.QuestionTypeTrueFalse:
		userValue, userOK := normalizeTrueFalse(userAnswer)
		correctValue, correctOK := normalizeTrueFalse(correctAnswer)
		return userOK && correctOK && userValue == correctValue
	case models.QuestionTypeFillBlank:
		userValue := normalizeBlankAnswer(userAnswer)
		for _, accepted := range parseAcceptedBlankAnswers(correctAnswer) {
			if userValue == normalizeBlankAnswer(accepted) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func normalizeChoiceAnswer(value string) string {
	parts := normalizeChoiceSet(value)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func normalizeChoiceSet(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	var jsonItems []string
	if strings.HasPrefix(value, "[") && json.Unmarshal([]byte(value), &jsonItems) == nil {
		value = strings.Join(jsonItems, ",")
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == ' ' || r == '\n' || r == '\t'
	})
	seen := map[string]struct{}{}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := strings.ToUpper(strings.TrimSpace(part))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func normalizeTrueFalse(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "yes", "y", "1", "a", "正确", "对", "是", "√":
		return true, true
	case "false", "f", "no", "n", "0", "b", "错误", "错", "否", "×", "x":
		return false, true
	default:
		return false, false
	}
}

func normalizeBlankAnswer(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseAcceptedBlankAnswers(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	var jsonItems []string
	if strings.HasPrefix(value, "[") && json.Unmarshal([]byte(value), &jsonItems) == nil {
		return jsonItems
	}
	return strings.Split(value, "|")
}

func objectiveMaxScore(questionType string) *float64 {
	if models.IsObjectiveQuestionType(questionType) {
		return ptrFloat(1)
	}
	return nil
}

func ptrFloat(value float64) *float64 {
	return &value
}

func applyPracticeGradeToLearningState(state *models.QuestionLearningState, grade practiceGrade, now time.Time) {
	state.LastPracticedAt = &now
	if grade.Status == "manual_required" || grade.IsCorrect == nil {
		reviewAt := now.Add(24 * time.Hour)
		state.NextReviewAt = &reviewAt
		return
	}
	if *grade.IsCorrect {
		state.CorrectStreak++
		if state.MasteryLevel < 5 {
			state.MasteryLevel++
		}
		days := []int{1, 2, 4, 7, 14, 30}
		reviewAt := now.Add(time.Duration(days[state.MasteryLevel]) * 24 * time.Hour)
		state.NextReviewAt = &reviewAt
		return
	}
	state.WrongCount++
	state.CorrectStreak = 0
	if state.MasteryLevel > 1 {
		state.MasteryLevel--
	} else {
		state.MasteryLevel = 0
	}
	reviewAt := now.Add(12 * time.Hour)
	state.NextReviewAt = &reviewAt
}

func buildRecommendationGroup(key, title, reason string, ids []int64) PracticeRecommendationGroup {
	ids = uniqueInt64(ids)
	return PracticeRecommendationGroup{
		Key:         key,
		Title:       title,
		Reason:      reason,
		QuestionIDs: ids,
		Count:       len(ids),
	}
}

func idsFromStates(states []models.QuestionLearningState) []int64 {
	result := make([]int64, 0, len(states))
	for _, state := range states {
		result = append(result, state.QuestionID)
	}
	return result
}

func idsFromQuestions(questions []models.Question) []int64 {
	result := make([]int64, 0, len(questions))
	for _, question := range questions {
		result = append(result, question.ID)
	}
	return result
}

func uniqueInt64(values []int64) []int64 {
	seen := map[int64]struct{}{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func topWeaknessTag(states []models.QuestionLearningState) string {
	counts := map[string]int{}
	best := ""
	bestCount := 0
	for _, state := range states {
		for _, tag := range unmarshalStringSlice(state.WeaknessTags) {
			counts[tag]++
			if counts[tag] > bestCount {
				best = tag
				bestCount = counts[tag]
			}
		}
	}
	return best
}
