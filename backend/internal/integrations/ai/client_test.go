package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentActionContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/agent/actions" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload AgentActionRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Action != "diagnose_mistake" || payload.Context.Question.Stem == "" {
			t.Fatalf("request context was not carried: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"traceId":"t-1","questionId":7,"action":"diagnose_mistake","status":"completed","result":{"mistakeReason":"条件遗漏","reasonType":"条件遗漏","evidence":["userAnswer:B"],"weaknessTags":["集合"],"reviewAdvice":["重做"],"uncertainties":[]},"warnings":[],"meta":{"source":"mock"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	response, err := client.AgentAction(context.Background(), AgentActionRequest{
		TraceID: "t-1", QuestionID: 7, Action: "diagnose_mistake",
		Context: AgentContext{Question: StructuredQuestion{Stem: "题干", QuestionType: "single_choice"}, LatestAnswer: "B"},
	})
	if err != nil {
		t.Fatalf("agent action: %v", err)
	}
	if response.Status != "completed" || response.Action != "diagnose_mistake" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestChatWithImagesSendsActualBytesInOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") == "" {
			t.Fatal("missing multipart content type")
		}
		form, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		var got [][]byte
		for {
			part, nextErr := form.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				t.Fatal(nextErr)
			}
			data, readErr := io.ReadAll(part)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if part.FormName() == "files" {
				got = append(got, data)
			}
		}
		if len(got) != 2 || string(got[0]) != "real-image-a" || string(got[1]) != "real-image-b" {
			t.Fatalf("image bytes were not forwarded: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"traceId":"t-image","questionId":1,"status":"completed","reply":"看到了图片","cost":{}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, 2*time.Second)
	response, err := client.ChatWithImages(context.Background(), ChatRequest{QuestionID: 1, TraceID: "t-image", Question: StructuredQuestion{Stem: "题干", QuestionType: "subjective"}, Message: "请看图"}, []ChatImage{{FileName: "a.png", ContentType: "image/png", Bytes: []byte("real-image-a")}, {FileName: "b.png", ContentType: "image/png", Bytes: []byte("real-image-b")}})
	if err != nil {
		t.Fatalf("chat with images: %v", err)
	}
	if response.Reply != "看到了图片" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
