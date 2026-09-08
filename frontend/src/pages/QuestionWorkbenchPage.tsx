import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { requestJson, buildConversation } from '../utils';
import type { QuestionItem, AnalysisItem, ChatMessage, TagItem, CategoryTreeNode, BatchImportResult, BatchProgress, OptionItem, LearningState, AgentActionName, AgentActionResponse } from '../types';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import QuestionSidebar from '../components/QuestionSidebar';
import ReasoningCenter from '../components/ReasoningCenter';
import DetailPanel from '../components/DetailPanel';
import ResizableDivider from '../components/ResizableDivider';
import ConfirmDialog from '../components/ConfirmDialog';

const SIDEBAR_MIN = 320;
const SIDEBAR_MAX = 500;
const SIDEBAR_DEFAULT = 300;
const DETAIL_MIN = 450;
const DETAIL_MAX = 700;
const DETAIL_DEFAULT = 420;

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function normalizeDraftOptions(options: OptionItem[]): OptionItem[] {
  const byKey = new Map<string, OptionItem>();
  for (const option of options) {
    const key = option.key.trim().toUpperCase();
    const content = option.content.trim();
    if (!key && !content) continue;
    byKey.set(key || String.fromCharCode(65 + byKey.size), {
      key: key || String.fromCharCode(65 + byKey.size),
      content,
    });
  }
  return Array.from(byKey.values()).map((option, index) => ({
    ...option,
    sortOrder: index + 1,
  }));
}

