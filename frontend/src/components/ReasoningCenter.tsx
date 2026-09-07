import React, { useRef } from 'react';
import { formatTime } from '../utils';
import type { QuestionItem, AnalysisItem, ChatMessage, PromptAction } from '../types';
import MarkdownRenderer from './MarkdownRenderer';
import aiAvatar from '../assets/line-husky-nobackground.png';
import {
  HierarchyIcon,
  SparkleIcon,
  ThumbsUpIcon,
  NotebookIcon,
  PaperclipIcon,
  ImageIcon,
  ArrowUpRightIcon,
  CompassIcon,
  ChainIcon,
} from './icons';

const promptActions: PromptAction[] = [
  { key: 'summarize_mistake', label: '归纳错因', prompt: '请帮我归纳这道题的错因，并给出下次避免的方法。', icon: <CompassIcon /> },
  { key: 'explain_differently', label: '换种讲法', prompt: '请换一种更直观的方式讲解这道题。', icon: <SparkleIcon /> },
  { key: 'similar_question', label: '生成相似题', prompt: '请基于这道题生成一道相似题，并给出答案。', icon: <NotebookIcon /> },
  { key: 'review_plan', label: '加入复习计划', prompt: '请根据这道题生成一个短期复习计划。', icon: <ChainIcon /> },
];

interface ReasoningCenterProps {
  question: QuestionItem | null;
  analysis: AnalysisItem | null;
  messages: ChatMessage[];
  reply: string;
  setReply: (value: string) => void;
  onSendMessage: () => void;
  onRetryMessage: (message: ChatMessage) => void;
  onReanalyze: () => void;
  onGenerateLearningState: () => void;
  attachments: File[];
  onAddAttachments: (files: File[]) => void;
  onRemoveAttachment: (index: number) => void;
  reanalyzing: boolean;
  sendingMessage: boolean;
  generatingLearning: boolean;
}

