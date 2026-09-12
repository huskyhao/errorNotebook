import React, { useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import settingIcon from '../assets/icon/setting.svg';
import notificationIcon from '../assets/icon/notification.svg';
import { UploadIcon } from './icons';

interface TopBarProps {
  onImport?: (files: File[]) => void;
  importing?: boolean;
  importProgress?: { completed: number; total: number; failed: number } | null;
}

export default function TopBar({ onImport, importing = false, importProgress }: TopBarProps) {
  const fileRef = useRef<HTMLInputElement | null>(null);
  const navigate = useNavigate();

  const buttonLabel = importProgress && importProgress.total > 1
    ? `导入中 ${importProgress.completed}/${importProgress.total}`
    : importing
      ? '导入中'
      : '导入题目';

  return (
    <header className="topbar">
      <div className="brand-mark">ErroNotebook</div>
      <div className="topbar-actions">
        {onImport ? (
          <>
            <button className="primary-button" type="button" onClick={() => fileRef.current?.click()} disabled={importing}>
              <UploadIcon />
              <span>{buttonLabel}</span>
            </button>
            {importProgress && importProgress.total > 1 && (
              <div className="batch-progress-bar">
                <div
                  className="batch-progress-fill"
                  style={{ width: `${(importProgress.completed / importProgress.total) * 100}%` }}
                />
              </div>
            )}
          </>
        ) : null}
        <div className="topbar-icon-group">
          <button
            className="icon-button"
            type="button"
            aria-label="设置"
            onClick={() => navigate('/settings')}
          >
            <img src={settingIcon} alt="" aria-hidden="true" />
          </button>
          <button
            className="icon-button"
            type="button"
            aria-label="通知"
            onClick={() => console.log('[TopBar] 点击通知')}
          >
            <img src={notificationIcon} alt="" aria-hidden="true" />
          </button>
        </div>
      </div>
      {onImport ? (
        <input
          ref={fileRef}
          hidden
          type="file"
          accept="image/*"
          multiple
          onChange={(event) => {
            const files = Array.from(event.target.files ?? []);
            if (files.length > 0) onImport(files);
            event.target.value = '';
          }}
        />
      ) : null}
    </header>
  );
}
