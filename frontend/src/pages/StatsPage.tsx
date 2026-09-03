import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import { questionTypeLabel, requestJson } from '../utils';
import type { PracticeRecommendationGroup, QuestionItem } from '../types';

function isTodayOrPast(value?: string | null): boolean {
  if (!value) return false;
  const target = new Date(value);
  if (Number.isNaN(target.getTime())) return false;
  const today = new Date();
  target.setHours(0, 0, 0, 0);
  today.setHours(0, 0, 0, 0);
  return target.getTime() <= today.getTime();
}

export default function StatsPage() {
  const [questions, setQuestions] = useState<QuestionItem[]>([]);
  const [recommendations, setRecommendations] = useState<PracticeRecommendationGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        const [questionItems, recommendationItems] = await Promise.all([
          requestJson<QuestionItem[]>('/questions'),
          requestJson<PracticeRecommendationGroup[]>('/recommendations/practice').catch(() => []),
        ]);
        setQuestions(questionItems);
        setRecommendations(recommendationItems);
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载学习统计失败');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const stats = useMemo(() => {
    const masteryBuckets = Array.from({ length: 6 }, (_, level) => ({
      level,
      count: questions.filter((q) => q.learningState?.masteryLevel === level).length,
    }));
    const weakTags = new Map<string, number>();
    for (const question of questions) {
      for (const tag of question.learningState?.weaknessTags ?? []) {
        weakTags.set(tag, (weakTags.get(tag) ?? 0) + 1);
      }
    }
    const recentWrongIds = new Set(recommendations.find((item) => item.key === 'recent_wrong')?.questionIds ?? []);
    const todayReviewIds = new Set(recommendations.find((item) => item.key === 'today_review')?.questionIds ?? []);
    return {
      total: questions.length,
      needsReview: questions.filter((q) => q.qualityStatus === 'needs_review').length,
      analyzed: questions.filter((q) => q.analysisStatus === 'completed').length,
      practiced: questions.filter((q) => q.learningState).length,
      todayReview: todayReviewIds.size || questions.filter((q) => isTodayOrPast(q.learningState?.nextReviewAt)).length,
      recentWrong: recentWrongIds.size || questions.filter((q) => (q.learningState?.wrongCount ?? 0) > 0).length,
      masteryBuckets,
      weakTags: Array.from(weakTags.entries())
        .map(([name, count]) => ({ name, count }))
        .sort((a, b) => b.count - a.count)
        .slice(0, 8),
      lowMastery: questions
        .filter((q) => q.learningState && q.learningState.masteryLevel <= 2)
        .slice(0, 6),
    };
  }, [questions, recommendations]);

  const maxBucket = Math.max(...stats.masteryBuckets.map((item) => item.count), 1);

  return (
    <div className="app-shell">
      <TopBar onImport={() => undefined} importing={false} />
      <div className="insight-workspace">
        <SideNavigation />
        <main className="insight-page" aria-label="学习统计">
          <header className="insight-header">
            <div>
              <h1 className="section-title-heading">学习统计</h1>
              <p>围绕错题质量、解析进度和复习状态聚合当前题库。</p>
            </div>
            <Link className="outline-button" to="/">回到工作台</Link>
          </header>

          {loading ? <div className="empty-state">正在加载统计...</div> : null}
          {error ? <div className="global-toast is-error">{error}</div> : null}
          {!loading && questions.length === 0 ? (
            <div className="empty-state">暂无题目，导入单题后这里会生成学习统计。</div>
          ) : null}

          {!loading && questions.length > 0 ? (
            <>
              <section className="metric-grid">
                <article className="metric-tile"><span>总题数</span><strong>{stats.total}</strong></article>
                <article className="metric-tile"><span>待校准</span><strong>{stats.needsReview}</strong></article>
                <article className="metric-tile"><span>已解析</span><strong>{stats.analyzed}</strong></article>
                <article className="metric-tile"><span>已练习</span><strong>{stats.practiced}</strong></article>
                <article className="metric-tile"><span>今日待复习</span><strong>{stats.todayReview}</strong></article>
                <article className="metric-tile"><span>最近错题</span><strong>{stats.recentWrong}</strong></article>
              </section>

              <section className="insight-grid">
                <article className="insight-panel">
                  <h2>掌握度分布</h2>
                  <div className="mastery-chart">
                    {stats.masteryBuckets.map((bucket) => (
                      <div className="mastery-row" key={bucket.level}>
                        <span>{bucket.level}</span>
                        <div className="mastery-track">
                          <i style={{ width: `${(bucket.count / maxBucket) * 100}%` }} />
                        </div>
                        <strong>{bucket.count}</strong>
                      </div>
                    ))}
                  </div>
                </article>

                <article className="insight-panel">
                  <h2>薄弱标签 Top</h2>
                  <div className="weak-tag-list">
                    {stats.weakTags.map((tag) => (
                      <div className="weak-tag-row" key={tag.name}>
                        <span>{tag.name}</span>
                        <strong>{tag.count}</strong>
                      </div>
                    ))}
                    {stats.weakTags.length === 0 ? <div className="empty-state">暂无薄弱标签</div> : null}
                  </div>
                </article>
              </section>

              <section className="insight-panel">
                <h2>低掌握度题目</h2>
                <div className="archive-list is-compact">
                  {stats.lowMastery.map((question) => (
                    <Link className="archive-item" key={question.id} to={`/?questionId=${question.id}`}>
                      <div className="archive-item-main">
                        <div className="archive-item-title">{question.stem}</div>
                        <div className="archive-item-meta">
                          <span>{questionTypeLabel(question.questionType)}</span>
                          <span>掌握 {question.learningState?.masteryLevel ?? 0}/5</span>
                        </div>
                      </div>
                    </Link>
                  ))}
                  {stats.lowMastery.length === 0 ? <div className="empty-state">暂无低掌握度题目</div> : null}
                </div>
              </section>
            </>
          ) : null}
        </main>
      </div>
    </div>
  );
}
