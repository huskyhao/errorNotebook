package services

import (
	"encoding/json"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/models"
)

func TestValidateImageFile(t *testing.T) {
	imageHeader := textproto.MIMEHeader{}
	imageHeader.Set("Content-Type", "image/png")
	if err := validateImageFile(&multipart.FileHeader{Filename: "question.png", Header: imageHeader}); err != nil {
		t.Fatalf("validateImageFile(image) returned error: %v", err)
	}

	pdfHeader := textproto.MIMEHeader{}
	pdfHeader.Set("Content-Type", "application/pdf")
	if err := validateImageFile(&multipart.FileHeader{Filename: "paper.pdf", Header: pdfHeader}); err == nil {
		t.Fatal("validateImageFile(pdf) returned nil, want rejection")
	}
}

func TestCategoryNamesForAIKeepsBroadSubjects(t *testing.T) {
	parentID := int64(9)
	categories := []models.Category{
		{ID: 1, Name: "数据结构"},
		{ID: 2, Name: "图论"},
		{ID: 3, Name: "操作系统"},
		{ID: 4, Name: "数据库系统"},
		{ID: 5, Name: "编译原理"},
		{ID: 6, Name: "进程调度", ParentID: &parentID},
	}

	got := categoryNamesForAI(categories)
	want := []string{"数据结构", "图论", "操作系统", "数据库系统", "编译原理"}
	if len(got) != len(want) {
		t.Fatalf("categoryNamesForAI() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("categoryNamesForAI() = %#v, want %#v", got, want)
		}
	}
}

func TestValidateTaxonomySuggestionLimitsTagsToThree(t *testing.T) {
	suggestion := (&QuestionService{}).validateTaxonomySuggestion(&ai.TaxonomySuggestion{
		TagNames: []string{"标签一", "标签二", "标签三", "标签四"},
	})
	if len(suggestion.TagNames) != 3 {
		t.Fatalf("validateTaxonomySuggestion() kept %d tags, want 3", len(suggestion.TagNames))
	}
}

func TestValidateChatAttachmentRejectsNonImage(t *testing.T) {
	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain")
	if err := validateImageFile(&multipart.FileHeader{Filename: "note.txt", Header: textHeader}); err == nil {
		t.Fatal("validateImageFile(text attachment) returned nil, want rejection")
	}
}

func TestBuildChatPromptWithAttachments(t *testing.T) {
	prompt := buildChatPromptWithAttachments("这一步为什么这样做？", []ChatAttachment{
		{FileName: "follow-up.png", ContentType: "image/png", FilePath: "backend/uploads/1/chat/follow-up.png"},
	})
	if prompt == "这一步为什么这样做？" {
		t.Fatal("prompt did not include attachment context")
	}
	if !strings.Contains(prompt, "follow-up.png") {
		t.Fatalf("prompt = %q, want attachment filename", prompt)
	}
}

func TestQuestionDetailIncludesStructureQuality(t *testing.T) {
	warningsJSON, _ := json.Marshal([]string{"options_incomplete", "missing_options_C"})
	confidence := 0.62
	question := &models.Question{
		ID:                  1,
		Stem:                "题干",
		QuestionType:        models.QuestionTypeSingleChoice,
		OCRStatus:           "completed",
		AnalysisStatus:      "pending",
		SourceType:          "image",
		StructureWarnings:   ptrString(string(warningsJSON)),
		StructureConfidence: &confidence,
		ParseSource:         "llm_refined",
	}

	detail := toQuestionDetail(question)

	if detail.StructureConfidence == nil || *detail.StructureConfidence != confidence {
		t.Fatalf("StructureConfidence = %v, want %v", detail.StructureConfidence, confidence)
	}
	if detail.ParseSource != "llm_refined" {
		t.Fatalf("ParseSource = %s, want llm_refined", detail.ParseSource)
	}
	if len(detail.StructureWarnings) != 2 || detail.StructureWarnings[1] != "missing_options_C" {
		t.Fatalf("StructureWarnings = %#v, want warnings", detail.StructureWarnings)
	}
	if detail.QualityStatus != "needs_review" {
		t.Fatalf("QualityStatus = %s, want needs_review", detail.QualityStatus)
	}
}

