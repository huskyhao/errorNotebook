import type { ApiEnvelope, ApiError, QuestionItem, AnalysisItem, ChatMessage } from './types';

export const API_BASE = process.env.REACT_APP_API_BASE_URL ?? 'http://localhost:8080/api/v1';

export function cx(...values: Array<string | false | null | undefined>): string {
  return values.filter(Boolean).join(' ');
}

export async function requestJson<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...(init?.headers ?? {}),
    },
  });

  const payload = (await response.json().catch(() => null)) as ApiEnvelope<T> & ApiError | null;
  if (!response.ok) {
    const message =
      typeof payload?.error === 'string'
        ? payload.error
        : payload?.error?.message ?? `Request failed (${response.status})`;
    throw new Error(message);
  }

  return (payload?.data ?? payload) as T;
}

export function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '';
}

export const QUESTION_TYPE_OPTIONS = [
  'single_choice',
  'multiple_choice',
  'true_false',
  'fill_blank',
  'subjective',
  'short_answer',
  'essay',
  'calculation',
];

const QUESTION_TYPE_LABELS: Record<string, string> = {
  single_choice: '单选题',
  multiple_choice: '多选题',
  true_false: '判断题',
  fill_blank: '填空题',
  subjective: '主观题',
  short_answer: '简答题',
  essay: '解答题',
  calculation: '计算题',
};

export function questionTypeLabel(value?: string | null): string {
  if (!value) return '未标注题型';
  const normalized = value.trim().toLowerCase();
  return QUESTION_TYPE_LABELS[normalized] ?? value;
}

const OCR_STATUS_LABELS: Record<string, string> = {
	queued: 'OCR 排队中',
	pending: '待识别',
	uploaded: '已上传',
	processing: '识别中',
	completed: '已识别',
	failed: '识别失败',
	needs_review: '待校准',
};

export function ocrStatusLabel(value: string): string {
  const normalized = value.trim().toLowerCase();
  return OCR_STATUS_LABELS[normalized] ?? value;
}

const ANALYSIS_STATUS_LABELS: Record<string, string> = {
	queued: '解析排队',
	pending: '待解析',
	processing: '解析中',
	completed: '已解析',
	failed: '解析失败',
	needs_review: '待校准',
};

export function analysisStatusLabel(value?: string | null): string {
  if (!value) return '待解析';
  const normalized = value.trim().toLowerCase();
  return ANALYSIS_STATUS_LABELS[normalized] ?? value;
}

export function buildConversation(question: QuestionItem | null, analysis: AnalysisItem | null): ChatMessage[] {
  if (!question) return [];
  const items: ChatMessage[] = [];
  if (question.userAnswer) {
    items.push({
      id: 1,
      questionId: question.id,
      role: 'user',
      message: `我的答案：${question.userAnswer}`,
      createdAt: '',
    });
  }
  if (analysis?.content.summary) {
    items.push({
      id: 2,
      questionId: question.id,
      role: 'assistant',
      message: analysis.content.summary,
      createdAt: analysis.createdAt,
    });
  }
  return items;
}
