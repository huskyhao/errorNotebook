import React from 'react';
import { StarIcon } from './icons';

interface FavoriteButtonProps {
  isFavorited: boolean;
  onToggle: () => void;
  size?: number;
}

export default function FavoriteButton({ isFavorited, onToggle, size = 18 }: FavoriteButtonProps) {
  return (
    <button
      className={`favorite-btn${isFavorited ? ' is-favorited' : ''}`}
      type="button"
      aria-label={isFavorited ? '取消收藏' : '收藏'}
      onClick={(e) => {
        e.stopPropagation();
        onToggle();
      }}
    >
      <StarIcon size={size} filled={isFavorited} />
    </button>
  );
}
