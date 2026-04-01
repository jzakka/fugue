import { cookies } from "next/headers";

export interface AuthUser {
  id: string;
  nickname: string;
  avatar_url: string;
  email: string;
}

const INTERNAL_API_URL = process.env.API_URL || "http://localhost:8080";

// SSR auth check — access token only. If expired, returns null and the
// page renders the logged-out state. Token refresh is handled client-side
// via /api/auth/refresh route (see apps/web/src/app/api/auth/refresh/route.ts),
// which properly forwards Set-Cookie headers to the browser. This avoids
// the SSR cookie forwarding problem where server-side refresh consumes
// the new token without updating the browser's cookie jar.
export async function getAuthUser(): Promise<AuthUser | null> {
  const cookieStore = await cookies();
  const token = cookieStore.get("fugue_access")?.value;
  if (!token) return null;

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);

  try {
    const res = await fetch(`${INTERNAL_API_URL}/api/auth/me`, {
      headers: { Cookie: `fugue_access=${token}` },
      signal: controller.signal,
      cache: "no-store",
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  } finally {
    clearTimeout(timeout);
  }
}
