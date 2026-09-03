import React from 'react';

function WorkspaceIcon({
  children,
  className,
  size = 18,
}: {
  children: React.ReactNode;
  className?: string;
  size?: number;
}) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export function LibraryIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M4 19.5V5.5A1.5 1.5 0 0 1 5.5 4H20" />
      <path d="M8 7h8" />
      <path d="M8 11h8" />
      <path d="M8 15h5" />
      <path d="M4 20h15" />
    </WorkspaceIcon>
  );
}

export function BookMarkedIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M6 3.5h10A1.5 1.5 0 0 1 17.5 5v15l-6-3-6 3V5A1.5 1.5 0 0 1 6 3.5Z" />
      <path d="M9 8h5" />
    </WorkspaceIcon>
  );
}

export function BarChartIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M4 20h16" />
      <path d="M7 16V9" />
      <path d="M12 16V5" />
      <path d="M17 16v-4" />
    </WorkspaceIcon>
  );
}

export function ArchiveIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M4 8h16v11a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V8Z" />
      <path d="M3 4h18v4H3z" />
      <path d="M10 12h4" />
    </WorkspaceIcon>
  );
}

export function HierarchyIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <circle cx="6" cy="6" r="2.5" />
      <circle cx="18" cy="6" r="2.5" />
      <circle cx="12" cy="18" r="2.5" />
      <path d="M8.5 6h7" />
      <path d="M12 8.5v7" />
    </WorkspaceIcon>
  );
}

export function SearchIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <circle cx="11" cy="11" r="6" />
      <path d="m16 16 4 4" />
    </WorkspaceIcon>
  );
}

export function SparkleIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M12 3.5 13.8 8 18.5 9.8 13.8 11.6 12 16.5 10.2 11.6 5.5 9.8 10.2 8 12 3.5Z" />
      <path d="M18 3v3" />
      <path d="M19.5 4.5h-3" />
    </WorkspaceIcon>
  );
}

export function CopyIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <rect x="9" y="9" width="10" height="11" rx="1.5" />
      <path d="M6 15H5a1 1 0 0 1-1-1V5.5A1.5 1.5 0 0 1 5.5 4H14a1 1 0 0 1 1 1v1" />
    </WorkspaceIcon>
  );
}

export function ThumbsUpIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M7 11v9H4V11h3Z" />
      <path d="M10 20h6.3a2 2 0 0 0 2-1.6l1-5.4a2 2 0 0 0-2-2.4H13l.5-3.3c.1-.8-.2-1.5-.8-2.1L12 4l-4 7v9" />
    </WorkspaceIcon>
  );
}

export function LinkIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M10.5 13.5 13.5 10.5" />
      <path d="M8 15.5H6a3.5 3.5 0 1 1 0-7h2" />
      <path d="M16 8.5h2a3.5 3.5 0 1 1 0 7h-2" />
    </WorkspaceIcon>
  );
}

export function NotebookIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M7 4h10a2 2 0 0 1 2 2v14H7a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z" />
      <path d="M9 8h6" />
      <path d="M9 12h6" />
      <path d="M9 16h4" />
    </WorkspaceIcon>
  );
}

export function PaperclipIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="m8 12 6.5-6.5a3 3 0 1 1 4.2 4.2L9.6 18.8a4 4 0 0 1-5.7-5.7l8.5-8.5" />
    </WorkspaceIcon>
  );
}

export function ImageIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <rect x="4" y="5" width="16" height="14" rx="1.5" />
      <circle cx="9" cy="10" r="1.5" />
      <path d="m20 16-4.5-4.5L8 19" />
    </WorkspaceIcon>
  );
}

export function ArrowUpRightIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M7 17 17 7" />
      <path d="M9 7h8v8" />
    </WorkspaceIcon>
  );
}

export function RefreshIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M20 11a8 8 0 0 0-14.5-4" />
      <path d="M4 4v5h5" />
      <path d="M4 13a8 8 0 0 0 14.5 4" />
      <path d="M20 20v-5h-5" />
    </WorkspaceIcon>
  );
}

export function EditIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M4 20h4l10-10-4-4L4 16v4Z" />
      <path d="m12.5 5.5 4 4" />
    </WorkspaceIcon>
  );
}

export function CheckIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="m5 12 4 4 10-10" />
    </WorkspaceIcon>
  );
}

