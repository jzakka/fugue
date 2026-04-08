export interface CreatorSummary {
  id: string;
  nickname: string;
  avatar_url: string | null;
}

export interface TagInfo {
  id: string;
  name: string;
  slug: string;
  category: string;
}

export interface Pin {
  id: string;
  url: string | null;
  title: string;
  description: string | null;
  media_url: string;
  media_type: "image" | "audio" | "video";
  og_image: string | null;
  og_data: Record<string, unknown> | null;
  tags: TagInfo[];
  created_at: string;
  creator: CreatorSummary;
}

export interface ListPinsResponse {
  pins: Pin[];
  has_more: boolean;
}

export interface CreatorPublic {
  id: string;
  nickname: string;
  avatar_url: string | null;
  pin_count: number;
  created_at: string;
}

export interface CreatorPrivate extends CreatorPublic {
  email: string | null;
}

export interface Board {
  id: string;
  creator_id: string;
  name: string;
  description: string | null;
  is_public: boolean;
  pin_count: number;
  cover_images: string[];
  created_at: string;
  updated_at: string;
}

export interface OgPreview {
  title: string;
  description: string;
  image: string;
  site_name: string;
  url: string;
  detected_field: string;
  suggested_tags: string[];
  error?: string;
}

export interface FeedResponse {
  pins: Pin[];
  next_cursor: string | null;
}

export interface PopularTag {
  id: string;
  name: string;
  slug: string;
  category: string;
  pin_count: number;
}

export interface TagListResponse {
  tags: TagInfo[];
}

const INTERNAL_API_URL = process.env.API_URL || "http://localhost:8080";

