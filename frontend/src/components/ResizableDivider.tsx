import React, { useCallback, useEffect, useRef } from 'react';

interface ResizableDividerProps {
  onResize: (delta: number) => void;
  onDragStart?: () => void;
  onDragEnd?: () => void;
  disabled?: boolean;
}

export default function ResizableDivider({ onResize, onDragStart, onDragEnd, disabled }: ResizableDividerProps) {
  const dragging = useRef(false);
  const lastX = useRef(0);

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      if (disabled) return;
      e.preventDefault();
      dragging.current = true;
      lastX.current = e.clientX;
      onDragStart?.();
    },
    [disabled, onDragStart],
  );

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!dragging.current) return;
      const delta = e.clientX - lastX.current;
      lastX.current = e.clientX;
      onResize(delta);
    },
    [onResize],
  );

  const handleMouseUp = useCallback(() => {
    if (!dragging.current) return;
    dragging.current = false;
    onDragEnd?.();
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }, [onDragEnd]);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!dragging.current) return;
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
      handleMouseMove(e);
    };
    const onUp = () => handleMouseUp();
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
  }, [handleMouseMove, handleMouseUp]);

  return (
    <div
      className={`resize-handle${disabled ? ' is-disabled' : ''}`}
      role="separator"
      aria-orientation="vertical"
      aria-label="拖拽调整面板宽度"
      onMouseDown={handleMouseDown}
    >
      <div className="resize-handle-grip" />
    </div>
  );
}
