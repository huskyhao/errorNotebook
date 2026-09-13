package services

import "testing"

func TestIsPlaceholderAnalysisAnswer(t *testing.T) {
	tests := []struct {
		name         string
		questionType string
		answer       string
		want         bool
	}{
		{name: "subjective placeholder", questionType: "subjective", answer: "参考答案见解析", want: true},
		{name: "short answer placeholder with punctuation", questionType: "short_answer", answer: "详见解析。", want: true},
		{name: "complete subjective answer", questionType: "subjective", answer: "先中序遍历二叉搜索树，再维护最小差值，并输出所有对应结点。", want: false},
		{name: "single choice remains unchanged", questionType: "single_choice", answer: "A", want: false},
		{name: "empty needs-review answer", questionType: "subjective", answer: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPlaceholderAnalysisAnswer(tt.questionType, tt.answer); got != tt.want {
				t.Fatalf("isPlaceholderAnalysisAnswer(%q, %q) = %v, want %v", tt.questionType, tt.answer, got, tt.want)
			}
		})
	}
}