export async function fetchPins(
  params: {
    media_type?: string;
    tag_ids?: string[];
    limit?: number;
    offset?: number;
    creator_id?: string;
  },
  options?: { serverSide?: boolean }
): Promise<ListPinsResponse> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  if (params.media_type) searchParams.set("media_type", params.media_type);
  if (params.tag_ids?.length) searchParams.set("tag_ids", params.tag_ids.join(","));
  if (params.limit) searchParams.set("limit", String(params.limit));
  if (params.offset) searchParams.set("offset", String(params.offset));
  if (params.creator_id) searchParams.set("creator_id", params.creator_id);

  const res = await fetch(`${baseUrl}/api/pins?${searchParams.toString()}`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function fetchPin(
  id: string,
  options?: { serverSide?: boolean }
): Promise<Pin> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const res = await fetch(`${baseUrl}/api/pins/${id}`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function fetchRelatedPins(
  id: string,
  options?: { serverSide?: boolean }
): Promise<{ pins: Pin[] }> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const res = await fetch(`${baseUrl}/api/pins/${id}/related`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function fetchTags(
  params?: { category?: string; q?: string },
  options?: { serverSide?: boolean; signal?: AbortSignal }
): Promise<TagListResponse> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  if (params?.category) searchParams.set("category", params.category);
  if (params?.q) searchParams.set("q", params.q);

  const res = await fetch(`${baseUrl}/api/tags?${searchParams.toString()}`, {
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function fetchPopularTags(
  params?: { limit?: number },
  options?: { serverSide?: boolean }
): Promise<{ tags: PopularTag[] }> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set("limit", String(params.limit));

  const res = await fetch(`${baseUrl}/api/tags/popular?${searchParams.toString()}`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
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
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function updateMe(
  data: { nickname?: string; avatar_url?: string }
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

export async function fetchOgPreview(url: string): Promise<OgPreview> {
  const res = await fetch(`/api/og/fetch`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function createPin(formData: FormData): Promise<{ id: string }> {
  const res = await fetch(`/api/pins`, {
    method: "POST",
    body: formData,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "요청 실패" }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

export async function deletePin(id: string): Promise<void> {
  const res = await fetch(`/api/pins/${id}`, { method: "DELETE" });
  if (!res.ok && res.status !== 204) throw new Error(`API error: ${res.status}`);
}

export async function fetchFeed(
  params: { limit?: number; cursor?: string },
  options?: { serverSide?: boolean; cookie?: string }
): Promise<FeedResponse> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  if (params.limit) searchParams.set("limit", String(params.limit));
  if (params.cursor) searchParams.set("cursor", params.cursor);

  const headers: Record<string, string> = {};
  if (options?.cookie) headers["Cookie"] = options.cookie;

  const res = await fetch(`${baseUrl}/api/feed?${searchParams.toString()}`, { headers });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function fetchBoards(
  creatorId: string,
  options?: { serverSide?: boolean; cookie?: string }
): Promise<Board[]> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const headers: Record<string, string> = {};
  if (options?.cookie) headers["Cookie"] = options.cookie;

  const res = await fetch(`${baseUrl}/api/boards?creator_id=${creatorId}`, { headers });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  const data = await res.json();
  return data.boards;
}

export async function fetchBoard(
  id: string,
  options?: { serverSide?: boolean; limit?: number; offset?: number }
): Promise<{ board: Board; pins: Pin[]; has_more: boolean }> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const params = new URLSearchParams();
  if (options?.limit) params.set("limit", String(options.limit));
  if (options?.offset) params.set("offset", String(options.offset));
  const qs = params.toString();

  const res = await fetch(`${baseUrl}/api/boards/${id}${qs ? `?${qs}` : ""}`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function createBoard(data: {
  name: string;
  description?: string;
  is_public?: boolean;
}): Promise<Board> {
  const res = await fetch(`/api/boards`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "요청 실패" }));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

export async function updateBoard(
  id: string,
  data: { name?: string; description?: string; is_public?: boolean }
): Promise<Board> {
  const res = await fetch(`/api/boards/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function deleteBoard(id: string): Promise<void> {
  const res = await fetch(`/api/boards/${id}`, { method: "DELETE" });
  if (!res.ok && res.status !== 204) throw new Error(`API error: ${res.status}`);
}

export async function addPinToBoard(boardId: string, pinId: string): Promise<void> {
  const res = await fetch(`/api/boards/${boardId}/pins`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pin_id: pinId }),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

export async function removePinFromBoard(boardId: string, pinId: string): Promise<void> {
  const res = await fetch(`/api/boards/${boardId}/pins/${pinId}`, { method: "DELETE" });
  if (!res.ok && res.status !== 204) throw new Error(`API error: ${res.status}`);
}

export interface PinBoardInfo {
  id: string;
  name: string;
  creator_id: string;
  creator_nickname: string;
}

export async function fetchPinBoards(
  pinId: string,
  options?: { serverSide?: boolean }
): Promise<PinBoardInfo[]> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const res = await fetch(`${baseUrl}/api/pins/${pinId}/boards`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  const data = await res.json();
  return data.boards;
}

export async function recordInteraction(pinId: string, type: "view" | "pin" | "board_add"): Promise<void> {
  fetch(`/api/interactions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pin_id: pinId, type }),
  }).catch(() => {}); // fire-and-forget
}

// --- Search ---

export interface SearchPinResult {
  id: string;
  title: string;
  media_url: string;
  media_type: "image" | "audio" | "video";
  url: string | null;
  description: string | null;
  og_image: string | null;
  creator_id: string;
  creator_nickname: string;
  creator_avatar_url: string | null;
  created_at: string;
}

export interface SearchCreatorResult {
  id: string;
  nickname: string;
  avatar_url: string | null;
  created_at: string;
}

export interface SearchBoardResult {
  id: string;
  name: string;
  description: string | null;
  creator_id: string;
  creator_nickname: string;
  created_at: string;
}

export interface SearchTopTag {
  id: string;
  name: string;
  slug: string;
  category: string;
  count: number;
}

export interface SearchResult {
  pins?: SearchPinResult[];
  creators?: SearchCreatorResult[];
  boards?: SearchBoardResult[];
  top_tags?: SearchTopTag[];
  has_more?: boolean;
}

export async function fetchSearch(
  params: {
    q: string;
    type?: "all" | "pins" | "creators" | "boards";
    tag_ids?: string[];
    limit?: number;
    offset?: number;
  },
  options?: { serverSide?: boolean }
): Promise<SearchResult> {
  const baseUrl = options?.serverSide
    ? INTERNAL_API_URL
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  searchParams.set("q", params.q);
  if (params.type) searchParams.set("type", params.type);
  if (params.tag_ids?.length) searchParams.set("tag_ids", params.tag_ids.join(","));
  if (params.limit) searchParams.set("limit", String(params.limit));
  if (params.offset) searchParams.set("offset", String(params.offset));

  const res = await fetch(`${baseUrl}/api/search?${searchParams.toString()}`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}
