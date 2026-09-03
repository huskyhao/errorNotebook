export type ApiEnvelope<T> = { data: T };
export type ApiError = { error?: { code?: string; message?: string } | string };

export type OptionItem = { key: string; content: string; sortOrder?: number };

export type QualityStatus = 'ok' | 'needs_review';

export type LearningState = {
  id: number;
  questionId: number;
  masteryLevel: number;
  wrongCount: number;
  correctStreak: number;
  lastPracticedAt?: string | null;
  nextReviewAt?: string | null;
  mistakeReason: string;
  weaknessTags: string[];
  reviewAdvice: string[];
};

export type QuestionItem = {
  id: number;
  stem: string;
  questionType: string;
  correctAnswer?: string | null;
  userAnswer?: string | null;
  ocrStatus: string;
  analysisStatus: string;
  sourceType: string;
  rawOcrText?: string | null;
  structureWarnings?: string[];
  structureConfidence?: number | null;
  parseSource?: string;
  qualityStatus?: QualityStatus;
  categoryId?: number | null;
  categoryName?: string | null;
  isFavorited: boolean;
  options: OptionItem[];
  tags?: TagItem[];
  learningState?: LearningState | null;
};

export type CategoryItem = {
  id: number;
  name: string;
  parentId?: number | null;
};

export type TagItem = {
  id: number;
  name: string;
};

export type AnalysisItem = {
  id: number;
  questionId: number;
  provider: string;
  answer?: string | null;
  content: {
    answer?: string;
    summary?: string;
    knowledgePoints?: string[];
    steps?: string[];
    optionAnalysis?: Record<string, string>;
    pitfalls?: string[];
    reviewAdvice?: string[];
  };
  createdAt: string;
};

export type ChatMessage = {
  id: number;
  questionId: number;
  role: 'user' | 'assistant' | 'system';
  message: string;
  attachmentJson?: string | null;
  createdAt: string;
  clientStatus?: 'pending' | 'failed';
};

export type BatchImportItemResult = {
  fileIndex: number;
  fileName: string;
  questionId?: number;
	status: string;
	processingStage?: string;
	error?: string;
};

export type BatchImportResult = {
  batchId: number;
  total: number;
  questions: BatchImportItemResult[];
};

export type BatchProgress = BatchImportResult & {
  completed: number;
  failed: number;
};

export type CategoryTreeNode = {
  id: number;
  name: string;
  parentId?: number | null;
  questionCount: number;
};
export type NavItem = { key: string; label: string; icon: React.ReactNode; path?: string };
export type PromptAction = { key: string; label: string; prompt: string; icon: React.ReactNode };

export type PracticeSessionListItem = {
  id: number;
  name: string;
  status: 'in_progress' | 'submitted';
  totalCount: number;
  correctCount: number;
  createdAt: string;
};

export type PracticeSessionQDetail = {
  id: number;
  orderIndex: number;
  userAnswer?: string | null;
  isCorrect?: boolean | null;
  status: 'unanswered' | 'answered' | 'skipped' | 'correct' | 'wrong' | 'manual_required' | 'ungraded';
  gradingStatus?: 'ungraded' | 'auto_graded' | 'manual_required' | 'llm_suggested';
  gradingComment?: string | null;
  question: QuestionBrief;
};

export type QuestionBrief = {
  id: number;
  stem: string;
  questionType: string;
  options: OptionItem[];
};

export type PracticeSessionDetail = {
  id: number;
  name: string;
  status: 'in_progress' | 'submitted';
  totalCount: number;
  correctCount: number;
  createdAt: string;
  questions: PracticeSessionQDetail[];
};

export type PracticeSessionQResult = {
  id: number;
  orderIndex: number;
  userAnswer?: string | null;
  correctAnswer?: string | null;
  isCorrect?: boolean | null;
  status: 'unanswered' | 'answered' | 'skipped' | 'correct' | 'wrong' | 'manual_required' | 'ungraded';
  score?: number | null;
  maxScore?: number | null;
  gradingStatus?: 'ungraded' | 'auto_graded' | 'manual_required' | 'llm_suggested';
  gradingComment?: string | null;
  question: QuestionFull;
};

export type QuestionFull = {
  id: number;
  stem: string;
  questionType: string;
  correctAnswer?: string | null;
  options: OptionItem[];
};

export type PracticeSessionResult = {
  id: number;
  name: string;
  status: 'submitted';
  totalCount: number;
  correctCount: number;
  scorePercent: number;
  createdAt: string;
  questions: PracticeSessionQResult[];
};

export type PracticeRecommendationGroup = {
  key: 'today_review' | 'recent_wrong' | 'weak_points' | 'new_questions' | 'mixed_random';
  title: string;
  reason: string;
  questionIds: number[];
  count: number;
};

export type DetailOption = { key: string; content: string; isCorrect?: boolean; isWrong?: boolean };
export type AnalysisSection = { key: string; title: string; icon: React.ReactNode; body?: React.ReactNode; expanded?: boolean };
