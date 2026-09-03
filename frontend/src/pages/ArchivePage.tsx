import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import { cx, questionTypeLabel, requestJson } from '../utils';
import type { QuestionItem } from '../types';

type ArchiveFilter = 'wrong' | 'review' | 'weak' | 'low_mastery' | 'uncategorized';

const filters: Array<{ key: ArchiveFilter; label: string }> = [
  { key: 'wrong', label: '最近错题' },
  { key: 'review', label: '待校准' },
  { key: 'weak', label: '薄弱标签' },
  { key: 'low_mastery', label: '低掌握度' },
  { key: 'uncategorized', label: '未分类' },
];

function scoreQuestion(question: QuestionItem): number {
  let score = 0;
  if ((question.learningState?.wrongCount ?? 0) > 0) score += 8;
  if (question.qualityStatus === 'needs_review') score += 6;
  if (!question.categoryId) score += 3;
  if (question.learningState && question.learningState.masteryLevel <= 2) score += 5;
  return score;
}

export default function ArchivePage() {
  const [questions, setQuestions] = useState<QuestionItem[]>([]);
  const [activeFilter, setActiveFilter] = useState<ArchiveFilter>('wrong');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        setQuestions(await requestJson<QuestionItem[]>('/questions'));
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载错题归档失败');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const counts = useMemo(() => ({
    wrong: questions.filter((q) => (q.learningState?.wrongCount ?? 0) > 0).length,
    review: questions.filter((q) => q.qualityStatus === 'needs_review').length,
    weak: questions.filter((q) => (q.learningState?.weaknessTags.length ?? 0) > 0).length,
    low_mastery: questions.filter((q) => q.learningState && q.learningState.masteryLevel <= 2).length,
    uncategorized: questions.filter((q) => !q.categoryId).length,
  }), [questions]);

  const visibleQuestions = useMemo(() => {
    const items = questions.filter((question) => {
      if (activeFilter === 'wrong') return (question.learningState?.wrongCount ?? 0) > 0;
      if (activeFilter === 'review') return question.qualityStatus === 'needs_review';
      if (activeFilter === 'weak') return (question.learningState?.weaknessTags.length ?? 0) > 0;
      if (activeFilter === 'low_mastery') return question.learningState && question.learningState.masteryLevel <= 2;
      return !question.categoryId;
    });
    return items.sort((a, b) => scoreQuestion(b) - scoreQuestion(a) || b.id - a.id);
  }, [activeFilter, questions]);

  return (
    <div className="app-shell">
      <TopBar onImport={() => undefined} importing={false} />
      <div className="archive-workspace">
        <SideNavigation />
        <aside className="archive-rail">
          <header>
            <h1>错题归档</h1>
            <p>按错题、校准状态和掌握度整理复习入口。</p>
          </header>
          <div className="archive-filter-list">
            {filters.map((filter) => (
              <button
                key={filter.key}
                className={cx('archive-filter', activeFilter === filter.key && 'is-active')}
                type="button"
                onClick={() => setActiveFilter(filter.key)}
              >
                <span>{filter.label}</span>
                <strong>{counts[filter.key]}</strong>
              </button>
            ))}
          </div>
        </aside>

        <main className="archive-page" aria-label="错题归档视图">
          <header className="archive-header">
            <div>
              <h2>{filters.find((item) => item.key === activeFilter)?.label}</h2>
              <p>优先展示错题、待校准、未分类和低掌握度题目。</p>
            </div>
            <Link className="outline-button" to="/">回到工作台</Link>
          </header>

          {loading ? <div className="empty-state">正在加载归档...</div> : null}
          {error ? <div className="global-toast is-error">{error}</div> : null}
          {!loading && questions.length === 0 ? (
            <div className="empty-state">暂无题目，导入后可在这里沉淀错题。</div>
          ) : null}
          {!loading && questions.length > 0 && visibleQuestions.length === 0 ? (
            <div className="empty-state">当前筛选下暂无题目。</div>
          ) : null}

          <div className="archive-list">
            {visibleQuestions.map((question) => (
              <Link className="archive-item" key={question.id} to={`/?questionId=${question.id}`}>
                <div className="archive-item-main">
                  <div className="archive-item-title">{question.stem}</div>
                  <div className="archive-item-meta">
                    <span>{questionTypeLabel(question.questionType)}</span>
                    <span>{question.categoryName ?? '未分类'}</span>
                    <span>掌握 {question.learningState?.masteryLevel ?? 0}/5</span>
                    <span>错 {question.learningState?.wrongCount ?? 0} 次</span>
                  </div>
                  {(question.learningState?.weaknessTags.length ?? 0) > 0 ? (
                    <div className="archive-tag-row">
                      {question.learningState?.weaknessTags.slice(0, 4).map((tag) => (
                        <span className="tag tag--outline" key={tag}>{tag}</span>
                      ))}
                    </div>
                  ) : null}
                </div>
                {question.qualityStatus === 'needs_review' ? (
                  <div className="archive-item-flags">
                    <span className="tag tag--review">待校准</span>
                  </div>
                ) : null}
              </Link>
            ))}
          </div>
        </main>
      </div>
    </div>
  );
}
