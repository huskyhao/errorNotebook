import React from 'react';

interface ConfirmDialogProps {
  title: string;
  description: string;
  cancelText?: string;
  confirmText?: string;
  danger?: boolean;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

export default function ConfirmDialog({
  title,
  description,
  cancelText = '取消',
  confirmText = '确认',
  danger = false,
  loading = false,
  onCancel,
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <div className="confirm-overlay" onClick={loading ? undefined : onCancel}>
      <div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-title" onClick={(e) => e.stopPropagation()}>
        <h2 id="confirm-title" className="confirm-dialog-title">{title}</h2>
        <p className="confirm-dialog-msg">{description}</p>
        <div className="confirm-dialog-actions">
          <button className="confirm-dialog-button outline-button" type="button" onClick={onCancel} disabled={loading}>
            {cancelText}
          </button>
          <button className={`confirm-dialog-button primary-button${danger ? ' danger-button' : ''}`} type="button" onClick={onConfirm} disabled={loading}>
            {loading ? '删除中...' : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
