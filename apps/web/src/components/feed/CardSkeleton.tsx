export default function CardSkeleton() {
  return (
    <div className="bg-surface rounded-[10px] overflow-hidden animate-pulse">
      <div className="h-48 bg-surface-elevated" />
      <div className="p-3 space-y-2">
        <div className="h-4 bg-surface-elevated rounded w-3/4" />
        <div className="flex items-center gap-2">
          <div className="w-5 h-5 rounded-full bg-surface-elevated" />
          <div className="h-3 bg-surface-elevated rounded w-1/3" />
        </div>
        <div className="flex gap-1">
          <div className="h-4 bg-surface-elevated rounded-full w-12" />
          <div className="h-4 bg-surface-elevated rounded-full w-10" />
        </div>
      </div>
    </div>
  );
}
