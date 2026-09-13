import React, { useEffect, useMemo, useState } from 'react';
import { analysisStatusLabel, cx, ocrStatusLabel, questionTypeLabel, QUESTION_TYPE_OPTIONS } from '../utils';
import type { QuestionItem, AnalysisItem, DetailOption, AnalysisSection, CategoryTreeNode, TagItem, OptionItem } from '../types';
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  ChevronUpIcon,
  ChevronDownIcon,
  EditIcon,
  RefreshIcon,
  CheckIcon,
  ChainIcon,
  CompassIcon,
  AlertTriangleIcon,
  DocumentIcon,
} from './icons';
import FavoriteButton from './FavoriteButton';
import CategorySelector from './CategorySelector';
import TagEditor from './TagEditor';
import MarkdownRenderer from './MarkdownRenderer';

interface DetailPanelProps {
  question: QuestionItem | null;
  analysis: AnalysisItem | null;
  editing: boolean;
  setEditing: (value: boolean) => void;
  draftStem: string;
  setDraftStem: (value: string) => void;
  draftAnswer: string;
  setDraftAnswer: (value: string) => void;
  draftQuestionType: string;
  setDraftQuestionType: (value: string) => void;
  draftOptions: OptionItem[];
  setDraftOptions: (value: OptionItem[]) => void;
  onSaveQuestion: () => void;
  onRetryOCR: () => void;
  onRetryAnalysis: () => void;
  detailOpen: boolean;
  onToggleDetail: () => void;
  showAnswer: boolean;
  userSelectedOption: string | null;
  onSelectOption: (key: string) => void;
  onSubmitTextAnswer: (answer: string) => void;
  onToggleAnswer: () => void;
  submittingAnswer: boolean;
  categories: CategoryTreeNode[];
  allTags: TagItem[];
  onToggleFavorite: () => void;
  onCategoryChange: (categoryId: number | null) => void;
  onTagsChange: (tagIds: number[]) => void;
}

