package services

import (
	"testing"
	"time"

	"erro-notebook/backend/internal/models"
)

func TestCompareObjectiveAnswer(t *testing.T) {
	tests := []struct {
		name          string
		questionType  string
		userAnswer    string
		correctAnswer string
		want          bool
	}{
		{"single choice exact normalized", models.QuestionTypeSingleChoice, " a ", "A", true},
		{"multiple choice ignores order", models.QuestionTypeMultipleChoice, "C,A", "A,C", true},
		{"multiple choice rejects missing option", models.QuestionTypeMultipleChoice, "A", "A,C", false},
		{"true false accepts Chinese true", models.QuestionTypeTrueFalse, "对", "true", true},
		{"true false accepts B false", models.QuestionTypeTrueFalse, "B", "false", true},
		{"fill blank trims and supports alternatives", models.QuestionTypeFillBlank, " TCP ", "UDP|tcp", true},
		{"fill blank supports JSON alternatives", models.QuestionTypeFillBlank, "http", `["tcp","http"]`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareObjectiveAnswer(tt.questionType, tt.userAnswer, tt.correctAnswer); got != tt.want {
				t.Fatalf("compareObjectiveAnswer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGradePracticeAnswerSubjectiveRequiresManualReview(t *testing.T) {
	answer := "Use an index to reduce full table scans."
	questionTypes := []string{
		models.QuestionTypeSubjective,
		models.QuestionTypeShortAnswer,
		models.QuestionTypeEssay,
		models.QuestionTypeCalculation,
	}

	for _, questionType := range questionTypes {
		t.Run(questionType, func(t *testing.T) {
			grade := gradePracticeAnswer(models.Question{QuestionType: questionType}, &answer, "answered")
			if grade.IsCorrect != nil {
				t.Fatalf("subjective grade IsCorrect = %v, want nil", *grade.IsCorrect)
			}
			if grade.Status != "manual_required" {
				t.Fatalf("subjective status = %s, want manual_required", grade.Status)
			}
			if grade.GradingStatus != "manual_required" {
				t.Fatalf("grading status = %s, want manual_required", grade.GradingStatus)
			}
		})
	}
}

func TestApplyPracticeGradeToLearningState(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	state := &models.QuestionLearningState{MasteryLevel: 2, CorrectStreak: 1}
	correct := true
	applyPracticeGradeToLearningState(state, practiceGrade{IsCorrect: &correct, Status: "correct"}, now)
	if state.CorrectStreak != 2 || state.MasteryLevel != 3 {
		t.Fatalf("correct state = streak %d mastery %d, want 2/3", state.CorrectStreak, state.MasteryLevel)
	}
	if state.NextReviewAt == nil || !state.NextReviewAt.After(now) {
		t.Fatalf("correct next review = %v, want future time", state.NextReviewAt)
	}

	wrong := false
	applyPracticeGradeToLearningState(state, practiceGrade{IsCorrect: &wrong, Status: "wrong"}, now)
	if state.WrongCount != 1 || state.CorrectStreak != 0 || state.MasteryLevel != 2 {
		t.Fatalf("wrong state = wrong %d streak %d mastery %d, want 1/0/2", state.WrongCount, state.CorrectStreak, state.MasteryLevel)
	}
	if state.NextReviewAt == nil || state.NextReviewAt.Sub(now) != 12*time.Hour {
		t.Fatalf("wrong next review = %v, want 12h later", state.NextReviewAt)
	}
}

func TestApplyPracticeGradeManualRequiredKeepsMastery(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	state := &models.QuestionLearningState{MasteryLevel: 4, CorrectStreak: 2}
	applyPracticeGradeToLearningState(state, practiceGrade{Status: "manual_required", IsCorrect: nil}, now)
	if state.MasteryLevel != 4 || state.CorrectStreak != 2 {
		t.Fatalf("manual state = mastery %d streak %d, want unchanged 4/2", state.MasteryLevel, state.CorrectStreak)
	}
	if state.NextReviewAt == nil || state.NextReviewAt.Sub(now) != 24*time.Hour {
		t.Fatalf("manual next review = %v, want 24h later", state.NextReviewAt)
	}
}

func TestBuildRecommendationGroupUniqueCount(t *testing.T) {
	group := buildRecommendationGroup("recent_wrong", "最近错题", "复盘错题", []int64{3, 3, 2})
	if group.Count != 2 {
		t.Fatalf("Count = %d, want 2", group.Count)
	}
	if len(group.QuestionIDs) != 2 || group.QuestionIDs[0] != 3 || group.QuestionIDs[1] != 2 {
		t.Fatalf("QuestionIDs = %#v, want unique stable ids", group.QuestionIDs)
	}
}
