package models

const (
	QuestionTypeSingleChoice   = "single_choice"
	QuestionTypeMultipleChoice = "multiple_choice"
	QuestionTypeTrueFalse      = "true_false"
	QuestionTypeFillBlank      = "fill_blank"
	QuestionTypeSubjective     = "subjective"
	QuestionTypeShortAnswer    = "short_answer"
	QuestionTypeEssay          = "essay"
	QuestionTypeCalculation    = "calculation"
)

var supportedQuestionTypes = map[string]struct{}{
	QuestionTypeSingleChoice:   {},
	QuestionTypeMultipleChoice: {},
	QuestionTypeTrueFalse:      {},
	QuestionTypeFillBlank:      {},
	QuestionTypeSubjective:     {},
	QuestionTypeShortAnswer:    {},
	QuestionTypeEssay:          {},
	QuestionTypeCalculation:    {},
}

func IsSupportedQuestionType(questionType string) bool {
	_, ok := supportedQuestionTypes[questionType]
	return ok
}

func IsObjectiveQuestionType(questionType string) bool {
	switch questionType {
	case QuestionTypeSingleChoice, QuestionTypeMultipleChoice, QuestionTypeTrueFalse, QuestionTypeFillBlank:
		return true
	default:
		return false
	}
}

func RequiresOptions(questionType string) bool {
	return questionType == QuestionTypeSingleChoice ||
		questionType == QuestionTypeMultipleChoice ||
		questionType == QuestionTypeTrueFalse
}
