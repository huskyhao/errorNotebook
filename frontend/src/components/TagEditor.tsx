import React, { useState, useRef, useEffect } from 'react';
import { requestJson } from '../utils';
import type { TagItem } from '../types';

interface TagEditorProps {
  questionTags: TagItem[];
  allTags: TagItem[];
  onTagsChange: (tagIds: number[]) => void;
}

export default function TagEditor({ questionTags, allTags, onTagsChange }: TagEditorProps) {
  const [inputValue, setInputValue] = useState('');
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [creating, setCreating] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const currentTagIds = new Set(questionTags.map((t) => t.id));
  const suggestions = allTags.filter(
    (t) => !currentTagIds.has(t.id) && t.name.toLowerCase().includes(inputValue.toLowerCase())
  );

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setShowSuggestions(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  async function addTagById(tagId: number) {
    const newIds = Array.from(new Set([...questionTags.map((t) => t.id), tagId]));
    onTagsChange(newIds);
    setInputValue('');
    setShowSuggestions(false);
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
      setShowSuggestions(false);
    } catch {
      // ignore, user retries
    } finally {
      setCreating(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      const exact = suggestions.find((t) => t.name === inputValue.trim());
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
    <div className="tag-editor" ref={containerRef}>
      <div className="tag-editor-add-row">
        <span className="selector-label">标签</span>
        <div className="tag-editor-input-wrap">
          <input
            ref={inputRef}
            className="tag-editor-input"
            type="text"
            placeholder="添加知识点..."
            value={inputValue}
            onChange={(e) => {
              setInputValue(e.target.value);
              setShowSuggestions(true);
            }}
            onFocus={() => setShowSuggestions(true)}
            onKeyDown={handleKeyDown}
            disabled={creating}
          />
          {showSuggestions && (suggestions.length > 0 || inputValue.trim()) && (
            <div className="tag-suggestions">
              {suggestions.map((tag) => (
                <button
                  key={tag.id}
                  type="button"
                  className="tag-suggestion"
                  onClick={() => addTagById(tag.id)}
                >
                  {tag.name}
                </button>
              ))}
              {inputValue.trim() && !suggestions.some((t) => t.name === inputValue.trim()) && (
                <button
                  type="button"
                  className="tag-suggestion is-create"
                  onClick={createAndAddTag}
                  disabled={creating}
                >
                  {creating ? '创建中...' : `创建 "${inputValue.trim()}"`}
                </button>
              )}
            </div>
          )}
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
