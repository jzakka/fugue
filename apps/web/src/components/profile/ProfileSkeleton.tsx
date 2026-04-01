export default function ProfileSkeleton() {
  return (
    <div className="animate-pulse space-y-6">
      {/* Header skeleton */}
      <div className="bg-surface rounded-[16px] p-6 sm:p-8 border border-border">
        <div className="flex flex-col sm:flex-row gap-6">
          <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-full bg-surface-elevated shrink-0" />
          <div className="flex-1 space-y-3">
            <div className="h-8 bg-surface-elevated rounded w-48" />
            <div className="h-4 bg-surface-elevated rounded w-72" />
            <div className="flex gap-2">
              <div className="h-6 bg-surface-elevated rounded-full w-16" />
              <div className="h-6 bg-surface-elevated rounded-full w-14" />
            </div>
          </div>
        </div>
      </div>
      {/* Works skeleton */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="bg-surface rounded-[10px] overflow-hidden">
            <div className="h-48 bg-surface-elevated" />
            <div className="p-3 space-y-2">
              <div className="h-4 bg-surface-elevated rounded w-3/4" />
              <div className="h-3 bg-surface-elevated rounded w-1/3" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
