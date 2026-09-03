import React from 'react';
import type { CategoryTreeNode } from '../types';

interface CategorySelectorProps {
  categories: CategoryTreeNode[];
  currentCategoryId: number | null | undefined;
  onChange: (categoryId: number | null) => void;
}

export default function CategorySelector({ categories, currentCategoryId, onChange }: CategorySelectorProps) {
  const tree = buildTree(categories);
  const flat = flattenTree(tree, 0);

  return (
    <div className="category-selector">
      <span className="selector-label">分类</span>
      <select
        className="form-input"
        value={currentCategoryId ?? ''}
        onChange={(e) => {
          const val = e.target.value;
          onChange(val ? Number(val) : null);
        }}
      >
        <option value="">无分类</option>
        {flat.map((item) => (
          <option key={item.id} value={item.id}>
            {'  '.repeat(item.depth)}{item.name}
          </option>
        ))}
      </select>
    </div>
  );
}

function buildTree(items: CategoryTreeNode[]): TreeNode[] {
  const map = new Map<number, TreeNode>();
  const roots: TreeNode[] = [];
  for (const item of items) {
    map.set(item.id, { ...item, children: [] });
  }
  for (const item of items) {
    const node = map.get(item.id)!;
    if (item.parentId != null && map.has(item.parentId)) {
      map.get(item.parentId)!.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

function flattenTree(nodes: TreeNode[], depth: number): FlattenedCategory[] {
  const result: FlattenedCategory[] = [];
  for (const node of nodes) {
    result.push({ id: node.id, name: node.name, depth });
    result.push(...flattenTree(node.children, depth + 1));
  }
  return result;
}

type TreeNode = CategoryTreeNode & { children: TreeNode[] };
type FlattenedCategory = { id: number; name: string; depth: number };
