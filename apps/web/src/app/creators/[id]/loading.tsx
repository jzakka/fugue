import ProfileSkeleton from "@/components/profile/ProfileSkeleton";

export default function Loading() {
  return (
    <>
      <header>
        <nav className="sticky top-0 z-50 bg-bg border-b border-border px-6 py-4 flex items-center gap-6 backdrop-blur-sm skeleton-shimmer">
          <div className="flex items-center gap-2 shrink-0">
            <div className="w-8 h-8 bg-surface-elevated rounded-md" />
            <div className="h-6 w-16 bg-surface-elevated rounded" />
          </div>
          <div className="flex-1 max-w-md">
            <div className="h-[42px] bg-surface-elevated rounded-full" />
          </div>
          <div className="ml-auto flex items-center gap-4 shrink-0">
            <div className="w-9 h-9 bg-surface-elevated rounded-full" />
          </div>
        </nav>
      </header>
      <main id="main" className="flex-1 max-w-4xl mx-auto w-full px-6 py-8">
        <ProfileSkeleton />
      </main>
    </>
  );
}
