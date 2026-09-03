import React, { useMemo, useState } from 'react';
import { cx } from '../utils';
import type { QuestionItem, CategoryTreeNode, TagItem } from '../types';
import { SearchIcon, ChevronLeftIcon, ChevronRightIcon, StarIcon, PlusIcon, ChevronDownIcon, TrashIcon } from './icons';
import MoveTargetPopover, { MoveTarget } from './MoveTargetPopover';
import QuestionCard from './QuestionCard';
import ConfirmDialog from './ConfirmDialog';

interface QuestionSidebarProps {
  questions: QuestionItem[];
  activeId: number | null;
  onSelect: (question: QuestionItem) => void;
  onDelete: () => void;
  deletingId: number | null;
  deleteConfirmId: number | null;
  setDeleteConfirmId: (id: number | null) => void;
  searchQuery: string;
  setSearchQuery: (value: string) => void;
  sidebarOpen: boolean;
  onToggleSidebar: () => void;
  categoryTree: CategoryTreeNode[];
  uncategorizedCount: number;
  totalCount: number;
  tags: TagItem[];
  activeTagFilter: number | null;
  onTagFilter: (tagId: number | null) => void;
  favoritedFilter: boolean;
  onFavoritedFilter: (val: boolean) => void;
  reviewFilter: boolean;
  onReviewFilter: (val: boolean) => void;
  onToggleFavorite: (questionId: number) => void;
  onCreateCategory: (name: string) => Promise<CategoryTreeNode | null>;
  onRequestDeleteCategory: (category: CategoryTreeNode) => void;
  onMoveQuestion: (questionId: number, categoryId: number | null) => void;
}

function dragStateClass(dragOver: string | null, targetKey: string): string {
  if (dragOver === targetKey) return ' drag-over';
  return '';
}

