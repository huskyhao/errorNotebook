package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type HealthResponse struct {
	Status           string  `json:"status"`
	OCRBackend       string  `json:"ocrBackend"`
	LLMBackend       string  `json:"llmBackend"`
	LLMConfigured    bool    `json:"llmConfigured"`
	LLMModel         *string `json:"llmModel"`
	VisionBackend    string  `json:"visionBackend"`
	VisionConfigured bool    `json:"visionConfigured"`
	VisionModel      *string `json:"visionModel"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/health", nil)
	if err != nil {
		return nil, fmt.Errorf("build ai health request: %w", err)
	}
	var result HealthResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type OptionItem struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type StructuredQuestion struct {
	Stem               string           `json:"stem"`
	QuestionType       string           `json:"questionType"`
	Options            []OptionItem     `json:"options"`
	Assets             []any            `json:"assets"`
	SuggestedAnswer    string           `json:"suggestedAnswer"`
	RawText            string           `json:"rawText"`
	HasDiagram         bool             `json:"hasDiagram"`
	DiagramDescription string           `json:"diagramDescription,omitempty"`
	Warnings           []string         `json:"warnings"`
	Metadata           QuestionMetadata `json:"metadata"`
}

type QuestionMetadata struct {
	SourceType       string   `json:"sourceType"`
	HasDiagram       bool     `json:"hasDiagram"`
	OCRConfidence    *float64 `json:"ocrConfidence,omitempty"`
	ImportMode       string   `json:"importMode"`
	ExtractionMethod *string  `json:"extractionMethod,omitempty"`
}

type OCRResponse struct {
	TraceID            string             `json:"traceId"`
	QuestionID         int64              `json:"questionId"`
	Status             string             `json:"status"`
	RawText            string             `json:"rawText"`
	StructuredQuestion StructuredQuestion `json:"structuredQuestion"`
	Warnings           []string           `json:"warnings"`
	Cost               map[string]int     `json:"cost"`
}

type AnalyzeQuestionRequest struct {
	QuestionID int64              `json:"questionId"`
	TraceID    string             `json:"traceId"`
	Question   StructuredQuestion `json:"question"`
	UserAnswer string             `json:"userAnswer,omitempty"`
	Context    map[string]any     `json:"context,omitempty"`
}

type AnalysisPayload struct {
	Answer                   string              `json:"answer"`
	AnswerFormat             string              `json:"answerFormat,omitempty"`
	BlankAnswers             []string            `json:"blankAnswers,omitempty"`
	ScoringPoints            []string            `json:"scoringPoints,omitempty"`
	Rubric                   []string            `json:"rubric,omitempty"`
	Summary                  string              `json:"summary"`
	KnowledgePoints          []string            `json:"knowledgePoints"`
	TaxonomySuggestion       *TaxonomySuggestion `json:"taxonomySuggestion,omitempty"`
	Steps                    []string            `json:"steps"`
	OptionAnalysis           map[string]string   `json:"optionAnalysis"`
	Pitfalls                 []string            `json:"pitfalls"`
	ReviewAdvice             []string            `json:"reviewAdvice"`
	TaxonomySuggestionReason string              `json:"taxonomySuggestionReason,omitempty"`
}

// TaxonomySuggestion is advisory only. The Go business layer stores it with
// the analysis, validates it, and applies it by default only when the user has
// not already chosen a taxonomy.
type TaxonomySuggestion struct {
	CategoryName       string   `json:"categoryName,omitempty"`
	TagNames           []string `json:"tagNames,omitempty"`
	Confidence         *float64 `json:"confidence,omitempty"`
	CategoryID         *int64   `json:"categoryId,omitempty"`
	TagIDs             []int64  `json:"tagIds,omitempty"`
	UnresolvedCategory bool     `json:"unresolvedCategory,omitempty"`
	UnresolvedTagNames []string `json:"unresolvedTagNames,omitempty"`
}

type AnalyzeQuestionResponse struct {
	TraceID    string          `json:"traceId"`
	QuestionID int64           `json:"questionId"`
	Status     string          `json:"status"`
	Analysis   AnalysisPayload `json:"analysis"`
	Cost       map[string]int  `json:"cost"`
}

type AgentActionRequest struct {
	TraceID    string         `json:"traceId"`
	QuestionID int64          `json:"questionId"`
	Action     string         `json:"action"`
	Context    AgentContext   `json:"context"`
	Params     map[string]any `json:"params,omitempty"`
}

type AgentContext struct {
	Question              StructuredQuestion `json:"question"`
	Warnings              []string           `json:"warnings,omitempty"`
	ReferenceAnswer       string             `json:"referenceAnswer,omitempty"`
	ReferenceAnswerSource string             `json:"referenceAnswerSource,omitempty"`
	LatestAnswer          string             `json:"latestAnswer,omitempty"`
	Analysis              *AnalysisPayload   `json:"analysis,omitempty"`
	Conversation          []ChatMessageItem  `json:"conversation,omitempty"`
	CategoryCandidates    []string           `json:"categoryCandidates,omitempty"`
	TagCandidates         []string           `json:"tagCandidates,omitempty"`
	ContentFingerprint    string             `json:"contentFingerprint,omitempty"`
	Version               string             `json:"version,omitempty"`
}

type AgentActionResponse struct {
	TraceID    string          `json:"traceId"`
	QuestionID int64           `json:"questionId"`
	Action     string          `json:"action"`
	Status     string          `json:"status"`
	Result     json.RawMessage `json:"result"`
	Warnings   []string        `json:"warnings"`
	Error      *AIError        `json:"error,omitempty"`
	Meta       map[string]any  `json:"meta"`
}

type AIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	TraceID   string `json:"traceId,omitempty"`
}

type DiagnoseMistakeResult struct {
	MistakeReason string   `json:"mistakeReason"`
	ReasonType    string   `json:"reasonType"`
	Evidence      []string `json:"evidence"`
	WeaknessTags  []string `json:"weaknessTags"`
	ReviewAdvice  []string `json:"reviewAdvice"`
	Uncertainties []string `json:"uncertainties"`
}

func (c *Client) ParseQuestionImage(
	ctx context.Context,
	questionID int64,
	traceID string,
	sourceType string,
	filePath string,
) (*OCRResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open image file: %w", err)
	}
	defer file.Close()

	contentType, err := detectFileContentType(file)
	if err != nil {
		return nil, fmt.Errorf("detect content type: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("question_id", strconv.FormatInt(questionID, 10)); err != nil {
		return nil, fmt.Errorf("write question_id: %w", err)
	}
	if err := writer.WriteField("trace_id", traceID); err != nil {
		return nil, fmt.Errorf("write trace_id: %w", err)
	}
	if err := writer.WriteField("source_type", sourceType); err != nil {
		return nil, fmt.Errorf("write source_type: %w", err)
	}

	part, err := createFilePart(writer, "file", filepath.Base(filePath), contentType)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/ocr/parse",
		&body,
	)
	if err != nil {
		return nil, fmt.Errorf("build ocr request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var result OCRResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) AnalyzeQuestion(
	ctx context.Context,
	payload AnalyzeQuestionRequest,
	imagePath string,
) (*AnalyzeQuestionResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal analyze request: %w", err)
	}
	if err := writer.WriteField("payload", string(payloadJSON)); err != nil {
		return nil, fmt.Errorf("write payload field: %w", err)
	}

	if imagePath != "" {
		file, err := os.Open(imagePath)
		if err != nil {
			return nil, fmt.Errorf("open image file for analysis: %w", err)
		}
		defer file.Close()

		contentType, err := detectFileContentType(file)
		if err != nil {
			return nil, fmt.Errorf("detect content type: %w", err)
		}

		part, err := createFilePart(writer, "file", filepath.Base(imagePath), contentType)
		if err != nil {
			return nil, fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("copy file content: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/analyze/question",
		&body,
	)
	if err != nil {
		return nil, fmt.Errorf("build analyze request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var result AnalyzeQuestionResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func detectFileContentType(file *os.File) (string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	contentType := http.DetectContentType(head[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return contentType, nil
}

func createFilePart(writer *multipart.Writer, fieldname, filename, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldname, filename))
	h.Set("Content-Type", contentType)
	return writer.CreatePart(h)
}

type ChatMessageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	QuestionID int64              `json:"questionId"`
	TraceID    string             `json:"traceId"`
	Question   StructuredQuestion `json:"question"`
	UserAnswer string             `json:"userAnswer,omitempty"`
	Analysis   *AnalysisPayload   `json:"analysis,omitempty"`
	History    []ChatMessageItem  `json:"history"`
	Message    string             `json:"message"`
}

type ChatImage struct {
	FileName    string
	ContentType string
	Bytes       []byte
}

type ChatResponse struct {
	TraceID    string         `json:"traceId"`
	QuestionID int64          `json:"questionId"`
	Status     string         `json:"status"`
	Reply      string         `json:"reply"`
	Cost       map[string]int `json:"cost"`
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/chat/question",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("build chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var result ChatResponse
	if err := c.doJSON(httpReq, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ChatWithImages preserves the JSON contract in a multipart payload and sends
// the actual server-read bytes as files. Paths and filenames are never used as
// visual input.
func (c *Client) ChatWithImages(ctx context.Context, req ChatRequest, images []ChatImage) (*ChatResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal chat payload: %w", err)
	}
	if err := writer.WriteField("payload", string(payload)); err != nil {
		return nil, err
	}
	for _, image := range images {
		part, err := createFilePart(writer, "files", filepath.Base(image.FileName), image.ContentType)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(image.Bytes); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/chat/question", &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	var result ChatResponse
	if err := c.doJSON(httpReq, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) AgentAction(ctx context.Context, req AgentActionRequest) (*AgentActionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal agent action request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/agent/actions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build agent action request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	var result AgentActionResponse
	if err := c.doJSON(httpReq, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) doJSON(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call ai service: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read ai response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("ai service returned status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(respBytes, target); err != nil {
		return fmt.Errorf("unmarshal ai response: %w", err)
	}

	return nil
}