export default function ReasoningCenter({
  question,
  analysis,
  messages,
  reply,
  setReply,
  onSendMessage,
  onRetryMessage,
  onReanalyze,
  onGenerateLearningState,
  attachments,
  onAddAttachments,
  onRemoveAttachment,
  reanalyzing,
  sendingMessage,
  generatingLearning,
}: ReasoningCenterProps) {
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  function handleAttachmentChange(event: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? []);
    onAddAttachments(files);
    event.target.value = '';
  }

  function applyPromptAction(action: PromptAction) {
    if (action.key === 'summarize_mistake') {
      onGenerateLearningState();
      return;
    }
    setReply(action.prompt);
    composerRef.current?.focus();
  }

  return (
    <section className="panel panel--center">
      <div className="center-layout">
        <header className="section-header">
          <div className="section-title">
            <HierarchyIcon size={20} />
            <h1>{question ? question.stem.slice(0, 18) : '题目工作台'}</h1>
          </div>
          <div className="section-meta">{question ? `ID: ${question.id}` : '未选择题目'}</div>
        </header>

        <div className="conversation-scroll">
          <div className="conversation-stack">
            {analysis ? (
              <article className="analysis-card">
                <div className="analysis-card-header">
                  <div className="analysis-card-title">
                    <SparkleIcon size={18} />
                    <span>AI 解析</span>
                  </div>
                  <span className="analysis-card-time">{formatTime(analysis.createdAt)}</span>
                </div>
                <div className="analysis-card-copy">
                  <p>
                    <strong>答案：</strong>
                    {analysis.content.answer || analysis.answer || '暂无'}
                  </p>
                  <p>{analysis.content.summary || '暂无解析摘要'}</p>
                  {analysis.content.taxonomySuggestion && (analysis.content.taxonomySuggestion.categoryName || analysis.content.taxonomySuggestion.tagNames?.length) ? (
                    <div className="taxonomy-suggestion" aria-label="AI 分类标签建议">
                      <span>AI 分类建议（待确认）</span>
                      {analysis.content.taxonomySuggestion.categoryName ? <strong>{analysis.content.taxonomySuggestion.categoryName}</strong> : null}
                      {analysis.content.taxonomySuggestion.tagNames?.map((tag) => <em key={tag}>{tag}</em>)}
                    </div>
                  ) : null}
                </div>
                <div className="analysis-card-actions">
                  <button className="text-button is-highlight" type="button" onClick={onReanalyze} disabled={reanalyzing}>
                    <SparkleIcon size={12} />
                    {reanalyzing ? '解析中...' : '重新解析'}
                  </button>
                  <button
                    className="text-button"
                    type="button"
                    onClick={() => {
                      composerRef.current?.focus();
                    }}
                  >
                    <ThumbsUpIcon size={14} />
                    发送追问
                  </button>
                </div>
              </article>
            ) : null}

            {messages.map((message) =>
              message.role === 'user' ? (
                <div key={message.id} className={`user-message${message.clientStatus === 'failed' ? ' is-failed' : ''}`}>
                  <div className="user-bubble">{message.message}</div>
                  <div className="message-meta">
                    <span className="section-meta">{formatTime(message.createdAt)}</span>
                    {message.clientStatus === 'pending' ? <span className="message-status">发送中...</span> : null}
                    {message.clientStatus === 'failed' ? (
                      <>
                        <span className="message-status is-error">发送失败</span>
                        <button className="text-button message-retry-button" type="button" onClick={() => onRetryMessage(message)}>
                          重新发送
                        </button>
                      </>
                    ) : null}
                  </div>
                </div>
              ) : (
                <div key={message.id} className={`ai-message${message.clientStatus === 'pending' ? ' is-pending' : ''}`}>
                  <div className="ai-message-row">
                    <div className="ai-badge" aria-label="AI 助手">
                      <img src={aiAvatar} alt="" />
                    </div>
                    <div className="ai-bubble">
                      {message.clientStatus === 'pending' ? (
                        <div className="thinking-indicator" aria-live="polite">
                          <span>正在思考</span>
                          <i />
                          <i />
                          <i />
                        </div>
                      ) : (
                        <div className="explanation-block">
                          <MarkdownRenderer content={message.message} />
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="message-meta">
                    <span className="section-meta">{formatTime(message.createdAt)}</span>
                  </div>
                </div>
              ),
            )}
          </div>
        </div>

        <footer className="prompt-area">
          <div className="chip-row agent-action-row">
            {promptActions.map((action) => (
              <button
                key={action.key}
                className="chip"
                type="button"
                onClick={() => applyPromptAction(action)}
              >
                {action.icon}
                <span>{action.key === 'summarize_mistake' && generatingLearning ? '归纳中...' : action.label}</span>
              </button>
            ))}
          </div>
          <div className="composer-card">
            <input
              ref={fileInputRef}
              className="sr-only"
              type="file"
              accept="image/*"
              multiple
              onChange={handleAttachmentChange}
            />
            <textarea
              ref={composerRef}
              className="composer-input composer-textarea"
              placeholder="输入追问内容..."
              value={reply}
              onChange={(event) => setReply(event.target.value)}
            />
            {attachments.length > 0 ? (
              <div className="attachment-list">
                {attachments.map((file, index) => (
                  <div className="attachment-item" key={`${file.name}-${index}`}>
                    <ImageIcon size={13} />
                    <span>{file.name}</span>
                    <button type="button" aria-label={`移除 ${file.name}`} onClick={() => onRemoveAttachment(index)}>
                      ×
                    </button>
                  </div>
                ))}
              </div>
            ) : null}
            <div className="composer-toolbar">
              <div className="toolbar-left">
                <button
                  className="toolbar-button"
                  type="button"
                  aria-label="附件"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <PaperclipIcon size={14} />
                </button>
                <button
                  className="toolbar-button"
                  type="button"
                  aria-label="图片"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <ImageIcon size={14} />
                </button>
              </div>
              <button className="send-button" type="button" onClick={onSendMessage} disabled={sendingMessage || (!reply.trim() && attachments.length === 0)}>
                {sendingMessage ? '等待中' : '发送'}
                <ArrowUpRightIcon size={12} />
              </button>
            </div>
          </div>
        </footer>
      </div>
    </section>
  );
}
