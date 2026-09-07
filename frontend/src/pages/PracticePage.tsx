import React, { useEffect, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { questionTypeLabel, requestJson } from '../utils';
import type {
  QuestionItem, CategoryTreeNode, BatchImportResult, BatchProgress,
  PracticeSessionDetail, PracticeSessionResult,
  PracticeRecommendationGroup,
} from '../types';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import MarkdownRenderer from '../components/MarkdownRenderer';

type Phase = 'setup' | 'quiz' | 'results';

const PRACTICE_TYPES = new Set([
  'single_choice',
  'multiple_choice',
  'true_false',
  'fill_blank',
  'subjective',
  'short_answer',
  'essay',
  'calculation',
]);

function isPracticeQuestion(q: QuestionItem): boolean {
  return PRACTICE_TYPES.has(q.questionType);
}

function isTextQuestion(questionType: string): boolean {
  return ['fill_blank', 'subjective', 'short_answer', 'essay', 'calculation'].includes(questionType);
}

function toggleMultiAnswer(current: string | null | undefined, key: string): string {
  const parts = new Set((current ?? '').split(',').map((x) => x.trim()).filter(Boolean));
  if (parts.has(key)) {
    parts.delete(key);
  } else {
    parts.add(key);
  }
  return Array.from(parts).sort().join(',');
}

export default function PracticePage() {
  const { sessionId } = useParams<{ sessionId?: string }>();
  const navigate = useNavigate();
  const [phase, setPhase] = useState<Phase>('setup');

  // ---- Setup state ----
  const [allQuestions, setAllQuestions] = useState<QuestionItem[]>([]);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [categoryTree, setCategoryTree] = useState<CategoryTreeNode[]>([]);
  const [activeCategoryId, setActiveCategoryId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [importing, setImporting] = useState(false);
  const [batchProgress, setBatchProgress] = useState<BatchProgress | null>(null);
  const [error, setError] = useState('');
  const [recommendations, setRecommendations] = useState<PracticeRecommendationGroup[]>([]);
  const [startingRecommendationKey, setStartingRecommendationKey] = useState<string | null>(null);

  // ---- Quiz state ----
  const [session, setSession] = useState<PracticeSessionDetail | null>(null);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [draftAnswers, setDraftAnswers] = useState<Record<number, string>>({});

  // ---- Results state ----
  const [results, setResults] = useState<PracticeSessionResult | null>(null);

  async function loadQuestions(categoryId: number | null = activeCategoryId) {
    setLoading(true);
    setError('');
    try {
      const params = new URLSearchParams();
      if (categoryId) params.set('categoryId', String(categoryId));
      const qs = params.toString();
      const items = await requestJson<QuestionItem[]>(`/questions${qs ? `?${qs}` : ''}`);
      setAllQuestions(items);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载题目失败');
    } finally {
      setLoading(false);
    }
  }

  async function loadCategories() {
    try {
      const resp = await requestJson<{ categories: CategoryTreeNode[] }>('/categories/tree');
      setCategoryTree(resp.categories);
    } catch {
      // taxonomy unavailable
    }
  }

  async function loadRecommendations() {
    try {
      const groups = await requestJson<PracticeRecommendationGroup[]>('/recommendations/practice');
      setRecommendations(groups);
    } catch {
      setRecommendations([]);
    }
  }

  useEffect(() => {
    loadQuestions();
    loadCategories();
    loadRecommendations();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    loadQuestions(activeCategoryId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeCategoryId]);

  // Resume session if sessionId is in the URL
  useEffect(() => {
    if (!sessionId) return;
    const id = parseInt(sessionId, 10);
    if (isNaN(id)) return;
    requestJson<PracticeSessionDetail>(`/practice-sessions/${id}`)
      .then((detail) => {
        if (detail.status === 'submitted') {
          return requestJson<PracticeSessionResult>(`/practice-sessions/${id}/results`).then((r) => {
            setResults(r);
            setPhase('results');
          });
        } else {
          setSession(detail);
          setPhase('quiz');
        }
      })
      .catch(() => {
        setError('加载做题会话失败');
      });
  }, [sessionId]);

  // Filtered questions: all supported practice question types, then apply search
  const practiceQuestions = allQuestions.filter(isPracticeQuestion);
  const filteredQuestions = searchQuery.trim()
    ? practiceQuestions.filter((q) => q.stem.toLowerCase().includes(searchQuery.toLowerCase()))
    : practiceQuestions;

  function toggleQuestion(id: number) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  function selectAll() {
    setSelectedIds(new Set(filteredQuestions.map((q) => q.id)));
  }

  function deselectAll() {
    setSelectedIds(new Set());
  }

  // ---- Import (reuses same logic as QuestionWorkbenchPage) ----
  async function handleImport(files: File[]) {
    if (files.length === 0) return;
    setImporting(true);
    setError('');

    if (files.length === 1) {
      try {
        const formData = new FormData();
        formData.append('file', files[0]);
        formData.append('sourceType', 'image');
        const result = await requestJson<{ jobId: string; questionId: number; status: string }>('/questions/import', {
          method: 'POST',
          body: formData,
        });
        // Poll for completion
        for (let i = 0; i < 60; i++) {
          await new Promise((r) => setTimeout(r, 5000));
          const q = await requestJson<QuestionItem>(`/questions/${result.questionId}`).catch(() => null);
          if (!q) break;
          if (q.analysisStatus === 'completed' || q.analysisStatus === 'failed') {
            break;
          }
        }
        setImporting(false);
        await loadQuestions();
        return;
      } catch (err) {
        setError(err instanceof Error ? err.message : '导入失败');
        setImporting(false);
        return;
      }
    }

    // Batch import
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

  // ---- Create session ----
  async function handleStartPractice() {
    if (selectedIds.size === 0) return;
    setError('');
    try {
      const detail = await requestJson<PracticeSessionDetail>('/practice-sessions', {
        method: 'POST',
        body: JSON.stringify({ questionIds: Array.from(selectedIds) }),
      });
      setSession(detail);
      setCurrentIndex(0);
      setPhase('quiz');
      navigate(`/practice/${detail.id}`, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建做题会话失败');
    }
  }

  async function handleStartRecommendation(group: PracticeRecommendationGroup) {
    if (group.questionIds.length === 0) return;
    setStartingRecommendationKey(group.key);
    setError('');
    try {
      const detail = await requestJson<PracticeSessionDetail>('/practice-sessions', {
        method: 'POST',
        body: JSON.stringify({ name: group.title, questionIds: group.questionIds }),
      });
      setSession(detail);
      setCurrentIndex(0);
      setPhase('quiz');
      navigate(`/practice/${detail.id}`, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建推荐练习失败');
    } finally {
      setStartingRecommendationKey(null);
    }
  }

  // ---- Quiz actions ----
  async function submitAnswer(answer: string, autoAdvance = false) {
    if (!session) return;
    const item = session.questions[currentIndex];
    if (!item) return;
    try {
      await requestJson<{ status: string }>(`/practice-sessions/${session.id}/answer`, {
        method: 'POST',
        body: JSON.stringify({ orderIndex: item.orderIndex, userAnswer: answer }),
      });
      setSession((prev) => {
        if (!prev) return prev;
        const questions = [...prev.questions];
        questions[currentIndex] = {
          ...questions[currentIndex],
          userAnswer: answer,
          status: 'answered',
        };
        return { ...prev, questions };
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : '提交答案失败');
      return;
    }
    if (autoAdvance && currentIndex < session.questions.length - 1) {
      setCurrentIndex((i) => i + 1);
    }
  }

  async function handleAnswer(optionKey: string) {
    await submitAnswer(optionKey, true);
  }

  async function handleSaveDraftAnswer() {
    if (!session) return;
    const item = session.questions[currentIndex];
    if (!item) return;
    const answer = draftAnswers[item.orderIndex] ?? item.userAnswer ?? '';
    await submitAnswer(answer, false);
  }

  async function handleSkip() {
    if (!session) return;
    const item = session.questions[currentIndex];
    if (!item) return;
    try {
      await requestJson<{ status: string }>(`/practice-sessions/${session.id}/skip`, {
        method: 'POST',
        body: JSON.stringify({ orderIndex: item.orderIndex }),
      });
      setSession((prev) => {
        if (!prev) return prev;
        const questions = [...prev.questions];
        questions[currentIndex] = {
          ...questions[currentIndex],
          userAnswer: null,
          status: 'skipped',
        };
        return { ...prev, questions };
      });
      // Auto-advance to next question
      if (currentIndex < session.questions.length - 1) {
        setCurrentIndex((i) => i + 1);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '跳过失败');
    }
  }

  function goToQuestion(index: number) {
    if (!session) return;
    if (index >= 0 && index < session.questions.length) {
      setCurrentIndex(index);
    }
  }

  async function handleSubmit() {
    if (!session) return;
    setError('');
    try {
      const result = await requestJson<PracticeSessionResult>(`/practice-sessions/${session.id}/submit`, {
        method: 'POST',
      });
      setResults(result);
      setPhase('results');
    } catch (err) {
      setError(err instanceof Error ? err.message : '提交失败');
    }
  }

  function handleReset() {
    setPhase('setup');
    setSession(null);
    setResults(null);
    setCurrentIndex(0);
    setSelectedIds(new Set());
    navigate('/practice', { replace: true });
  }

  function renderAnswerInput(currentQ: PracticeSessionDetail['questions'][number]) {
    const q = currentQ.question;
    const questionType = q.questionType;

    if (questionType === 'true_false') {
      return (
        <div className="option-list">
          {[
            { key: 'true', content: '正确' },
            { key: 'false', content: '错误' },
          ].map((opt) => {
            const isSelected = currentQ.userAnswer === opt.key;
            return (
              <button
                key={opt.key}
                className={`option-card is-practice${isSelected ? ' is-selected' : ''}`}
                onClick={() => handleAnswer(opt.key)}
              >
                <span className="option-letter">{opt.key === 'true' ? '√' : '×'}</span>
                <span>{opt.content}</span>
              </button>
            );
          })}
        </div>
      );
    }

    if (questionType === 'multiple_choice') {
      const value = draftAnswers[currentQ.orderIndex] ?? currentQ.userAnswer ?? '';
      return (
        <>
          <div className="option-list">
            {q.options.map((opt) => {
              const selected = value.split(',').map((x) => x.trim()).includes(opt.key);
              return (
                <button
                  key={opt.key}
                  className={`option-card is-practice${selected ? ' is-selected' : ''}`}
                  onClick={() => setDraftAnswers((prev) => ({ ...prev, [currentQ.orderIndex]: toggleMultiAnswer(value, opt.key) }))}
                >
                  <span className="option-letter">{opt.key}</span>
                  <span>{opt.content}</span>
                </button>
              );
            })}
          </div>
          <button className="primary-button" type="button" onClick={handleSaveDraftAnswer}>
            保存答案
          </button>
        </>
      );
    }

    if (questionType === 'single_choice' || q.options.length > 0) {
      return (
        <div className="option-list">
          {q.options.map((opt) => {
            const isSelected = currentQ.userAnswer === opt.key;
            return (
              <button
                key={opt.key}
                className={`option-card is-practice${isSelected ? ' is-selected' : ''}`}
                onClick={() => handleAnswer(opt.key)}
              >
                <span className="option-letter">{opt.key}</span>
                <span>{opt.content}</span>
              </button>
            );
          })}
        </div>
      );
    }

    if (isTextQuestion(questionType)) {
      const value = draftAnswers[currentQ.orderIndex] ?? currentQ.userAnswer ?? '';
      const multiline = questionType !== 'fill_blank';
      return (
        <div className="practice-text-answer">
          {multiline ? (
            <textarea
              value={value}
              onChange={(e) => setDraftAnswers((prev) => ({ ...prev, [currentQ.orderIndex]: e.target.value }))}
              placeholder="输入你的答案"
              rows={6}
            />
          ) : (
            <input
              value={value}
              onChange={(e) => setDraftAnswers((prev) => ({ ...prev, [currentQ.orderIndex]: e.target.value }))}
              placeholder="输入填空答案"
            />
          )}
          <button className="primary-button" type="button" onClick={handleSaveDraftAnswer}>
            保存答案
          </button>
        </div>
      );
    }

    return <div className="empty-state">暂不支持该题型作答</div>;
  }

  // ---- Render: Setup Phase ----
  if (phase === 'setup') {
    return (
      <div className="app-shell">
        <TopBar onImport={handleImport} importing={importing} importProgress={batchProgress} />
        <div className="practice-setup-workspace">
          <SideNavigation />
          <div className="practice-setup-content">
            <div className="practice-setup-header">
              <h1 className="section-title-heading">做题模式</h1>
              <p className="practice-setup-desc">面向当前行动：从错题生成练习，作答后回到单题工作台查看 AI 解析并继续追问。</p>
            </div>

            <div className="recommendation-strip">
              {recommendations.map((group) => (
                <button
                  key={group.key}
                  className="recommendation-card"
                  type="button"
                  disabled={group.count === 0 || startingRecommendationKey === group.key}
                  onClick={() => handleStartRecommendation(group)}
                >
                  <span className="recommendation-title">{group.title}</span>
                  <strong>{group.count}</strong>
                  <span className="recommendation-reason">{group.count === 0 ? '暂无可推荐题目' : group.reason}</span>
                </button>
              ))}
              {recommendations.length === 0 ? <div className="empty-state">暂无推荐练习</div> : null}
            </div>

            <div className="practice-setup-toolbar">
              <div className="searchbox" style={{ flex: 1, maxWidth: 360 }}>
                <input
                  placeholder="搜索题目..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
              <div className="chip-row">
                <button
                  className={`chip${!activeCategoryId ? ' is-active' : ''}`}
                  type="button"
                  onClick={() => setActiveCategoryId(null)}
                >
                  全部
                </button>
                {categoryTree.map((cat) => (
                  <button
                    key={cat.id}
                    className={`chip${activeCategoryId === cat.id ? ' is-active' : ''}`}
                    type="button"
                    onClick={() => setActiveCategoryId(cat.id)}
                  >
                    {cat.name} ({cat.questionCount})
                  </button>
                ))}
              </div>
              <div style={{ display: 'flex', gap: 8, marginLeft: 'auto' }}>
                <button className="text-button" onClick={selectAll}>全选</button>
                <button className="text-button" onClick={deselectAll}>取消全选</button>
              </div>
            </div>

            <p className="practice-setup-hint">上方推荐优先覆盖今日复习、最近答错和薄弱知识点；手动选择适合临时按学科练习。</p>

            {loading ? (
              <div className="empty-state">正在加载题目...</div>
            ) : filteredQuestions.length === 0 ? (
              <div className="empty-state">暂无可练习题目，请先导入图片或手动创建题目</div>
            ) : (
              <div className="practice-question-grid">
                {filteredQuestions.map((q) => {
                  const isSelected = selectedIds.has(q.id);
                  return (
                    <button
                      key={q.id}
                      className={`practice-select-card${isSelected ? ' is-selected' : ''}`}
                      onClick={() => toggleQuestion(q.id)}
                    >
                      <div className="practice-select-check">
                        <span className={`check-box${isSelected ? ' is-checked' : ''}`}>
                          {isSelected && (
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                              <path d="m5 12 4 4 10-10" />
                            </svg>
                          )}
                        </span>
                      </div>
                      <div className="practice-select-body">
                        <div className="practice-select-stem">{q.stem}</div>
                        <div className="practice-select-meta">
                          <span className="tag tag--outline">{questionTypeLabel(q.questionType)}</span>
                          {q.tags?.slice(0, 3).map((tag) => <span className="tag tag--user" key={tag.id}>{tag.name}</span>)}
                          {q.analysisStatus === 'completed' ? (
                            <span className="tag-dot is-success" />
                          ) : q.analysisStatus === 'processing' ? (
                            <span className="tag-dot" />
                          ) : null}
                        </div>
                      </div>
                    </button>
                  );
                })}
              </div>
            )}

            <div className="practice-setup-footer">
              <span className="section-meta">已选 {selectedIds.size} 题</span>
              <button
                className="primary-button"
                disabled={selectedIds.size === 0}
                onClick={handleStartPractice}
              >
                开始做题 ({selectedIds.size} 题)
              </button>
            </div>
          </div>
        </div>
        {error ? <div className="global-toast is-error">{error}</div> : null}
      </div>
    );
  }

  // ---- Render: Quiz Phase ----
  if (phase === 'quiz' && session) {
    const currentQ = session.questions[currentIndex];
    const totalCount = session.questions.length;

    return (
      <div className="app-shell">
        <TopBar onImport={handleImport} importing={false} />
        <div className="practice-workspace">
          <SideNavigation />
          <div className="practice-question-area">
            {!currentQ ? (
              <div className="empty-state">题目不存在</div>
            ) : (
              <div className="practice-question-card">
                <div className="practice-quiz-progress">
                  <div
                    className="practice-quiz-progress-fill"
                    style={{ width: `${((currentIndex + 1) / totalCount) * 100}%` }}
                  />
                </div>
                <div className="practice-question-header">
                  <span className="practice-question-num">
                    第 {currentQ.orderIndex + 1} / {totalCount} 题
                  </span>
                  <span className="tag tag--outline">{questionTypeLabel(currentQ.question.questionType)}</span>
                  {currentQ.status === 'skipped' && (
                    <span className="tag" style={{ background: 'rgba(226,197,69,0.14)', color: 'var(--color-tertiary)' }}>已跳过</span>
                  )}
                  {currentQ.status === 'answered' && (
                    <span className="tag" style={{ background: 'rgba(119,217,112,0.14)', color: 'var(--color-success)' }}>已作答</span>
                  )}
                  {currentQ.status === 'unanswered' && (
                    <span className="tag tag--outline">未作答</span>
                  )}
                </div>

                <div className="practice-question-stem">
                  <MarkdownRenderer content={currentQ.question.stem} />
                </div>

                {renderAnswerInput(currentQ)}

                <div className="practice-bottom-bar">
                  <div className="practice-nav-buttons">
                    <button
                      className="outline-button"
                      disabled={currentIndex === 0}
                      onClick={() => goToQuestion(currentIndex - 1)}
                    >
                      上一题
                    </button>
                    <button
                      className="outline-button"
                      onClick={handleSkip}
                    >
                      跳过
                    </button>
                    <button
                      className="outline-button"
                      disabled={currentIndex >= totalCount - 1}
                      onClick={() => goToQuestion(currentIndex + 1)}
                    >
                      下一题
                    </button>
                  </div>
                  <button className="primary-button" onClick={handleSubmit}>
                    提交批改
                  </button>
                </div>
              </div>
            )}
          </div>

          <div className="question-navigator">
            <div className="navigator-header">题目导航</div>
            <div className="navigator-grid">
              {session.questions.map((q, idx) => {
                let statusClass = '';
                if (idx === currentIndex) statusClass = 'is-current';
                else if (q.status === 'answered') statusClass = 'is-answered';
                else if (q.status === 'skipped') statusClass = 'is-skipped';

                return (
                  <button
                    key={q.orderIndex}
                    className={`navigator-btn ${statusClass}`}
                    onClick={() => goToQuestion(idx)}
                  >
                    {q.orderIndex + 1}
                  </button>
                );
              })}
            </div>
            <div className="navigator-legend">
              <div className="legend-item">
                <span className="legend-dot is-answered" /> 已答
              </div>
              <div className="legend-item">
                <span className="legend-dot is-skipped" /> 跳过
              </div>
              <div className="legend-item">
                <span className="legend-dot" /> 未答
              </div>
            </div>
          </div>
        </div>
        {error ? <div className="global-toast is-error">{error}</div> : null}
      </div>
    );
  }

  // ---- Render: Results Phase ----
  if (phase === 'results' && results) {
    return (
      <div className="app-shell">
        <TopBar onImport={handleImport} importing={false} />
        <div className="practice-results-workspace">
          <SideNavigation />
          <div className="practice-results">
            <div className="results-hero">
              <div className="score-ring">
                <svg width="120" height="120" viewBox="0 0 120 120">
                  <circle cx="60" cy="60" r="52" fill="none" stroke="var(--color-border)" strokeWidth="6" />
                  <circle
                    cx="60" cy="60" r="52"
                    fill="none"
                    stroke="var(--color-primary)"
                    strokeWidth="6"
                    strokeLinecap="round"
                    strokeDasharray={`${(results.scorePercent / 100) * 327} 327`}
                    transform="rotate(-90 60 60)"
                  />
                </svg>
                <div className="score-ring-text">
                  <span className="score-ring-pct">{Math.round(results.scorePercent)}%</span>
                </div>
              </div>
              <h2 className="results-heading">
                {results.correctCount} / {results.totalCount} 正确
              </h2>
              <p className="results-sub">{results.name}</p>
            </div>

            <div className="results-list">
              {results.questions.map((q) => (
                <div
                  key={q.orderIndex}
                  className={`result-question-card${q.isCorrect ? ' is-correct' : ''}${q.status === 'skipped' ? ' is-skipped' : ''}`}
                >
                  <div className="result-question-header">
                    <span className="result-q-num">第 {q.orderIndex + 1} 题</span>
                    <span className="tag tag--outline">{questionTypeLabel(q.question.questionType)}</span>
                    {q.isCorrect ? (
                      <span className="tag" style={{ background: 'rgba(119,217,112,0.14)', color: 'var(--color-success)' }}>正确</span>
                    ) : q.status === 'skipped' ? (
                      <span className="tag" style={{ background: 'rgba(226,197,69,0.14)', color: 'var(--color-tertiary)' }}>已跳过</span>
                    ) : q.status === 'manual_required' ? (
                      <span className="tag" style={{ background: 'rgba(122,162,247,0.14)', color: 'var(--color-primary)' }}>待批改</span>
                    ) : (
                      <span className="tag" style={{ background: 'rgba(240,123,123,0.14)', color: '#f07b7b' }}>错误</span>
                    )}
                  </div>
                  <div className="result-stem">
                    <MarkdownRenderer content={q.question.stem} />
                  </div>
                  <div className="result-answers">
                    {q.userAnswer && (
                      <div className={`result-answer-row${!q.isCorrect ? ' is-wrong' : ''}`}>
                        <span className="result-answer-label">你的答案：</span>
                        <strong>{q.userAnswer}</strong>
                      </div>
                    )}
                    {(!q.isCorrect || q.status === 'skipped') && q.correctAnswer && (
                      <div className="result-answer-row is-correct-answer">
                        <span className="result-answer-label">正确答案：</span>
                        <strong>{q.correctAnswer}</strong>
                      </div>
                    )}
                    {q.status === 'skipped' && !q.userAnswer && (
                      <div className="result-answer-row is-skipped-answer">
                        <span className="result-answer-label">未作答</span>
                      </div>
                    )}
                    {q.gradingComment && (
                      <div className="result-answer-row">
                        <span className="result-answer-label">{q.gradingComment}</span>
                      </div>
                    )}
                  </div>
                  <div className="result-question-actions">
                    <Link className="text-button" to={`/?questionId=${q.question.id}`}>查看 AI 解析与追问</Link>
                  </div>
                </div>
              ))}
            </div>

            <div className="results-actions">
              <Link to="/" className="outline-button">返回工作台</Link>
              <button className="primary-button" onClick={handleReset}>再来一次</button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return null;
}
