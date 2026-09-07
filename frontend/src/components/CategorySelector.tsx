import React from 'react';
import type { CategoryTreeNode } from '../types';

interface CategorySelectorProps {
  categories: CategoryTreeNode[];
  currentCategoryId: number | null | undefined;
  onChange: (categoryId: number | null) => void;
}

export default function CategorySelector({ categories, currentCategoryId, onChange }: CategorySelectorProps) {
  return (
    <div className="category-selector">
      <span className="selector-label">学科</span>
      <select
        className="form-input"
        value={currentCategoryId ?? ''}
        onChange={(e) => {
          const val = e.target.value;
          onChange(val ? Number(val) : null);
        }}
      >
        <option value="">无分类</option>
        {categories.map((item) => (
          <option key={item.id} value={item.id}>
            {item.name}
          </option>
        ))}
      </select>
    </div>
  );
}
