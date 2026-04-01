"use client";

import { useState } from "react";
import type { CreatorPrivate, Work } from "@/lib/api";
import ProfileHeader from "./ProfileHeader";
import ProfileEditForm from "./ProfileEditForm";
import WorksGrid from "./WorksGrid";

export default function MyPageClient({
  creator: initialCreator,
  works,
  hasMore,
}: {
  creator: CreatorPrivate;
  works: Work[];
  hasMore: boolean;
}) {
  const [creator, setCreator] = useState(initialCreator);
  const [editing, setEditing] = useState(false);

  return (
    <div className="space-y-6">
      {editing ? (
        <ProfileEditForm
          creator={creator}
          onSave={(updated) => {
            setCreator(updated);
            setEditing(false);
          }}
          onCancel={() => setEditing(false)}
        />
      ) : (
        <ProfileHeader
          creator={creator}
          isOwner
          onEdit={() => setEditing(true)}
        />
      )}

      <WorksGrid
        creatorId={creator.id}
        initialWorks={works}
        initialHasMore={hasMore}
      />
    </div>
  );
}
