import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import { requestJson } from '../utils';
import type { PracticeRecommendationGroup, PracticeSessionListItem, QuestionItem } from '../types';

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
  const [sessions, setSessions] = useState<PracticeSessionListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError('');
      try {
        const [questionItems, recommendationItems, sessionItems] = await Promise.all([
          requestJson<QuestionItem[]>('/questions'),
          requestJson<PracticeRecommendationGroup[]>('/recommendations/practice').catch(() => []),
          requestJson<PracticeSessionListItem[]>('/practice-sessions').catch(() => []),
        ]);
        setQuestions(questionItems);
        setRecommendations(recommendationItems);
        setSessions(sessionItems);
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
    const weakCategories = new Map<string, number>();
    for (const question of questions) {
      const needsPractice = (question.learningState?.wrongCount ?? 0) > 0 || (question.learningState?.masteryLevel ?? 5) <= 2;
      if (!needsPractice) continue;
      if (question.categoryName) {
        weakCategories.set(question.categoryName, (weakCategories.get(question.categoryName) ?? 0) + 1);
      }
      for (const tag of question.learningState?.weaknessTags ?? []) {
        weakTags.set(tag, (weakTags.get(tag) ?? 0) + 1);
      }
    }
    const recentWrongIds = new Set(recommendations.find((item) => item.key === 'recent_wrong')?.questionIds ?? []);
    const todayReviewIds = new Set(recommendations.find((item) => item.key === 'today_review')?.questionIds ?? []);
    const submitted = sessions.filter((session) => session.status === 'submitted');
    const totalAttempts = submitted.reduce((sum, session) => sum + session.totalCount, 0);
    const totalCorrect = submitted.reduce((sum, session) => sum + session.correctCount, 0);
    const sortedSessions = [...submitted].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
    const recent = sortedSessions.slice(0, 3);
    const previous = sortedSessions.slice(3, 6);
    const rate = (items: PracticeSessionListItem[]) => {
      const attempts = items.reduce((sum, session) => sum + session.totalCount, 0);
      return attempts ? items.reduce((sum, session) => sum + session.correctCount, 0) / attempts : null;
    };
    const recentRate = rate(recent);
    const previousRate = rate(previous);

    return {
      practiced: questions.filter((q) => q.learningState?.lastPracticedAt).length,
      analyzed: questions.filter((q) => q.analysisStatus === 'completed').length,
      todayReview: todayReviewIds.size || questions.filter((q) => isTodayOrPast(q.learningState?.nextReviewAt)).length,
      recentWrong: recentWrongIds.size || questions.filter((q) => (q.learningState?.wrongCount ?? 0) > 0).length,
      correctRate: totalAttempts ? totalCorrect / totalAttempts : null,
      recentRate,
      previousRate,
      masteryBuckets,
      weakTags: Array.from(weakTags.entries()).map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count).slice(0, 8),
      weakCategories: Array.from(weakCategories.entries()).map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count).slice(0, 6),
    };
  }, [questions, recommendations, sessions]);

  const maxBucket = Math.max(...stats.masteryBuckets.map((item) => item.count), 1);
  const trendLabel = stats.recentRate == null || stats.previousRate == null
    ? '完成更多练习后显示趋势'
    : `${stats.recentRate >= stats.previousRate ? '较前一阶段提升' : '较前一阶段回落'} ${Math.round(Math.abs(stats.recentRate - stats.previousRate) * 100)}%`;
  const insight = stats.weakTags[0]
    ? `基于已保存的 AI 错因归纳，当前最需要回看的知识点是「${stats.weakTags[0].name}」。`
    : stats.todayReview > 0
      ? `有 ${stats.todayReview} 道题达到复习时间，建议先完成今日复习。`
      : '完成一次练习或使用“归纳错因”后，这里会形成更具体的学习洞察。';

  return (
    <div className="app-shell">
      <TopBar onImport={() => undefined} importing={false} />
      <div className="insight-workspace">
        <SideNavigation />
        <main className="insight-page" aria-label="学习统计">
          <header className="insight-header">
            <div>
              <h1 className="section-title-heading">学习统计</h1>
              <p>看整体趋势、薄弱学科和下一步复习方向，不承载题目归档或做题流程。</p>
            </div>
            <Link className="outline-button" to="/practice">去做题</Link>
          </header>

          {loading ? <div className="empty-state">正在加载学习反馈...</div> : null}
          {error ? <div className="global-toast is-error">{error}</div> : null}
          {!loading && questions.length === 0 ? (
            <div className="empty-state">暂无题目，导入单题并完成一次作答后这里会生成趋势反馈。</div>
          ) : null}

          {!loading && questions.length > 0 ? (
            <>
              <section className="metric-grid">
                <article className="metric-tile"><span>练习正确率</span><strong>{stats.correctRate == null ? '—' : `${Math.round(stats.correctRate * 100)}%`}</strong></article>
                <article className="metric-tile"><span>已练习题目</span><strong>{stats.practiced}</strong></article>
                <article className="metric-tile"><span>今日待复习</span><strong>{stats.todayReview}</strong></article>
                <article className="metric-tile"><span>最近答错</span><strong>{stats.recentWrong}</strong></article>
                <article className="metric-tile"><span>已生成解析</span><strong>{stats.analyzed}</strong></article>
                <article className="metric-tile"><span>练习趋势</span><strong className="metric-tile-small">{trendLabel}</strong></article>
              </section>

              <section className="insight-grid">
                <article className="insight-panel">
                  <h2>掌握度分布</h2>
                  <div className="mastery-chart">
                    {stats.masteryBuckets.map((bucket) => (
                      <div className="mastery-row" key={bucket.level}>
                        <span>{bucket.level}</span>
                        <div className="mastery-track"><i style={{ width: `${(bucket.count / maxBucket) * 100}%` }} /></div>
                        <strong>{bucket.count}</strong>
                      </div>
                    ))}
                  </div>
                </article>

                <article className="insight-panel">
                  <h2>薄弱学科</h2>
                  <div className="weak-tag-list">
                    {stats.weakCategories.map((category) => (
                      <div className="weak-tag-row" key={category.name}><span>{category.name}</span><strong>{category.count}</strong></div>
                    ))}
                    {stats.weakCategories.length === 0 ? <div className="empty-state">暂无可归纳的薄弱学科</div> : null}
                  </div>
                </article>
              </section>

              <section className="insight-panel ai-insight-panel">
                <div>
                  <h2>AI 学习洞察</h2>
                  <p>{insight}</p>
                  {stats.weakTags.length > 0 ? (
                    <div className="archive-tag-row">
                      {stats.weakTags.slice(0, 5).map((tag) => <span className="tag tag--outline" key={tag.name}>{tag.name}</span>)}
                    </div>
                  ) : null}
                </div>
                <div className="insight-actions">
                  <Link className="primary-button" to="/practice">生成练习</Link>
                  <Link className="outline-button" to="/archive">查看归档</Link>
                </div>
              </section>
            </>
          ) : null}
        </main>
      </div>
    </div>
  );
}