export default function QuestionSidebar({
  questions, activeId, onSelect, onDelete, deletingId, deleteConfirmId,
  setDeleteConfirmId, searchQuery, setSearchQuery, sidebarOpen, onToggleSidebar,
  categoryTree, uncategorizedCount, totalCount, tags, activeTagFilter, onTagFilter,
  favoritedFilter, onFavoritedFilter, onToggleFavorite, onCreateCategory, onMoveQuestion,
  reviewFilter, onReviewFilter,
  onRequestDeleteCategory,
}: QuestionSidebarProps) {
  const isFiltering = searchQuery.trim() !== '' || activeTagFilter !== null || favoritedFilter || reviewFilter;
  const reviewCount = questions.filter((item) => item.qualityStatus === 'needs_review').length;
  const [expandedCats, setExpandedCats] = useState<Set<string>>(() => {
    const s = new Set<string>();
    s.add('all');
    s.add('uncategorized');
    categoryTree.forEach((c) => s.add(String(c.id)));
    return s;
  });
  const [newCatName, setNewCatName] = useState('');
  const [showNewCat, setShowNewCat] = useState(false);
  const [dragOver, setDragOver] = useState<string | null>(null);
  const [draggingQuestionId, setDraggingQuestionId] = useState<number | null>(null);
  const [movePopoverStyle, setMovePopoverStyle] = useState<React.CSSProperties | undefined>();

  const toggleCat = (key: string) => {
    setExpandedCats((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  async function handleCreateCategory() {
    const name = newCatName.trim();
    if (!name) return;
    const cat = await onCreateCategory(name);
    if (cat) {
      setNewCatName('');
      setShowNewCat(false);
      setExpandedCats((prev) => new Set(prev).add(String(cat.id)));
    }
  }

  const grouped = useMemo(() => {
    const uncat: QuestionItem[] = [];
    const byCat = new Map<number, QuestionItem[]>();
    categoryTree.forEach((c) => byCat.set(c.id, []));
    for (const q of questions) {
      if (q.categoryId != null && byCat.has(q.categoryId)) {
        byCat.get(q.categoryId)!.push(q);
      } else {
        uncat.push(q);
      }
    }
    return { all: questions, uncat, byCat };
  }, [questions, categoryTree]);

  const filteredQuestions = useMemo(() => {
    let items = questions;
    if (reviewFilter) {
      items = items.filter((item) => item.qualityStatus === 'needs_review');
    }
    if (!searchQuery.trim()) return items;
    const q = searchQuery.trim().toLowerCase();
    return items.filter((item) => item.stem.toLowerCase().includes(q));
  }, [questions, searchQuery, reviewFilter]);

  // ---- Drag handlers ----
  function handleDragStart(e: React.DragEvent, questionId: number) {
    e.dataTransfer.setData('text/plain', String(questionId));
    e.dataTransfer.effectAllowed = 'move';
    const sidebar = e.currentTarget.closest('.panel--sidebar');
    const rect = sidebar?.getBoundingClientRect();
    if (rect) {
      const width = 248;
      const gap = 12;
      const top = Math.min(rect.top + 136, window.innerHeight - 280);
      const left = Math.min(rect.right + gap, window.innerWidth - width - 12);
      setMovePopoverStyle({ top: Math.max(72, top), left, width });
    }
    setDraggingQuestionId(questionId);
  }
  function handleDragOverCat(e: React.DragEvent, key: string) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    setDragOver(key);
    setExpandedCats((prev) => new Set(prev).add(key));
  }
  function handleDragLeaveCat() {
    setDragOver(null);
  }
  function handleDropOnCat(e: React.DragEvent, catId: number | null) {
    e.preventDefault();
    setDragOver(null);
    setDraggingQuestionId(null);
    setMovePopoverStyle(undefined);
    const qid = parseInt(e.dataTransfer.getData('text/plain'), 10);
    if (!isNaN(qid)) onMoveQuestion(qid, catId);
  }
  function handleDragEnd() {
    setDragOver(null);
    setDraggingQuestionId(null);
    setMovePopoverStyle(undefined);
  }

  const moveTargets = useMemo<MoveTarget[]>(
    () => [
      { key: 'uncategorized', id: null, name: '未分类' },
      ...categoryTree.map((cat) => ({ key: String(cat.id), id: cat.id, name: cat.name })),
    ],
    [categoryTree],
  );

  function handleDragOverMoveTarget(e: React.DragEvent, key: string) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    setDragOver(key);
  }

  function renderMoveOverlay() {
    if (draggingQuestionId == null) return null;
    return (
      <MoveTargetPopover
        targets={moveTargets}
        activeTargetKey={dragOver}
        style={movePopoverStyle}
        onDragOverTarget={handleDragOverMoveTarget}
        onDragLeaveTarget={handleDragLeaveCat}
        onDropTarget={handleDropOnCat}
      />
    );
  }

  // ---- Render helpers ----
  function renderCard(item: QuestionItem) {
    return (
      <QuestionCard
        key={item.id}
        item={item}
        active={activeId === item.id}
        deleting={deletingId === item.id}
        onSelect={onSelect}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        onToggleFavorite={onToggleFavorite}
        onRequestDelete={setDeleteConfirmId}
      />
    );
  }

  // ---- Filter mode ----
  if (isFiltering) {
    return (
      <aside className={cx('panel panel--sidebar', !sidebarOpen && 'collapsed')} aria-label="题目列表">
        <div className="panel-reveal">
          <button className="panel-toggle" type="button" aria-label="展开题目列表" onClick={onToggleSidebar}>
            <ChevronRightIcon size={16} />
          </button>
        </div>
        <div className="sidebar-head">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <h2 className="sidebar-heading">筛选结果</h2>
            <button className="panel-toggle" type="button" aria-label="收起题目列表" onClick={onToggleSidebar}>
              <ChevronLeftIcon size={16} />
            </button>
          </div>
          <label className="searchbox">
            <SearchIcon size={15} />
            <span className="sr-only">搜索题目</span>
            <input value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} placeholder="搜索题干" />
          </label>
          <div className="filter-bar">
            <button className={cx('chip', favoritedFilter && 'is-active')} type="button" onClick={() => onFavoritedFilter(!favoritedFilter)}>
              <StarIcon size={12} filled={favoritedFilter} /><span style={{ marginLeft: 4 }}>收藏</span>
            </button>
            <button className={cx('chip', reviewFilter && 'is-active')} type="button" onClick={() => onReviewFilter(!reviewFilter)}>
              <span>待校准 {reviewCount}</span>
            </button>
            <select className="filter-select" value={activeTagFilter ?? ''} onChange={(e) => onTagFilter(e.target.value ? Number(e.target.value) : null)}>
              <option value="">全部标签</option>
              {tags.map((tag) => (<option key={tag.id} value={tag.id}>{tag.name}</option>))}
            </select>
          </div>
        </div>
        <div className="question-list">
          {filteredQuestions.map((item) => renderCard(item))}
          {filteredQuestions.length === 0 && (
            <div className="empty-state">{searchQuery.trim() ? '没有匹配的题目' : '暂无题目'}</div>
          )}
        </div>
        {deleteConfirmId != null && (
          <ConfirmDialog
            title="删除题目"
            description="确定删除该题目？此操作不可撤销。"
            confirmText="确认删除"
            danger
            loading={deletingId === deleteConfirmId}
            onCancel={() => setDeleteConfirmId(null)}
            onConfirm={onDelete}
          />
        )}
        {renderMoveOverlay()}
      </aside>
    );
  }

  // ---- Browse mode ----
  return (
    <aside className={cx('panel panel--sidebar', !sidebarOpen && 'collapsed')} aria-label="题目列表">
      <div className="panel-reveal">
        <button className="panel-toggle" type="button" aria-label="展开题目列表" onClick={onToggleSidebar}>
          <ChevronRightIcon size={16} />
        </button>
      </div>
      <div className="sidebar-head">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h2 className="sidebar-heading">题库</h2>
          <button className="panel-toggle" type="button" aria-label="收起题目列表" onClick={onToggleSidebar}>
            <ChevronLeftIcon size={16} />
          </button>
        </div>
        <label className="searchbox">
          <SearchIcon size={15} />
          <span className="sr-only">搜索题目</span>
          <input value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} placeholder="搜索题干" />
        </label>
        <div className="filter-bar">
          <button className={cx('chip', favoritedFilter && 'is-active')} type="button" onClick={() => onFavoritedFilter(!favoritedFilter)}>
            <StarIcon size={12} filled={favoritedFilter} /><span style={{ marginLeft: 4 }}>收藏</span>
          </button>
          <button className={cx('chip', reviewFilter && 'is-active')} type="button" onClick={() => onReviewFilter(!reviewFilter)}>
            <span>待校准 {reviewCount}</span>
          </button>
          <select className="filter-select" value={activeTagFilter ?? ''} onChange={(e) => onTagFilter(e.target.value ? Number(e.target.value) : null)}>
            <option value="">全部标签</option>
            {tags.map((tag) => (<option key={tag.id} value={tag.id}>{tag.name}</option>))}
          </select>
        </div>
      </div>

      <div className="category-browser">
        {/* All */}
        <div className="category-section">
          <button
            className={cx('category-section-header', dragStateClass(dragOver, 'all'))}
            type="button"
            onClick={() => toggleCat('all')}
            onDragOver={(e) => handleDragOverCat(e, 'all')}
            onDragLeave={handleDragLeaveCat}
            onDrop={(e) => handleDropOnCat(e, null)}
          >
            <ChevronDownIcon size={12} className={cx('cat-chevron', !expandedCats.has('all') && 'collapsed')} />
            <span style={{ flex: 1, textAlign: 'left' }}>全部题目</span>
            <span className="category-count">{totalCount}</span>
          </button>
          {expandedCats.has('all') && (
            <div className="category-section-body">
              {grouped.all.map((item) => renderCard(item))}
            </div>
          )}
        </div>

        {/* Uncategorized */}
        {grouped.uncat.length > 0 && (
          <div className="category-section">
            <button
              className={cx('category-section-header', dragStateClass(dragOver, 'uncategorized'))}
              type="button"
              onClick={() => toggleCat('uncategorized')}
              onDragOver={(e) => handleDragOverCat(e, 'uncategorized')}
              onDragLeave={handleDragLeaveCat}
              onDrop={(e) => handleDropOnCat(e, null)}
            >
              <ChevronDownIcon size={12} className={cx('cat-chevron', !expandedCats.has('uncategorized') && 'collapsed')} />
              <span style={{ flex: 1, textAlign: 'left' }}>未分类</span>
              <span className="category-count">{grouped.uncat.length}</span>
            </button>
            {expandedCats.has('uncategorized') && (
              <div className="category-section-body">
                {grouped.uncat.map((item) => renderCard(item))}
              </div>
            )}
          </div>
        )}

        {/* Named categories */}
        {categoryTree.map((cat) => {
          const items = grouped.byCat.get(cat.id) ?? [];
          const key = String(cat.id);
          return (
            <div key={cat.id} className="category-section">
              <button
                className={cx('category-section-header', dragStateClass(dragOver, key))}
                type="button"
                onClick={() => toggleCat(key)}
                onDragOver={(e) => handleDragOverCat(e, key)}
                onDragLeave={handleDragLeaveCat}
                onDrop={(e) => handleDropOnCat(e, cat.id)}
              >
                <ChevronDownIcon size={12} className={cx('cat-chevron', !expandedCats.has(key) && 'collapsed')} />
                <span style={{ flex: 1, textAlign: 'left' }}>{cat.name}</span>
                <span className="category-count">{items.length}</span>
                <span
                  className="category-delete-action"
                  role="button"
                  tabIndex={0}
                  aria-label={`删除分类 ${cat.name}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    onRequestDeleteCategory(cat);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      e.stopPropagation();
                      onRequestDeleteCategory(cat);
                    }
                  }}
                >
                  <TrashIcon size={12} />
                </span>
              </button>
              {expandedCats.has(key) && (
                <div className="category-section-body">
                  {items.map((item) => renderCard(item))}
                </div>
              )}
            </div>
          );
        })}

        {/* New category */}
        {showNewCat ? (
          <div className="category-new-form">
            <input
              className="form-input form-input--sm"
              value={newCatName}
              onChange={(e) => setNewCatName(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleCreateCategory(); if (e.key === 'Escape') { setShowNewCat(false); setNewCatName(''); } }}
              placeholder="分类名称"
              autoFocus
            />
            <button className="text-button" type="button" onClick={handleCreateCategory}>确定</button>
            <button className="text-button" type="button" onClick={() => { setShowNewCat(false); setNewCatName(''); }}>取消</button>
          </div>
        ) : (
          <button
            className="category-new-btn"
            type="button"
            onClick={() => setShowNewCat(true)}
          >
            <PlusIcon size={12} />
            <span>新建分类</span>
          </button>
        )}
      </div>

      {deleteConfirmId != null && (
        <ConfirmDialog
          title="删除题目"
          description="确定删除该题目？此操作不可撤销。"
          confirmText="确认删除"
          danger
          loading={deletingId === deleteConfirmId}
          onCancel={() => setDeleteConfirmId(null)}
          onConfirm={onDelete}
        />
      )}
      {renderMoveOverlay()}
    </aside>
  );
}