export function ChainIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M8.5 8.5 11 6a3 3 0 0 1 4.2 4.2L13 12.5" />
      <path d="m15.5 15.5-2.5 2.5a3 3 0 0 1-4.2-4.2L11 11.5" />
    </WorkspaceIcon>
  );
}

export function CompassIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <circle cx="12" cy="12" r="8" />
      <path d="m9 15 2-6 6-2-2 6-6 2Z" />
    </WorkspaceIcon>
  );
}

export function AlertTriangleIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M12 4 21 20H3L12 4Z" />
      <path d="M12 9v4" />
      <path d="M12 17h.01" />
    </WorkspaceIcon>
  );
}

export function ChevronUpIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="m6 14 6-6 6 6" />
    </WorkspaceIcon>
  );
}

export function ChevronDownIcon({ size = 18, className }: { size?: number; className?: string }) {
  return (
    <WorkspaceIcon size={size} className={className}>
      <path d="m6 10 6 6 6-6" />
    </WorkspaceIcon>
  );
}

export function ChevronLeftIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="m14 6-6 6 6 6" />
    </WorkspaceIcon>
  );
}

export function ChevronRightIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="m10 6 6 6-6 6" />
    </WorkspaceIcon>
  );
}

export function PlusIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M12 5v14M5 12h14" />
    </WorkspaceIcon>
  );
}

export function DocumentIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M7 3.5h7l4 4V20H7a2 2 0 0 1-2-2V5.5a2 2 0 0 1 2-2Z" />
      <path d="M14 3.5V8h4" />
    </WorkspaceIcon>
  );
}

export function TrashIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M4 6h16" />
      <path d="M7 6V4a1 1 0 0 1 1-1h8a1 1 0 0 1 1 1v2" />
      <path d="M18 6v12a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V6" />
      <path d="M10 10v5" />
      <path d="M14 10v5" />
    </WorkspaceIcon>
  );
}

export function UploadIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M12 16V4" />
      <path d="m7 9 5-5 5 5" />
      <path d="M5 19h14" />
    </WorkspaceIcon>
  );
}

export function SettingsIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1 1 0 0 0 .2 1.1l.1.1a1 1 0 0 1 0 1.4l-1 1a1 1 0 0 1-1.4 0l-.1-.1a1 1 0 0 0-1.1-.2 1 1 0 0 0-.6.9V20a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1v-.2a1 1 0 0 0-.6-.9 1 1 0 0 0-1.1.2l-.1.1a1 1 0 0 1-1.4 0l-1-1a1 1 0 0 1 0-1.4l.1-.1a1 1 0 0 0 .2-1.1 1 1 0 0 0-.9-.6H4a1 1 0 0 1-1-1v-2a1 1 0 0 1 1-1h.2a1 1 0 0 0 .9-.6 1 1 0 0 0-.2-1.1l-.1-.1a1 1 0 0 1 0-1.4l1-1a1 1 0 0 1 1.4 0l.1.1a1 1 0 0 0 1.1.2 1 1 0 0 0 .6-.9V4a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v.2a1 1 0 0 0 .6.9 1 1 0 0 0 1.1-.2l.1-.1a1 1 0 0 1 1.4 0l1 1a1 1 0 0 1 0 1.4l-.1.1a1 1 0 0 0-.2 1.1 1 1 0 0 0 .9.6H20a1 1 0 0 1 1 1v2a1 1 0 0 1-1 1h-.2a1 1 0 0 0-.9.6Z" />
    </WorkspaceIcon>
  );
}

export function StarIcon({ size = 18, filled = false }: { size?: number; filled?: boolean }) {
  if (filled) {
    return (
      <WorkspaceIcon size={size}>
        <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" fill="currentColor" stroke="none" />
      </WorkspaceIcon>
    );
  }
  return (
    <WorkspaceIcon size={size}>
      <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
    </WorkspaceIcon>
  );
}

export function TagIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M4 4h6l6 6-7 7-6-6V4Z" />
      <circle cx="8" cy="8" r="1" fill="currentColor" stroke="none" />
    </WorkspaceIcon>
  );
}

export function BellIcon({ size = 18 }: { size?: number }) {
  return (
    <WorkspaceIcon size={size}>
      <path d="M6 9a6 6 0 1 1 12 0c0 6 2 7 2 7H4s2-1 2-7" />
      <path d="M10 20a2 2 0 0 0 4 0" />
    </WorkspaceIcon>
  );
}
