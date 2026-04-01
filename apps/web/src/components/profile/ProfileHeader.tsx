import type { CreatorPublic } from "@/lib/api";

const ROLE_COLORS: Record<string, string> = {
  작곡: "bg-blue-500/15 text-blue-400",
  보컬: "bg-pink-500/15 text-pink-400",
  일러스트: "bg-purple-500/15 text-purple-400",
  영상: "bg-green-500/15 text-green-400",
  글: "bg-yellow-500/15 text-yellow-400",
};

function getRoleColor(role: string) {
  return ROLE_COLORS[role] || "bg-accent-subtle text-accent";
}

const CONTACT_LABELS: Record<string, string> = {
  twitter: "Twitter",
  discord: "Discord",
  instagram: "Instagram",
  youtube: "YouTube",
  soundcloud: "SoundCloud",
  pixiv: "pixiv",
  website: "Website",
  email: "Email",
};

export default function ProfileHeader({
  creator,
  isOwner,
  onEdit,
}: {
  creator: CreatorPublic;
  isOwner?: boolean;
  onEdit?: () => void;
}) {
  const contacts = creator.contacts || {};
  const contactEntries = Object.entries(contacts).filter(
    ([, v]) => v && v.trim() !== ""
  );

  return (
    <div className="bg-surface rounded-[16px] p-6 sm:p-8 border border-border">
      <div className="flex flex-col sm:flex-row gap-6">
        {/* Avatar */}
        <div className="shrink-0">
          {creator.avatar_url ? (
            <img
              src={creator.avatar_url}
              alt={creator.nickname}
              className="w-20 h-20 sm:w-24 sm:h-24 rounded-full border-2 border-border object-cover"
            />
          ) : (
            <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-full bg-gradient-to-br from-accent to-orange-400 border-2 border-border" />
          )}
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl sm:text-3xl font-bold tracking-tight">
                {creator.nickname}
              </h1>
              {creator.bio && (
                <p className="mt-2 text-text-muted text-sm sm:text-base leading-relaxed">
                  {creator.bio}
                </p>
              )}
            </div>
            {isOwner && onEdit && (
              <button
                onClick={onEdit}
                className="shrink-0 px-4 py-2 bg-surface-elevated border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent transition-colors cursor-pointer"
              >
                편집
              </button>
            )}
          </div>

          {/* Roles */}
          {creator.roles.length > 0 && (
            <div className="mt-4 flex flex-wrap gap-2">
              {creator.roles.map((role) => (
                <span
                  key={role}
                  className={`px-3 py-1 rounded-full text-xs font-medium ${getRoleColor(role)}`}
                >
                  {role}
                </span>
              ))}
            </div>
          )}

          {/* Stats + Contacts */}
          <div className="mt-4 flex flex-wrap items-center gap-4 text-sm text-text-muted">
            <span
              className="font-medium"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              {creator.work_count} works
            </span>

            {contactEntries.length > 0 && (
              <>
                <span className="text-border">|</span>
                {contactEntries.map(([key, value]) => (
                  <span key={key} className="text-text-dim hover:text-text-muted transition-colors">
                    {value.startsWith("http") ? (
                      <a
                        href={value}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="hover:text-accent transition-colors"
                      >
                        {CONTACT_LABELS[key] || key}
                      </a>
                    ) : (
                      <span title={value}>
                        {CONTACT_LABELS[key] || key}: {value}
                      </span>
                    )}
                  </span>
                ))}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