func TestQualityStatusManualCorrectedSuppressesResolvedWarnings(t *testing.T) {
	if got := qualityStatus([]string{}, nil, "manual_corrected"); got != "ok" {
		t.Fatalf("qualityStatus manual corrected = %s, want ok", got)
	}
	if got := qualityStatus([]string{"llm_refine_failed"}, nil, "rules"); got != "needs_review" {
		t.Fatalf("qualityStatus refine failed = %s, want needs_review", got)
	}
	low := 0.71
	if got := qualityStatus(nil, &low, "llm_refined"); got != "needs_review" {
		t.Fatalf("qualityStatus low confidence = %s, want needs_review", got)
	}
}

func TestJobStatusContractUsesPendingAndNeedsReview(t *testing.T) {
	pending := &models.Job{Status: "pending"}
	if pending.Status != "pending" {
		t.Fatalf("new task status = %q, want pending", pending.Status)
	}

	job := markJobNeedsReview(&models.Job{JobID: "ocr_test", Status: "processing"})
	if job.Status != "needs_review" {
		t.Fatalf("review task status = %q, want needs_review", job.Status)
	}
	if job.FinishedAt == nil {
		t.Fatal("needs_review task has no finished timestamp")
	}
}

func TestAIStructuredQuestionCarriesStructureQuality(t *testing.T) {
	raw := "raw ocr"
	confidence := 0.62
	question := &models.Question{
		ID:                  1,
		Stem:                "题干",
		QuestionType:        models.QuestionTypeSingleChoice,
		CorrectAnswer:       ptrString("B"),
		SourceType:          "image",
		RawOCRText:          &raw,
		StructureConfidence: &confidence,
		ParseSource:         "llm_refined",
		Options: []models.QuestionOption{
			{OptionKey: "A", Content: "1"},
			{OptionKey: "B", Content: "2"},
		},
	}
	warnings := []string{"options_incomplete", "missing_options_CD"}

	payload := toAIStructuredQuestion(question, warnings)

	if payload.RawText != raw {
		t.Fatalf("RawText = %q, want raw ocr", payload.RawText)
	}
	if len(payload.Warnings) != 2 || payload.Warnings[0] != "options_incomplete" {
		t.Fatalf("Warnings = %#v, want structure warnings", payload.Warnings)
	}
	if payload.Metadata.OCRConfidence == nil || *payload.Metadata.OCRConfidence != confidence {
		t.Fatalf("OCRConfidence = %v, want %v", payload.Metadata.OCRConfidence, confidence)
	}
	if payload.Metadata.ExtractionMethod == nil || *payload.Metadata.ExtractionMethod != "llm_refined" {
		t.Fatalf("ExtractionMethod = %v, want llm_refined", payload.Metadata.ExtractionMethod)
	}
}

func TestAIStructuredQuestionSerializesEmptyWarningsAsArray(t *testing.T) {
	payload := toAIStructuredQuestion(&models.Question{
		ID: 1, Stem: "干净题干", QuestionType: "subjective", SourceType: "manual",
	}, nil)

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"warnings":[]`) {
		t.Fatalf("warnings serialized as null: %s", encoded)
	}
}

func TestNormalizeOptionsKeepsStableSortOrder(t *testing.T) {
	options := normalizeOptions(9, []models.QuestionOption{
		{OptionKey: " A ", Content: " 选项一 ", SortOrder: 99},
		{OptionKey: "B", Content: "选项二", SortOrder: 1},
	})
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2", len(options))
	}
	if options[0].QuestionID != 9 || options[0].OptionKey != "A" || options[0].Content != "选项一" || options[0].SortOrder != 1 {
		t.Fatalf("first option = %#v, want normalized option with sortOrder 1", options[0])
	}
	if options[1].SortOrder != 2 {
		t.Fatalf("second sortOrder = %d, want 2", options[1].SortOrder)
	}
}

func ptrString(value string) *string {
	return &value
}
