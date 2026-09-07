import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import { cx, questionTypeLabel, requestJson } from '../utils';
import type { CategoryTreeNode, QuestionItem, TagItem } from '../types';

type ArchiveFilter = 'all' | 'wrong' | 'review' | 'uncategorized' | 'favorite';

const filters: Array<{ key: ArchiveFilter; label: string }> = [
  { key: 'all', label: '全部题目' },
  { key: 'wrong', label: '最近答错' },
  { key: 'review', label: '待校准' },
  { key: 'uncategorized', label: '未分类' },
  { key: 'favorite', label: '已收藏' },
];

function scoreQuestion(question: QuestionItem): number {
  let score = 0;
  if ((question.learningState?.wrongCount ?? 0) > 0) score += 8;
  if (question.qualityStatus === 'needs_review') score += 6;
  if (!question.categoryId) score += 3;
  if (question.isFavorited) score += 2;
  return score;
}

export default function ArchivePage() {
  const [questions, setQuestions] = useState<QuestionItem[]>([]);
  const [categories, setCategories] = useState<CategoryTreeNode[]>([]);
  const [tags, setTags] = useState<TagItem[]>([]);
  const [activeFilter, setActiveFilter] = useState<ArchiveFilter>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [categoryId, setCategoryId] = useState<number | null>(null);
  const [tagId, setTagId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        const [questionItems, categoryResponse, tagItems] = await Promise.all([
          requestJson<QuestionItem[]>('/questions'),
          requestJson<{ categories: CategoryTreeNode[] }>('/categories/tree'),
          requestJson<TagItem[]>('/tags'),
        ]);
        setQuestions(questionItems);
        setCategories(categoryResponse.categories);
        setTags(tagItems);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载错题归档失败');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const counts = useMemo(() => ({
    all: questions.length,
    wrong: questions.filter((q) => (q.learningState?.wrongCount ?? 0) > 0).length,
    review: questions.filter((q) => q.qualityStatus === 'needs_review').length,
    uncategorized: questions.filter((q) => !q.categoryId).length,
    favorite: questions.filter((q) => q.isFavorited).length,
  }), [questions]);

  const visibleQuestions = useMemo(() => {
    const keyword = searchQuery.trim().toLowerCase();
    const items = questions.filter((question) => {
      if (activeFilter === 'wrong' && (question.learningState?.wrongCount ?? 0) === 0) return false;
      if (activeFilter === 'review' && question.qualityStatus !== 'needs_review') return false;
      if (activeFilter === 'uncategorized' && question.categoryId) return false;
      if (activeFilter === 'favorite' && !question.isFavorited) return false;
      if (keyword && !question.stem.toLowerCase().includes(keyword)) return false;
      if (categoryId != null && question.categoryId !== categoryId) return false;
      if (tagId != null && !(question.tags ?? []).some((tag) => tag.id === tagId)) return false;
      return true;
    });
    return items.sort((a, b) => scoreQuestion(b) - scoreQuestion(a) || b.id - a.id);
  }, [activeFilter, categoryId, questions, searchQuery, tagId]);

  return (
    <div className="app-shell">
      <TopBar onImport={() => undefined} importing={false} />
      <div className="archive-workspace">
        <SideNavigation />
        <aside className="archive-rail">
          <header>
            <h1>错题归档</h1>
            <p>管理题目资产，按学科、知识点、收藏和校准状态再次进入工作台。</p>
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
              <p>这里保存可检索、可复习、可再次追问的题目，不重复承载学习统计报表。</p>
            </div>
            <Link className="outline-button" to="/">回到工作台</Link>
          </header>

          <div className="archive-toolbar" aria-label="归档筛选">
            <label className="searchbox archive-searchbox">
              <span className="sr-only">搜索题目</span>
              <input value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder="搜索题干" />
            </label>
            <select className="filter-select" value={categoryId ?? ''} onChange={(event) => setCategoryId(event.target.value ? Number(event.target.value) : null)}>
              <option value="">全部学科分类</option>
              {categories.map((category) => <option key={category.id} value={category.id}>{category.name}</option>)}
            </select>
            <select className="filter-select" value={tagId ?? ''} onChange={(event) => setTagId(event.target.value ? Number(event.target.value) : null)}>
              <option value="">全部知识点标签</option>
              {tags.map((tag) => <option key={tag.id} value={tag.id}>{tag.name}</option>)}
            </select>
          </div>

          {loading ? <div className="empty-state">正在加载归档...</div> : null}
          {error ? <div className="global-toast is-error">{error}</div> : null}
          {!loading && questions.length === 0 ? <div className="empty-state">暂无题目，导入后可在这里沉淀错题。</div> : null}
          {!loading && questions.length > 0 && visibleQuestions.length === 0 ? <div className="empty-state">当前筛选下暂无题目。</div> : null}

          <div className="archive-list">
            {visibleQuestions.map((question) => (
              <Link className="archive-item" key={question.id} to={`/?questionId=${question.id}`}>
                <div className="archive-item-main">
                  <div className="archive-item-title">{question.stem}</div>
                  <div className="archive-item-meta">
                    <span>{questionTypeLabel(question.questionType)}</span>
                    <span>{question.categoryName ?? '未分类'}</span>
                    {question.learningState?.lastPracticedAt ? <span>有复习记录</span> : <span>未复习</span>}
                    {question.isFavorited ? <span>已收藏</span> : null}
                    {question.learningState && (question.learningState.wrongCount ?? 0) > 0 ? <span>错 {question.learningState.wrongCount} 次</span> : null}
                  </div>
                  {(question.tags ?? []).length > 0 ? (
                    <div className="archive-tag-row">
                      {(question.tags ?? []).slice(0, 6).map((tag) => <span className="tag tag--outline" key={tag.id}>{tag.name}</span>)}
                    </div>
                  ) : null}
                </div>
                {question.qualityStatus === 'needs_review' ? <div className="archive-item-flags"><span className="tag tag--review">待校准</span></div> : null}
              </Link>
            ))}
          </div>
        </main>
      </div>
    </div>
  );
}
