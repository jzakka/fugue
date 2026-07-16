import HideOnErrorImage from "@/components/ui/HideOnErrorImage";
import type { CreatorPublic } from "@/lib/api";

export default function ProfileHeader({
  creator,
  isOwner,
  onEdit,
}: {
  creator: CreatorPublic;
  isOwner?: boolean;
  onEdit?: () => void;
}) {
  return (
    <div className="bg-surface rounded-[16px] p-6 sm:p-8 border border-border">
      <div className="flex flex-col sm:flex-row gap-6">
        {/* Avatar */}
        <div className="shrink-0">
          {creator.avatar_url ? (
            <HideOnErrorImage
              src={creator.avatar_url}
              alt=""
              className="w-20 h-20 sm:w-24 sm:h-24 rounded-full border-2 border-border object-cover"
            />
          ) : (
            <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-full bg-gradient-to-br from-accent to-accent-hover border-2 border-border" />
          )}
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h1 className="text-2xl sm:text-3xl font-bold tracking-tight font-display break-words">
                {creator.nickname}
              </h1>
            </div>
            {isOwner && onEdit && (
              <button
                onClick={onEdit}
                className="shrink-0 px-4 py-2 bg-surface-elevated border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors cursor-pointer"
              >
                편집
              </button>
            )}
          </div>

          {/* Pin count */}
          <div className="mt-4 text-sm text-text-muted">
            <span className="font-medium font-mono">
              {creator.pin_count} pins
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
