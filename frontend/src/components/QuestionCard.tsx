import React from 'react';
import { cx } from '../utils';
import type { QuestionItem } from '../types';
import { TrashIcon } from './icons';
import FavoriteButton from './FavoriteButton';

interface QuestionCardProps {
  item: QuestionItem;
  active: boolean;
  deleting: boolean;
  onSelect: (question: QuestionItem) => void;
  onDragStart: (event: React.DragEvent, questionId: number) => void;
  onDragEnd: () => void;
  onToggleFavorite: (questionId: number) => void;
  onRequestDelete: (questionId: number) => void;
}

export default function QuestionCard({
  item,
  active,
  deleting,
  onSelect,
  onDragStart,
  onDragEnd,
  onToggleFavorite,
  onRequestDelete,
}: QuestionCardProps) {
  const userTags = item.tags ?? [];
  const needsReview = item.qualityStatus === 'needs_review';
  const showHeader = userTags.length > 0 || item.isFavorited || needsReview;

  return (
    <article
      key={item.id}
      className={cx('question-card', active && 'is-active')}
      role="button"
      tabIndex={0}
      draggable
      onDragStart={(event) => onDragStart(event, item.id)}
      onDragEnd={onDragEnd}
      onClick={() => onSelect(item)}
    >
      <div className="question-card-main">
        {showHeader && (
          <div className={cx('question-card-header', userTags.length === 0 && 'is-flags-only')}>
            {userTags.length > 0 && (
              <div className="tag-group">
                {userTags.map((tag) => (
                  <span key={tag.id} className="tag tag--user">{tag.name}</span>
                ))}
              </div>
            )}
            {item.isFavorited && (
              <span className="question-card-favorite-mark" aria-label="已收藏">★</span>
            )}
            {needsReview && <span className="tag tag--review">待校准</span>}
          </div>
        )}
        <p className="question-card-body">{item.stem}</p>
      </div>
      <div className="question-card-actions">
        <FavoriteButton
          isFavorited={item.isFavorited}
          onToggle={() => onToggleFavorite(item.id)}
          size={14}
        />
        <button
          className="question-card-delete"
          type="button"
          aria-label="删除题目"
          disabled={deleting}
          onClick={(event) => {
            event.stopPropagation();
            onRequestDelete(item.id);
          }}
        >
          <TrashIcon size={14} />
        </button>
      </div>
    </article>
  );
}
