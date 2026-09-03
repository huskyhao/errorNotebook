import React from 'react';
import { cx } from '../utils';

export type MoveTarget = {
  key: string;
  id: number | null;
  name: string;
};

interface MoveTargetPopoverProps {
  targets: MoveTarget[];
  activeTargetKey: string | null;
  style?: React.CSSProperties;
  onDragOverTarget: (event: React.DragEvent, targetKey: string) => void;
  onDragLeaveTarget: () => void;
  onDropTarget: (event: React.DragEvent, categoryId: number | null) => void;
}

export default function MoveTargetPopover({
  targets,
  activeTargetKey,
  style,
  onDragOverTarget,
  onDragLeaveTarget,
  onDropTarget,
}: MoveTargetPopoverProps) {
  return (
    <div className="move-target-popover" style={style} aria-label="移动到分类">
      <div className="move-target-title">移动到</div>
      <div className="move-target-list">
        {targets.map((target) => (
          <button
            key={target.key}
            className={cx('move-target-item', activeTargetKey === target.key && 'drag-over')}
            type="button"
            onDragOver={(event) => onDragOverTarget(event, target.key)}
            onDragLeave={onDragLeaveTarget}
            onDrop={(event) => onDropTarget(event, target.id)}
          >
            <span>{target.name}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
