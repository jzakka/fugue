export interface CreatorSummary {
  id: string;
  nickname: string;
  avatar_url: string | null;
}

export interface Work {
  id: string;
  url: string;
  title: string;
  description: string | null;
  field: string;
  tags: string[];
  og_image: string | null;
  og_data: Record<string, unknown> | null;
  created_at: string;
  creator: CreatorSummary;
}

export interface ListWorksResponse {
  works: Work[];
  has_more: boolean;
}

export interface CreatorPublic {
  id: string;
  nickname: string;
  bio: string | null;
  roles: string[];
  contacts: Record<string, string>;
  avatar_url: string | null;
  work_count: number;
  created_at: string;
}

export interface CreatorPrivate extends CreatorPublic {
  email: string | null;
}

const INTERNAL_API_URL = process.env.API_URL || "http://localhost:8080";

export async function fetchWorks(
  params: {
    field?: string;
    tags?: string[];
    limit?: number;
    offset?: number;
    creator_id?: string;
  },
  options?: { serverSide?: boolean }
): Promise<ListWorksResponse> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  if (params.field) searchParams.set("field", params.field);
  if (params.tags?.length) searchParams.set("tags", params.tags.join(","));
  if (params.limit) searchParams.set("limit", String(params.limit));
  if (params.offset) searchParams.set("offset", String(params.offset));
  if (params.creator_id) searchParams.set("creator_id", params.creator_id);

  const res = await fetch(`${baseUrl}/api/works?${searchParams.toString()}`);

  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }

  return res.json();
}

export async function fetchCreator(
  id: string,
  options?: { serverSide?: boolean }
): Promise<CreatorPublic> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const res = await fetch(`${baseUrl}/api/creators/${id}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export async function updateMe(
  data: {
    nickname?: string;
    bio?: string;
    roles?: string[];
    contacts?: Record<string, string>;
    avatar_url?: string;
  }
): Promise<CreatorPrivate> {
  const res = await fetch(`/api/creators/me`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "요청 실패" }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}
