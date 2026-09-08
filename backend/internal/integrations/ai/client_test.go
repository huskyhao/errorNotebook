package ai

import (
	"context"
	"encoding/json"
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