export default function DetailPanel({
  question,
  analysis,
  editing,
  setEditing,
  draftStem,
  setDraftStem,
  draftAnswer,
  setDraftAnswer,
  draftQuestionType,
  setDraftQuestionType,
  draftOptions,
  setDraftOptions,
  onSaveQuestion,
  onRetryOCR,
  onRetryAnalysis,
  detailOpen,
  onToggleDetail,
  showAnswer,
  userSelectedOption,
  onSelectOption,
  onSubmitTextAnswer,
  onToggleAnswer,
  submittingAnswer,
  categories,
  allTags,
  onToggleFavorite,
  onCategoryChange,
  onTagsChange,
}: DetailPanelProps) {
  const [practiceTextAnswer, setPracticeTextAnswer] = useState('');

  useEffect(() => {
    setPracticeTextAnswer(question?.userAnswer ?? '');
  }, [question?.id, question?.userAnswer]);
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    steps: true,
    options: false,
    pitfalls: false,
  });

  function toggleSection(key: string) {
    setExpandedSections((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  const questionType = question?.questionType ?? '';
  const textQuestion = ['fill_blank', 'subjective', 'short_answer', 'essay', 'calculation'].includes(questionType);
  const draftTextQuestion = ['fill_blank', 'subjective', 'short_answer', 'essay', 'calculation'].includes(draftQuestionType);
  const multiChoice = questionType === 'multiple_choice';
  const displayOptions = useMemo(() => (
    questionType === 'true_false' && !(question?.options?.length)
      ? [{ key: 'true', content: '正确' }, { key: 'false', content: '错误' }]
      : (question?.options ?? [])
  ), [question?.options, questionType]);
  const optionCount = question?.options?.length ?? 0;
  const selectedAnswers = useMemo(() => new Set((userSelectedOption ?? '').split(',').map((value) => value.trim()).filter(Boolean)), [userSelectedOption]);
  const correctAnswers = useMemo(() => new Set((question?.correctAnswer ?? '').split(/[,，、;；\s]+/).map((value) => value.trim()).filter(Boolean)), [question?.correctAnswer]);

  const detailOptions: DetailOption[] = useMemo(
    () =>
      displayOptions.map((option) => ({
        key: option.key,
        content: option.content,
        isCorrect: showAnswer && correctAnswers.has(option.key),
        isWrong: showAnswer && selectedAnswers.has(option.key) && !correctAnswers.has(option.key),
      })),
    [correctAnswers, displayOptions, selectedAnswers, showAnswer],
  );

  const analysisSections: AnalysisSection[] = useMemo(() => {
    const content = analysis?.content;
    return [
      {
        key: 'steps',
        title: '步骤拆解',
        icon: <ChainIcon />,
        expanded: expandedSections['steps'] ?? true,
        body: (
          <>
            {(content?.steps ?? []).map((item, index) => (
              <p key={`${index}-${item}`}>{item}</p>
            ))}
            {!content?.steps?.length ? <p>暂无步骤</p> : null}
          </>
        ),
      },
      {
        key: 'options',
        title: optionCount ? '选项分析' : '答案核对',
        icon: <CompassIcon />,
        expanded: expandedSections['options'] ?? false,
        body: (
          <>
            {Object.entries(content?.optionAnalysis ?? {}).map(([key, value]) => (
              <p key={key}>
                <span className="analysis-highlight">{key}：</span>
                {value}
              </p>
            ))}
            {!Object.keys(content?.optionAnalysis ?? {}).length ? <p>暂无选项分析</p> : null}
          </>
        ),
      },
      {
        key: 'answer-details',
        title: '答案与评分要点',
        icon: <DocumentIcon />,
        expanded: expandedSections['answer-details'] ?? true,
        body: (
          <>
            {content?.answerFormat ? <p><span className="analysis-highlight">答案格式：</span>{content.answerFormat}</p> : null}
            {(content?.blankAnswers ?? []).map((item, index) => <p key={`blank-${index}`}><span className="analysis-highlight">第 {index + 1} 空：</span>{item}</p>)}
            {(content?.scoringPoints ?? []).map((item) => <p key={`score-${item}`}><span className="analysis-highlight">得分点：</span>{item}</p>)}
            {(content?.rubric ?? []).map((item) => <p key={`rubric-${item}`}><span className="analysis-highlight">评分依据：</span>{item}</p>)}
            {!content?.blankAnswers?.length && !content?.scoringPoints?.length && !content?.rubric?.length ? <p>暂无额外评分要点</p> : null}
          </>
        ),
      },
      {
        key: 'pitfalls',
        title: '易错点',
        icon: <AlertTriangleIcon />,
        expanded: expandedSections['pitfalls'] ?? false,
        body: (
          <>
            {(content?.pitfalls ?? []).map((item) => (
              <p key={item}>{item}</p>
            ))}
            {!content?.pitfalls?.length ? <p>暂无易错点</p> : null}
          </>
        ),
      },
    ];
  }, [analysis, expandedSections, optionCount]);

  const structureNotices = useMemo(() => {
    const warnings = question?.structureWarnings ?? [];
    const notices: string[] = [];
    if (warnings.includes('options_incomplete') || warnings.some((item) => item.startsWith('missing_options_'))) {
      notices.push('选项可能缺失，建议人工校准');
    }
    if (warnings.includes('llm_refine_failed') || warnings.includes('llm_refine_unavailable')) {
      notices.push('LLM 结构校验失败，当前为规则解析结果');
    }
    if (warnings.includes('ocr_low_confidence') || (question?.qualityStatus === 'needs_review' && warnings.length === 0)) {
      notices.push('低置信度或待校准状态，建议人工复核');
    }
    return Array.from(new Set(notices));
  }, [question?.qualityStatus, question?.structureWarnings]);

  function resetQuestionDraft() {
    setDraftStem(question?.stem ?? '');
    setDraftAnswer(question?.correctAnswer ?? '');
    setDraftQuestionType(question?.questionType ?? 'subjective');
    setDraftOptions([...(question?.options ?? [])].sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0)));
    setPracticeTextAnswer(question?.userAnswer ?? '');
  }

  function updateDraftOption(index: number, patch: Partial<OptionItem>) {
    setDraftOptions(draftOptions.map((item, idx) => (idx === index ? { ...item, ...patch } : item)));
  }

  function addDraftOption() {
    const existingKeys = new Set(draftOptions.map((item) => item.key.trim().toUpperCase()));
    const nextKey = ['A', 'B', 'C', 'D', 'E', 'F'].find((key) => !existingKeys.has(key)) ?? String.fromCharCode(65 + draftOptions.length);
    setDraftOptions([...draftOptions, { key: nextKey, content: '', sortOrder: draftOptions.length + 1 }]);
  }

  function removeDraftOption(index: number) {
    setDraftOptions(draftOptions.filter((_, idx) => idx !== index).map((item, idx) => ({ ...item, sortOrder: idx + 1 })));
  }

  return (
    <aside className={cx('panel panel--detail', !detailOpen && 'collapsed')} aria-label="题目详情">
      <div className="panel-reveal">
        <button
          className="panel-toggle"
          type="button"
          aria-label="展开题目详情"
          onClick={onToggleDetail}
        >
          <ChevronLeftIcon size={16} />
        </button>
      </div>
      <div className="detail-layout">
        <header className="section-header">
          <div className="section-title">
            <h2>题目详情</h2>
            <button
              className="panel-toggle"
              type="button"
              aria-label="收起题目详情"
              onClick={onToggleDetail}
            >
              <ChevronRightIcon size={16} />
            </button>
          </div>
          <div className="detail-actions">
            {question && (
              <FavoriteButton isFavorited={question.isFavorited} onToggle={onToggleFavorite} />
            )}
            <button
              className={cx('outline-button', !showAnswer && 'is-highlight')}
              type="button"
              onClick={onToggleAnswer}
            >
              {showAnswer ? <EditIcon size={12} /> : <RefreshIcon size={12} />}
              {showAnswer ? '重新练习' : '查看答案'}
            </button>
            <button
              className="outline-button"
              type="button"
              onClick={() => {
                setEditing(!editing);
                resetQuestionDraft();
              }}
            >
              <EditIcon size={12} />
              {editing ? '取消' : '编辑'}
            </button>
            <button className="filled-button" type="button" onClick={onSaveQuestion} disabled={!question}>
              <CheckIcon size={12} />
              保存
            </button>
          </div>
        </header>

        <div className="detail-scroll">
          <section className="question-sheet">
            <div className="sheet-meta-row">
              <div className="sheet-meta-primary">
                <span className="meta-caption">题型</span>
                <span className="tag tag--type">{questionTypeLabel(question?.questionType)}</span>
                {question?.qualityStatus === 'needs_review' ? <span className="tag tag--review">待校准</span> : null}
              </div>
              <div className="sheet-status-group" aria-label="题目处理状态">
                <span className="status-pill">{question?.ocrStatus ? ocrStatusLabel(question.ocrStatus) : '待识别'}</span>
                <span className={cx('status-pill', question?.analysisStatus === 'completed' && 'is-success')}>
                  {analysisStatusLabel(question?.analysisStatus)}
                </span>
                {question?.ocrStatus === 'failed' ? (
                  <button className="text-button" type="button" onClick={onRetryOCR}>重试识别</button>
                ) : null}
                {question?.analysisStatus === 'failed' ? (
                  <button className="text-button" type="button" onClick={onRetryAnalysis}>重试解析</button>
                ) : null}
              </div>
            </div>

            <div className="detail-taxonomy">
              <div className="taxonomy-field taxonomy-field--category">
                <CategorySelector
                  categories={categories}
                  currentCategoryId={question?.categoryId}
                  onChange={onCategoryChange}
                />
              </div>
              <div className="taxonomy-field taxonomy-field--tags">
                <TagEditor
                  questionTags={question?.tags ?? []}
                  allTags={allTags}
                  onTagsChange={onTagsChange}
                />
              </div>
            </div>

            {structureNotices.length > 0 ? (
              <div className="structure-warning-panel" aria-label="OCR 结构化提示">
                <AlertTriangleIcon size={14} />
                <div>
                  {structureNotices.map((notice) => (
                    <p key={notice}>{notice}</p>
                  ))}
                  <span>
                    {question?.parseSource ? `来源：${question.parseSource}` : '来源：规则解析'}
                    {question?.structureConfidence != null ? `，置信度：${Math.round(question.structureConfidence * 100)}%` : ''}
                  </span>
                </div>
              </div>
            ) : null}

            <div className="detail-copy">
              {editing ? (
                <div className="edit-form">
                  <label className="form-field">
                    <span className="form-label">题干</span>
                    <textarea className="form-input form-textarea" value={draftStem} onChange={(e) => setDraftStem(e.target.value)} />
                  </label>
                  <label className="form-field">
                    <span className="form-label">题型</span>
                    <select className="form-input" value={draftQuestionType} onChange={(e) => setDraftQuestionType(e.target.value)}>
                      {QUESTION_TYPE_OPTIONS.map((type) => (
                        <option key={type} value={type}>{questionTypeLabel(type)}</option>
                      ))}
                    </select>
                  </label>
                  <label className="form-field">
                    <span className="form-label">正确答案</span>
                    {draftTextQuestion ? (
                      <textarea className="form-input form-textarea" rows={8} value={draftAnswer} onChange={(e) => setDraftAnswer(e.target.value)} />
                    ) : (
                      <input className="form-input" value={draftAnswer} onChange={(e) => setDraftAnswer(e.target.value)} />
                    )}
                  </label>
                  <div className="form-field">
                    <div className="option-edit-header">
                      <span className="form-label">选项</span>
                      <button className="text-button" type="button" onClick={addDraftOption}>新增选项</button>
                    </div>
                    <div className="option-edit-list">
                      {draftOptions.map((option, index) => (
                        <div className="option-edit-row" key={`${index}-${option.sortOrder ?? index}`}>
                          <input
                            className="form-input option-key-input"
                            value={option.key}
                            onChange={(e) => updateDraftOption(index, { key: e.target.value })}
                            aria-label={`选项 ${index + 1} 标识`}
                          />
                          <input
                            className="form-input"
                            value={option.content}
                            onChange={(e) => updateDraftOption(index, { content: e.target.value })}
                            aria-label={`选项 ${index + 1} 内容`}
                          />
                          <button className="text-button" type="button" onClick={() => removeDraftOption(index)}>删除</button>
                        </div>
                      ))}
                      {draftOptions.length === 0 ? <div className="empty-state">暂无选项，可手动新增 A/B/C/D</div> : null}
                    </div>
                  </div>
                </div>
              ) : (
                <p>{question?.stem ?? '未选择题目'}</p>
              )}
            </div>

            {!editing && !showAnswer && textQuestion ? (
              <div className="practice-text-answer">
                {questionType === 'fill_blank' ? (
                  <input value={practiceTextAnswer} onChange={(event) => setPracticeTextAnswer(event.target.value)} placeholder="按空位顺序填写答案" />
                ) : (
                  <textarea value={practiceTextAnswer} onChange={(event) => setPracticeTextAnswer(event.target.value)} placeholder="输入你的答案" rows={6} />
                )}
                <button className="primary-button" type="button" disabled={submittingAnswer || !practiceTextAnswer.trim()} onClick={() => onSubmitTextAnswer(practiceTextAnswer)}>
                  {submittingAnswer ? '提交中...' : '提交答案'}
                </button>
              </div>
            ) : !editing ? <div className="option-list">
              {detailOptions.map((option) => (
                <div
                  key={option.key}
                  className={cx(
                    'option-card',
                    !showAnswer && 'is-practice',
                    option.isCorrect && 'is-correct',
                    option.isWrong && 'is-wrong',
                    !showAnswer && userSelectedOption === option.key && 'is-selected',
                  )}
                  onClick={() => {
                    if (!showAnswer && !submittingAnswer) onSelectOption(option.key);
                  }}
                  role={!showAnswer ? 'button' : undefined}
                  tabIndex={!showAnswer ? 0 : undefined}
                  onKeyDown={(e) => {
                    if (!showAnswer && (e.key === 'Enter' || e.key === ' ')) {
                      e.preventDefault();
                      if (!submittingAnswer) onSelectOption(option.key);
                    }
                  }}
                >
                  <span className="option-letter">{option.key}.</span>
                  <span>{option.content}</span>
                  {option.isCorrect ? <CheckIcon size={14} /> : null}
                  {option.isWrong ? <span style={{ color: '#f07b7b', fontSize: 12, marginLeft: 4 }}>你的选择</span> : null}
                </div>
              ))}
              {!detailOptions.length ? <div className="empty-state">暂无选项，当前题型使用文本作答</div> : null}
              {multiChoice && detailOptions.length ? <div className="empty-state">可选择多个选项，点击后会保存当前组合</div> : null}
              {submittingAnswer ? <div className="empty-state">提交中...</div> : null}
            </div> : null}

            {showAnswer && (
              <div className={cx('answer-row', textQuestion && 'answer-row--long')}>
                <span>正确答案</span>
                {textQuestion ? (
                  <div className="answer-row-content">
                    <MarkdownRenderer content={question?.correctAnswer ?? '未设置'} />
                  </div>
                ) : <strong>{question?.correctAnswer ?? '未设置'}</strong>}
              </div>
            )}
          </section>

          {showAnswer && (
          <>
          <section className="analysis-panel">
            <h3 className="analysis-panel-title">解析</h3>
            {!analysis && ['queued', 'processing', 'pending'].includes(question?.analysisStatus ?? '') ? (
              <div className="analysis-placeholder" role="status">解析中，AI 正在整理题干和解题步骤…</div>
            ) : null}
            {analysis ? analysisSections.map((section) => (
              <article key={section.key} className="accordion">
                <header
                  className="accordion-header"
                  role="button"
                  tabIndex={0}
                  onClick={() => toggleSection(section.key)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      toggleSection(section.key);
                    }
                  }}
                >
                  <div className="accordion-title">
                    {section.icon}
                    <span>{section.title}</span>
                  </div>
                  {section.expanded ? <ChevronUpIcon size={12} /> : <ChevronDownIcon size={12} />}
                </header>
                {section.expanded ? <div className="accordion-panel">{section.body}</div> : null}
              </article>
            )) : null}
            {!analysis && !['queued', 'processing', 'pending'].includes(question?.analysisStatus ?? '') ? <div className="analysis-placeholder">暂无解析</div> : null}
          </section>
          </>
          )}

          <section className="ocr-panel">
            <span className="ocr-label">OCR 原文</span>
            <div className="ocr-dropzone">
              <div className="ocr-dropzone-inner">
                <DocumentIcon size={20} />
                <span>{question?.rawOcrText ?? '暂无 OCR 内容'}</span>
              </div>
            </div>
          </section>
        </div>
      </div>
    </aside>
  );
}
