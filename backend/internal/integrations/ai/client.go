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

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
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
	Answer             string              `json:"answer"`
	Summary            string              `json:"summary"`
	KnowledgePoints    []string            `json:"knowledgePoints"`
	TaxonomySuggestion *TaxonomySuggestion `json:"taxonomySuggestion,omitempty"`
	Steps              []string            `json:"steps"`
	OptionAnalysis     map[string]string   `json:"optionAnalysis"`
	Pitfalls           []string            `json:"pitfalls"`
	ReviewAdvice       []string            `json:"reviewAdvice"`
}

// TaxonomySuggestion is advisory only. The Go business layer stores it with
// the analysis and remains responsible for validating and applying any
// category/tag changes after user confirmation.
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
		return fmt.Errorf("ai service returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	if err := json.Unmarshal(respBytes, target); err != nil {
		return fmt.Errorf("unmarshal ai response: %w", err)
	}

	return nil
}
