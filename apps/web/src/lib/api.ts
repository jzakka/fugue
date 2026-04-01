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

export async function fetchWorks(
  params: {
    field?: string;
    tags?: string[];
    limit?: number;
    offset?: number;
  },
  options?: { serverSide?: boolean }
): Promise<ListWorksResponse> {
  // SSR: use internal service URL (not exposed to browser)
  // Client: use NEXT_PUBLIC_API_URL (must be set in production deployments)
  const baseUrl = options?.serverSide
    ? process.env.API_URL || "http://localhost:8080"
    : process.env.NEXT_PUBLIC_API_URL || "";

  const searchParams = new URLSearchParams();
  if (params.field) searchParams.set("field", params.field);
  if (params.tags?.length) searchParams.set("tags", params.tags.join(","));
  if (params.limit) searchParams.set("limit", String(params.limit));
  if (params.offset) searchParams.set("offset", String(params.offset));

  const res = await fetch(`${baseUrl}/api/works?${searchParams.toString()}`);

  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }

  return res.json();
}