export default function QuestionWorkbenchPage() {
  const [searchParams] = useSearchParams();
  const routeQuestionId = Number(searchParams.get('questionId') ?? 0) || null;
  const [questions, setQuestions] = useState<QuestionItem[]>([]);
  const [selectedQuestion, setSelectedQuestion] = useState<QuestionItem | null>(null);
  const [analysis, setAnalysis] = useState<AnalysisItem | null>(null);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [reply, setReply] = useState('');
  const [chatAttachments, setChatAttachments] = useState<File[]>([]);
  const [sendingMessage, setSendingMessage] = useState(false);
  const [loading, setLoading] = useState(true);
  const [importing, setImporting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draftStem, setDraftStem] = useState('');
  const [draftAnswer, setDraftAnswer] = useState('');
  const [draftQuestionType, setDraftQuestionType] = useState('subjective');
  const [draftOptions, setDraftOptions] = useState<OptionItem[]>([]);
  const [generatingLearning, setGeneratingLearning] = useState(false);
  const [agentActionPending, setAgentActionPending] = useState(false);
  const [activeProposalId, setActiveProposalId] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<number | null>(null);
  const [categoryDeleteTarget, setCategoryDeleteTarget] = useState<CategoryTreeNode | null>(null);
  const [deletingCategoryId, setDeletingCategoryId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [detailOpen, setDetailOpen] = useState(true);
  const [sidebarWidth, setSidebarWidth] = useState(SIDEBAR_DEFAULT);
  const [detailWidth, setDetailWidth] = useState(DETAIL_DEFAULT);
  const [isResizing, setIsResizing] = useState(false);
  const [practiceMode, setPracticeMode] = useState<Record<number, boolean>>({});
  const [selectedOptions, setSelectedOptions] = useState<Record<number, string>>({});
  const [submittingAnswer, setSubmittingAnswer] = useState(false);
  const [reanalyzing, setReanalyzing] = useState(false);
  const [batchProgress, setBatchProgress] = useState<BatchProgress | null>(null);

  const [categoryTree, setCategoryTree] = useState<CategoryTreeNode[]>([]);
  const [uncategorizedCount, setUncategorizedCount] = useState(0);
  const [totalCount, setTotalCount] = useState(0);
  const [allTags, setAllTags] = useState<TagItem[]>([]);
  const [tagFilter, setTagFilter] = useState<number | null>(null);
  const [favoritedFilter, setFavoritedFilter] = useState(false);
  const [reviewFilter, setReviewFilter] = useState(false);
  const initialLoadDone = useRef(false);
  const selectedQuestionIdRef = useRef<number | null>(null);

  useEffect(() => {
    selectedQuestionIdRef.current = selectedQuestion?.id ?? null;
  }, [selectedQuestion?.id]);

  function buildFilterQuery(): string {
    const params = new URLSearchParams();
    if (tagFilter) params.set('tagIds', String(tagFilter));
    if (favoritedFilter) params.set('isFavorited', 'true');
    const qs = params.toString();
    return qs ? `?${qs}` : '';
  }

  async function loadQuestions() {
    setLoading(true);
    setError('');
    try {
      const qs = buildFilterQuery();
      const items = await requestJson<QuestionItem[]>(`/questions${qs}`);
      setQuestions(items);
      const routedQuestion = routeQuestionId ? items.find((item) => item.id === routeQuestionId) : null;
      if (routedQuestion) {
        setSelectedQuestion(routedQuestion);
      } else if (!selectedQuestion && items.length) {
        setSelectedQuestion(items[0]);
      } else if (selectedQuestion && !items.some((item) => item.id === selectedQuestion.id) && items.length) {
        setSelectedQuestion(items[0]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
      setQuestions([]);
      setSelectedQuestion(null);
    } finally {
      setLoading(false);
    }
  }

  async function loadQuestionDetail(questionId: number) {
    const detail = await requestJson<QuestionItem>(`/questions/${questionId}`);
    setSelectedQuestion(detail);
    setDraftStem(detail.stem);
    setDraftAnswer(detail.correctAnswer ?? '');
    setDraftQuestionType(detail.questionType);
    setDraftOptions([...(detail.options ?? [])].sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0)));
    const analysisResult = await requestJson<AnalysisItem>(`/questions/${questionId}/analysis`).catch(() => null);
    setAnalysis(analysisResult);
    const messages = await requestJson<ChatMessage[]>(`/questions/${questionId}/chat`).catch(() => []);
    setChatMessages(messages.length > 0 ? messages : buildConversation(detail, analysisResult));
  }

  async function loadTaxonomy() {
    try {
      const resp = await requestJson<{ categories: CategoryTreeNode[]; uncategorized: number }>('/categories/tree');
      setCategoryTree(resp.categories);
      setUncategorizedCount(resp.uncategorized);
      const total = resp.categories.reduce((sum, c) => sum + c.questionCount, 0) + resp.uncategorized;
      setTotalCount(total);
    } catch {
      // taxonomy unavailable
    }
    try {
      const tags = await requestJson<TagItem[]>('/tags');
      setAllTags(tags);
    } catch {
      // taxonomy unavailable
    }
  }

  useEffect(() => {
    loadQuestions().catch(() => null);
    loadTaxonomy();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Reload when taxonomy filters change (skip initial mount)
  useEffect(() => {
    if (!initialLoadDone.current) {
      initialLoadDone.current = true;
      return;
    }
    loadQuestions().catch(() => null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tagFilter, favoritedFilter]);

  useEffect(() => {
    if (!selectedQuestion) return;
    setChatAttachments([]);
    loadQuestionDetail(selectedQuestion.id).catch((err) => setError(err instanceof Error ? err.message : '加载详情失败'));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedQuestion?.id]);

  useEffect(() => {
    if (!routeQuestionId || selectedQuestion?.id === routeQuestionId) return;
    loadQuestionDetail(routeQuestionId).catch((err) => setError(err instanceof Error ? err.message : '加载详情失败'));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeQuestionId]);

  async function handleImport(files: File[]) {
    // If single file, use existing import endpoint for faster UX
    if (files.length === 1) {
      setImporting(true);
      setError('');
      try {
        const formData = new FormData();
        formData.append('file', files[0]);
        formData.append('sourceType', 'image');
        const result = await requestJson<{ jobId: string; questionId: number; status: string }>('/questions/import', {
          method: 'POST',
          body: formData,
        });
        await loadQuestions();
        await loadQuestionDetail(result.questionId);
        setImporting(false);

        for (let i = 0; i < 60; i++) {
          await new Promise((r) => setTimeout(r, 5000));
          const q = await requestJson<QuestionItem>(`/questions/${result.questionId}`).catch(() => null);
          if (!q) break;
          if (q.ocrStatus === 'failed') {
            setError('OCR 识别失败，可在题目详情中重试识别');
            setSelectedQuestion(q);
            await loadQuestions();
            setImporting(false);
            return;
          }
          if (q.ocrStatus === 'needs_review') {
            setError('OCR 已完成，但题面需要人工校准');
            setSelectedQuestion(q);
            await loadQuestions();
            setImporting(false);
            return;
          }
          if (q.analysisStatus === 'completed') {
            const analysisResult = await requestJson<AnalysisItem>(`/questions/${result.questionId}/analysis`).catch(() => null);
            setAnalysis(analysisResult);
            setSelectedQuestion(q);
            setChatMessages(buildConversation(q, analysisResult));
            setImporting(false);
            return;
          }
          if (q.analysisStatus === 'failed') {
            setError('AI 解析失败，请重新分析');
            setSelectedQuestion(q);
            await loadQuestions();
            setImporting(false);
            return;
          }
        }
        setError('解析超时，请检查后重试');
        setImporting(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : '导入失败');
        setImporting(false);
      }
      return;
    }

    // Batch import for multiple files
    setImporting(true);
    setError('');
    try {
      const formData = new FormData();
      for (const file of files) {
        formData.append('files', file);
      }
      formData.append('sourceType', 'image');

      const result = await requestJson<BatchImportResult>('/questions/batch-import', {
        method: 'POST',
        body: formData,
      });

      setBatchProgress({ ...result, completed: 0, failed: 0 });

      // Poll batch progress
      let pollCount = 0;
      const maxPollCount = 90;
      const pollTimer = setInterval(async () => {
        pollCount += 1;
        try {
          const batch = await requestJson<BatchImportResult>(`/batch-imports/${result.batchId}`);
          const completed = batch.questions.filter((q) => q.status === 'completed' || q.status === 'needs_review').length;
          const failed = batch.questions.filter((q) => q.status === 'failed').length;
          const totalDone = completed + failed;
          setBatchProgress({ ...batch, completed: totalDone, failed });

          if (batch.questions.every((q) => q.status === 'completed' || q.status === 'failed' || q.status === 'needs_review')) {
            clearInterval(pollTimer);
            setImporting(false);
            setBatchProgress(null);
            await loadQuestions();
            const firstSuccess = batch.questions.find((q) => q.status === 'completed' || q.status === 'needs_review');
            if (firstSuccess?.questionId) {
              await loadQuestionDetail(firstSuccess.questionId);
            }
            if (failed > 0) {
              setError(`导入完成：${failed}/${batch.total} 个文件失败`);
            }
            return;
          }

          if (pollCount >= maxPollCount) {
            clearInterval(pollTimer);
            setImporting(false);
            setBatchProgress(null);
            await loadQuestions();
            const stuck = batch.questions.filter((q) => q.status !== 'completed' && q.status !== 'failed' && q.status !== 'needs_review');
            setError(`导入超时：${stuck.length}/${batch.total} 个文件仍未完成，请稍后刷新查看`);
          }
        } catch {
          clearInterval(pollTimer);
          setImporting(false);
          setBatchProgress(null);
          setError('获取批次进度失败');
        }
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : '批量导入失败');
      setImporting(false);
    }
  }

  async function handleSendMessage(messageOverride?: string, attachmentsOverride?: File[], idempotencyKeyOverride?: string) {
    if (!selectedQuestion) return;
    const message = (messageOverride ?? reply).trim();
    const filesToSend = attachmentsOverride ?? chatAttachments;
    if (!message && filesToSend.length === 0) return;
    const questionId = selectedQuestion.id;
    const createdAt = new Date().toISOString();
    const idempotencyKey = idempotencyKeyOverride ?? `chat-${questionId}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const attachmentNames = filesToSend.map((file) => file.name);
    const localUserMessage: ChatMessage = {
      id: -Date.now(),
      questionId,
      role: 'user',
      message: attachmentNames.length > 0 ? `${message || '追问附图'}\n[附图] ${attachmentNames.join('、')}` : message,
      createdAt,
      clientStatus: 'pending',
      idempotencyKey,
    };
    const pendingAssistantMessage: ChatMessage = {
      id: localUserMessage.id - 1,
      questionId,
      role: 'assistant',
      message: '正在思考...',
      createdAt,
      clientStatus: 'pending',
    };

    setReply('');
    setChatAttachments([]);
    setError('');
    setSendingMessage(true);
    setChatMessages((prev) => [...prev, localUserMessage, pendingAssistantMessage]);
    try {
      const request =
        filesToSend.length > 0
          ? (() => {
              const formData = new FormData();
              formData.append('message', message);
              formData.append('idempotencyKey', idempotencyKey);
              filesToSend.forEach((file) => formData.append('attachments', file));
              return { method: 'POST', body: formData };
            })()
          : { method: 'POST', body: JSON.stringify({ message, idempotencyKey }) };
      const messages = await requestJson<ChatMessage[]>(`/questions/${questionId}/chat`, request);
      if (selectedQuestionIdRef.current === questionId) {
        setChatMessages(messages);
      }
    } catch (err) {
      if (selectedQuestionIdRef.current === questionId) {
        setChatMessages((prev) =>
          prev
            .filter((item) => item.id !== pendingAssistantMessage.id)
            .map((item) =>
              item.id === localUserMessage.id
                ? { ...item, clientStatus: 'failed' }
                : item,
            ),
        );
      }
      setError(err instanceof Error ? err.message : '发送失败');
      setChatAttachments(filesToSend);
    } finally {
      setSendingMessage(false);
    }
  }

  function handleAddChatAttachments(files: File[]) {
    const imageFiles = files.filter((file) => file.type.startsWith('image/'));
    if (imageFiles.length !== files.length) {
      setError('追问附件只支持图片');
    }
    if (imageFiles.length === 0) return;
    setChatAttachments((prev) => [...prev, ...imageFiles].slice(0, 4));
  }

  function handleRetryMessage(message: ChatMessage) {
    const text = message.message.replace(/\n\[附图\].*$/s, '').trim();
    setError('');
    void handleSendMessage(text, [], message.idempotencyKey);
  }

  function handleRemoveChatAttachment(index: number) {
    setChatAttachments((prev) => prev.filter((_, idx) => idx !== index));
  }

  async function handleReanalyze() {
    if (!selectedQuestion) return;
    setReanalyzing(true);
    setError('');
    try {
      await requestJson<{ jobId: string; questionId: number; status: string }>(
        `/questions/${selectedQuestion.id}/analyze`,
        { method: 'POST', body: JSON.stringify({}) },
      );
      for (let i = 0; i < 72; i++) {
        await new Promise((r) => setTimeout(r, 5000));
        const q = await requestJson<QuestionItem>(`/questions/${selectedQuestion.id}`);
        if (q.analysisStatus === 'completed') {
          const analysisResult = await requestJson<AnalysisItem>(`/questions/${selectedQuestion.id}/analysis`);
          setAnalysis(analysisResult);
          setSelectedQuestion(q);
          setChatMessages(buildConversation(q, analysisResult));
          return;
        }
        if (q.analysisStatus === 'failed') {
          setError('AI 解析失败，请重试');
          setSelectedQuestion(q);
          await loadQuestions();
          return;
        }
      }
      setError('解析超时，请检查后重试');
    } catch (err) {
      setError(err instanceof Error ? err.message : '解析失败');
    } finally {
      setReanalyzing(false);
    }
  }

  async function handleRetryOCR() {
    if (!selectedQuestion) return;
    setError('');
    try {
      const questionId = selectedQuestion.id;
      await requestJson(`/questions/${questionId}/ocr/retry`, { method: 'POST', body: JSON.stringify({}) });
      await loadQuestionDetail(questionId);
      await loadQuestions();
    } catch (err) {
      setError(err instanceof Error ? err.message : '重试识别失败');
    }
  }

  async function handleSaveQuestion() {
    if (!selectedQuestion) return;
    const questionId = selectedQuestion.id;
    setSaving(true);
    setError('');
    try {
      await requestJson<QuestionItem>(`/questions/${questionId}`, {
        method: 'PATCH',
        body: JSON.stringify({
          stem: draftStem,
          questionType: draftQuestionType,
          correctAnswer: draftAnswer,
          options: normalizeDraftOptions(draftOptions),
        }),
      });
      await loadQuestionDetail(questionId);
      await loadQuestions();
      setEditing(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  async function handleGenerateLearningState() {
    if (!selectedQuestion) return;
    setGeneratingLearning(true);
    setError('');
    try {
      const state = await requestJson<LearningState>(`/questions/${selectedQuestion.id}/learning-state/generate`, {
        method: 'POST',
        body: JSON.stringify({}),
      });
      setSelectedQuestion((prev) => (prev ? { ...prev, learningState: state } : prev));
      await loadQuestions();
    } catch (err) {
      setError(err instanceof Error ? err.message : '错因归纳失败');
    } finally {
      setGeneratingLearning(false);
    }
  }

  async function handleAgentAction(action: AgentActionName, params: Record<string, unknown> = {}) {
    if (!selectedQuestion) return;
    setError('');
    setAgentActionPending(true);
    try {
      const result = await requestJson<AgentActionResponse>(`/questions/${selectedQuestion.id}/agent-actions`, {
        method: 'POST',
        body: JSON.stringify({ action, params }),
      });
      if (!['completed', 'needs_review'].includes(result.status) || !result.result) {
        setError(result.error?.message ?? '该动作需要补充信息或人工复核');
        return;
      }
      const data = result.result;
      if (data.proposalId) setActiveProposalId(data.proposalId);
      const content = data.stem
        ? `相似题候选（${data.qualityStatus === 'needs_review' ? '待复核' : '待确认'}）\n\n${data.stem}\n\n答案：${data.answer ?? '待复核'}\n解析：${data.analysis ?? ''}\n变化策略：${data.variationStrategy ?? ''}`
        : data.suggestedScore !== undefined
          ? `AI 建议分数：${data.suggestedScore}/${data.maxScore}\n\n${data.feedback ?? ''}\n缺失要点：${data.missingPoints?.join('；') ?? '—'}\n不确定项：${data.uncertainties?.join('；') ?? '—'}`
          : data.explanation
        ? `${data.explanation}\n\n${data.focusPoints?.join('；') ?? ''}\n${data.checkQuestion ?? ''}`
        : data.hint
          ? `第 ${data.hintLevel ?? 1} 级提示：${data.hint}\n\n${data.nextQuestion ?? ''}`
          : `${data.mistakeReason ?? ''}\n\n复习建议：${data.reviewAdvice?.join('；') ?? ''}`;
      setChatMessages((prev) => [...prev, { id: -Date.now(), questionId: selectedQuestion.id, role: 'assistant', message: content, createdAt: new Date().toISOString() }]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Agent 动作失败');
    } finally {
      setAgentActionPending(false);
    }
  }

  async function handleConfirmProposal() {
    if (!selectedQuestion || !activeProposalId) return;
    try {
      const created = await requestJson<QuestionItem>(`/questions/${selectedQuestion.id}/ai-proposals/${activeProposalId}/confirm`, { method: 'POST', body: JSON.stringify({}) });
      setActiveProposalId(null);
      setChatMessages((prev) => [...prev, { id: -Date.now(), questionId: selectedQuestion.id, role: 'assistant', message: `已确认创建相似题：题目 #${created.id}`, createdAt: new Date().toISOString() }]);
      await loadQuestions();
    } catch (err) { setError(err instanceof Error ? err.message : '确认候选失败'); }
  }

  async function handleRejectProposal() {
    if (!selectedQuestion || !activeProposalId) return;
    try { await requestJson(`/questions/${selectedQuestion.id}/ai-proposals/${activeProposalId}/reject`, { method: 'POST', body: JSON.stringify({}) }); setActiveProposalId(null); }
    catch (err) { setError(err instanceof Error ? err.message : '放弃候选失败'); }
  }

  async function handleApplyTaxonomySuggestion() {
    if (!selectedQuestion) return;
    try {
      const updated = await requestJson<QuestionItem>(`/questions/${selectedQuestion.id}/taxonomy-suggestion/apply`, { method: 'POST', body: JSON.stringify({}) });
      setSelectedQuestion(updated);
      setQuestions((prev) => prev.map((item) => item.id === updated.id ? updated : item));
      await loadTaxonomy();
    } catch (err) {
      setError(err instanceof Error ? err.message : '应用分类建议失败');
    }
  }


  async function handleSelectOption(optionKey: string) {
    if (!selectedQuestion) return;
    setSelectedOptions((prev) => ({ ...prev, [selectedQuestion.id]: optionKey }));
    setSubmittingAnswer(true);
    try {
      await requestJson<QuestionItem>(`/questions/${selectedQuestion.id}/answer`, {
        method: 'POST',
        body: JSON.stringify({ userAnswer: optionKey }),
      });
    } catch (err) {
      console.log('[Detail] 提交答案失败:', err);
    } finally {
      setSubmittingAnswer(false);
    }
    setPracticeMode((prev) => {
      const next = { ...prev };
      delete next[selectedQuestion.id];
      return next;
    });
  }

  async function handleDeleteQuestion() {
    if (!deleteConfirmId) return;
    setDeletingId(deleteConfirmId);
    setDeleteConfirmId(null);
    setError('');
    try {
      await requestJson<null>(`/questions/${deleteConfirmId}`, { method: 'DELETE' });
      setDeletingId(null);
      if (selectedQuestion?.id === deleteConfirmId) {
        setSelectedQuestion(null);
        setAnalysis(null);
        setChatMessages([]);
      }
      await loadQuestions();
    } catch (err) {
      setDeletingId(null);
      setError(err instanceof Error ? err.message : '删除失败');
    }
  }

  async function handleDeleteCategory() {
    if (!categoryDeleteTarget) return;
    const categoryId = categoryDeleteTarget.id;
    setDeletingCategoryId(categoryId);
    setError('');
    try {
      await requestJson<null>(`/categories/${categoryId}`, { method: 'DELETE' });
      setQuestions((prev) =>
        prev.map((item) => (item.categoryId === categoryId ? { ...item, categoryId: null, categoryName: null } : item)),
      );
      if (selectedQuestion?.categoryId === categoryId) {
        setSelectedQuestion((prev) => (prev ? { ...prev, categoryId: null, categoryName: null } : prev));
      }
      setCategoryDeleteTarget(null);
      await loadTaxonomy();
      await loadQuestions();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除分类失败');
    } finally {
      setDeletingCategoryId(null);
    }
  }

  async function handleToggleFavorite(questionId: number) {
    const question = questions.find((q) => q.id === questionId);
    if (!question) return;
    const newVal = !question.isFavorited;
    try {
      const updated = await requestJson<QuestionItem>(`/questions/${questionId}/favorite`, {
        method: 'POST',
        body: JSON.stringify({ isFavorited: newVal }),
      });
      setQuestions((prev) => prev.map((q) => (q.id === questionId ? updated : q)));
      if (selectedQuestion?.id === questionId) {
        setSelectedQuestion(updated);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '收藏操作失败');
    }
  }

  async function handleUpdateCategory(categoryId: number | null) {
    if (!selectedQuestion) return;
    try {
      const updated = await requestJson<QuestionItem>(`/questions/${selectedQuestion.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ categoryId }),
      });
      setSelectedQuestion(updated);
      setQuestions((prev) => prev.map((q) => (q.id === updated.id ? updated : q)));
    } catch (err) {
      setError(err instanceof Error ? err.message : '分类更新失败');
    }
  }

  async function handleTagsChange(tagIds: number[]) {
    if (!selectedQuestion) return;
    try {
      const updated = await requestJson<QuestionItem>(`/questions/${selectedQuestion.id}/tags`, {
        method: 'POST',
        body: JSON.stringify({ tagIds }),
      });
      setSelectedQuestion(updated);
      setQuestions((prev) => prev.map((q) => (q.id === updated.id ? updated : q)));
      // Refresh tag list in case new tags were created
      loadTaxonomy();
    } catch (err) {
      setError(err instanceof Error ? err.message : '标签更新失败');
    }
  }

  async function handleCreateCategory(name: string) {
    try {
      const resp = await requestJson<{ id: number; name: string }>('/categories', {
        method: 'POST',
        body: JSON.stringify({ name }),
      });
      await loadTaxonomy();
      return { id: resp.id, name: resp.name, questionCount: 0, parentId: null };
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建分类失败');
      return null;
    }
  }

  async function handleMoveQuestion(questionId: number, categoryId: number | null) {
    try {
      await requestJson<object>(`/questions/${questionId}`, {
        method: 'PATCH',
        body: JSON.stringify({ categoryId }),
      });
      setQuestions((prev) =>
        prev.map((q) => (q.id === questionId ? { ...q, categoryId } : q))
      );
      if (selectedQuestion?.id === questionId) {
        setSelectedQuestion((prev) => (prev ? { ...prev, categoryId } : prev));
      }
      loadTaxonomy();
    } catch (err) {
      setError(err instanceof Error ? err.message : '移动题目失败');
    }
  }

  function handleTagFilter(tagId: number | null) {
    setTagFilter(tagId);
  }

  function handleFavoritedFilter(val: boolean) {
    setFavoritedFilter(val);
  }

  const handleSidebarResize = useCallback((delta: number) => {
    setSidebarWidth((prev) => clamp(prev + delta, SIDEBAR_MIN, SIDEBAR_MAX));
  }, []);

  const handleDetailResize = useCallback((delta: number) => {
    setDetailWidth((prev) => clamp(prev - delta, DETAIL_MIN, DETAIL_MAX));
  }, []);

  const gridCols = `80px ${sidebarOpen ? `minmax(${SIDEBAR_MIN}px, ${sidebarWidth}px)` : '36px'} 6px minmax(460px, 1fr) 6px ${detailOpen ? `minmax(${DETAIL_MIN}px, ${detailWidth}px)` : '36px'}`;

  return (
    <div className="app-shell">
      <TopBar onImport={handleImport} importing={importing} importProgress={batchProgress} />
      <div
        className={`workspace${!sidebarOpen ? ' sidebar-collapsed' : ''}${!detailOpen ? ' detail-collapsed' : ''}${isResizing ? ' is-resizing' : ''}`}
        style={{ gridTemplateColumns: gridCols }}
      >
        <SideNavigation />
        <QuestionSidebar
          questions={questions}
          activeId={selectedQuestion?.id ?? null}
          onSelect={setSelectedQuestion}
          onDelete={handleDeleteQuestion}
          deletingId={deletingId}
          deleteConfirmId={deleteConfirmId}
          setDeleteConfirmId={setDeleteConfirmId}
          searchQuery={searchQuery}
          setSearchQuery={setSearchQuery}
          sidebarOpen={sidebarOpen}
          onToggleSidebar={() => setSidebarOpen((v) => !v)}
          categoryTree={categoryTree}
          uncategorizedCount={uncategorizedCount}
          totalCount={totalCount}
          tags={allTags}
          activeTagFilter={tagFilter}
          onTagFilter={handleTagFilter}
          favoritedFilter={favoritedFilter}
          onFavoritedFilter={handleFavoritedFilter}
          reviewFilter={reviewFilter}
          onReviewFilter={setReviewFilter}
          onToggleFavorite={handleToggleFavorite}
          onCreateCategory={handleCreateCategory}
          onRequestDeleteCategory={setCategoryDeleteTarget}
          onMoveQuestion={handleMoveQuestion}
        />
        <ResizableDivider
          onResize={handleSidebarResize}
          disabled={!sidebarOpen}
          onDragStart={() => setIsResizing(true)}
          onDragEnd={() => setIsResizing(false)}
        />
        <ReasoningCenter
          question={selectedQuestion}
          analysis={analysis}
          messages={chatMessages}
          reply={reply}
          setReply={setReply}
          onSendMessage={handleSendMessage}
          onRetryMessage={handleRetryMessage}
          onReanalyze={handleReanalyze}
          onGenerateLearningState={handleGenerateLearningState}
          onAgentAction={handleAgentAction}
          onApplyTaxonomySuggestion={handleApplyTaxonomySuggestion}
          onConfirmProposal={handleConfirmProposal}
          onRejectProposal={handleRejectProposal}
          activeProposalId={activeProposalId}
          attachments={chatAttachments}
          onAddAttachments={handleAddChatAttachments}
          onRemoveAttachment={handleRemoveChatAttachment}
          reanalyzing={reanalyzing}
          sendingMessage={sendingMessage}
          generatingLearning={generatingLearning}
          agentActionPending={agentActionPending}
        />
        <ResizableDivider
          onResize={handleDetailResize}
          disabled={!detailOpen}
          onDragStart={() => setIsResizing(true)}
          onDragEnd={() => setIsResizing(false)}
        />
        <DetailPanel
          question={selectedQuestion}
          analysis={analysis}
          editing={editing}
          setEditing={setEditing}
          draftStem={draftStem}
          setDraftStem={setDraftStem}
          draftAnswer={draftAnswer}
          setDraftAnswer={setDraftAnswer}
          draftQuestionType={draftQuestionType}
          setDraftQuestionType={setDraftQuestionType}
          draftOptions={draftOptions}
          setDraftOptions={setDraftOptions}
          onSaveQuestion={handleSaveQuestion}
          onRetryOCR={handleRetryOCR}
          onRetryAnalysis={handleReanalyze}
          detailOpen={detailOpen}
          onToggleDetail={() => setDetailOpen((v) => !v)}
          showAnswer={!practiceMode[selectedQuestion?.id ?? 0]}
          userSelectedOption={selectedOptions[selectedQuestion?.id ?? 0] ?? null}
          onSelectOption={handleSelectOption}
          onToggleAnswer={() => {
            const qid = selectedQuestion?.id;
            if (!qid) return;
            if (!practiceMode[qid]) {
              setPracticeMode((prev) => ({ ...prev, [qid]: true }));
              setSelectedOptions((prev) => {
                const next = { ...prev };
                delete next[qid];
                return next;
              });
            } else {
              setPracticeMode((prev) => {
                const next = { ...prev };
                delete next[qid];
                return next;
              });
            }
          }}
          submittingAnswer={submittingAnswer}
          categories={categoryTree}
          allTags={allTags}
          onToggleFavorite={() => {
            if (selectedQuestion) handleToggleFavorite(selectedQuestion.id);
          }}
          onCategoryChange={handleUpdateCategory}
          onTagsChange={handleTagsChange}
        />
      </div>

      {loading ? <div className="global-toast">正在加载题目...</div> : null}
      {saving ? <div className="global-toast">正在保存...</div> : null}
      {reanalyzing ? <div className="global-toast">正在解析...</div> : null}
      {error ? <div className="global-toast is-error">{error}</div> : null}
      {categoryDeleteTarget ? (
        <ConfirmDialog
          title="删除分类"
          description={`确定删除“${categoryDeleteTarget.name}”？分类下的题目不会被删除，会移动到未分类。`}
          confirmText="确认删除"
          danger
          loading={deletingCategoryId === categoryDeleteTarget.id}
          onCancel={() => setCategoryDeleteTarget(null)}
          onConfirm={handleDeleteCategory}
        />
      ) : null}
    </div>
  );
}
