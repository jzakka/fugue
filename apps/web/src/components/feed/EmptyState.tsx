import type { ReactNode } from "react";

type EmptyStateProps = {
  message: string;
  description?: string;
  children?: ReactNode;
};

export default function EmptyState({
  message,
  description,
  children,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="text-5xl mb-4">🐡</div>
      <p className="text-text-muted text-sm mb-1">{message}</p>
      {description && (
        <p className="text-text-dim text-xs">{description}</p>
      )}
      {children && <div className="mt-4">{children}</div>}
    </div>
  );
}
