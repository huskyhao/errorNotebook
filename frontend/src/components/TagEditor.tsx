import React, { useState } from 'react';
import { requestJson } from '../utils';
import type { TagItem } from '../types';

interface TagEditorProps {
  questionTags: TagItem[];
  allTags: TagItem[];
  onTagsChange: (tagIds: number[]) => void;
}

export default function TagEditor({ questionTags, allTags, onTagsChange }: TagEditorProps) {
  const [inputValue, setInputValue] = useState('');
  const [creating, setCreating] = useState(false);

  const currentTagIds = new Set(questionTags.map((t) => t.id));

  async function addTagById(tagId: number) {
    const newIds = Array.from(new Set([...questionTags.map((t) => t.id), tagId]));
    onTagsChange(newIds);
    setInputValue('');
  }

  async function removeTag(tagId: number) {
    const newIds = questionTags.filter((t) => t.id !== tagId).map((t) => t.id);
    onTagsChange(newIds);
  }

  async function createAndAddTag() {
    const name = inputValue.trim();
    if (!name) return;
    setCreating(true);
    try {
      const tag = await requestJson<TagItem>('/tags', { method: 'POST', body: JSON.stringify({ name }) });
      const newIds = [...questionTags.map((t) => t.id), tag.id];
      onTagsChange(newIds);
      setInputValue('');
    } catch {
      // ignore, user retries
    } finally {
      setCreating(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      const exact = allTags.find((t) => !currentTagIds.has(t.id) && t.name === inputValue.trim());
      if (exact) {
        addTagById(exact.id);
      } else if (inputValue.trim()) {
        createAndAddTag();
      }
    }
    if (e.key === 'Backspace' && !inputValue && questionTags.length > 0) {
      removeTag(questionTags[questionTags.length - 1].id);
    }
  }

  return (
    <div className="tag-editor">
      <div className="tag-editor-add-row">
        <span className="selector-label">标签</span>
        <div className="tag-editor-input-wrap">
          <input
            className="tag-editor-input"
            type="text"
            placeholder="添加知识点..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={creating}
          />
        </div>
      </div>
      {questionTags.length > 0 && (
        <div className="tag-editor-chip-list" aria-label="已添加标签">
          {questionTags.map((tag) => (
            <span key={tag.id} className="tag-chip">
              {tag.name}
              <button
                type="button"
                className="tag-chip-remove"
                aria-label={`移除标签 ${tag.name}`}
                onClick={() => removeTag(tag.id)}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
