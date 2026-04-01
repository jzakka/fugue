import { cookies } from "next/headers";

export interface AuthUser {
  id: string;
  nickname: string;
  avatar_url: string;
  email: string;
}

const INTERNAL_API_URL = process.env.API_URL || "http://localhost:8080";

export async function getAuthUser(): Promise<AuthUser | null> {
  const cookieStore = await cookies();
  const token = cookieStore.get("fugue_access")?.value;
  const refreshToken = cookieStore.get("fugue_refresh")?.value;

  if (!token && !refreshToken) return null;

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);

  try {
    // Try with access token
    if (token) {
      const res = await fetch(`${INTERNAL_API_URL}/api/auth/me`, {
        headers: { Cookie: `fugue_access=${token}` },
        signal: controller.signal,
        cache: "no-store",
      });
      if (res.ok) return res.json();
    }

    // Access token missing/expired — try refresh via Go API directly
    // (SSR can forward the refresh cookie since we read it from the cookie store)
    if (refreshToken) {
      const refreshRes = await fetch(`${INTERNAL_API_URL}/api/auth/refresh`, {
        method: "POST",
        headers: { Cookie: `fugue_refresh=${refreshToken}` },
        cache: "no-store",
      });
      if (refreshRes.ok) {
        // Extract new access token from Set-Cookie
        const setCookie = refreshRes.headers.get("set-cookie") || "";
        const match = setCookie.match(/fugue_access=([^;]+)/);
        if (match) {
          const meRes = await fetch(`${INTERNAL_API_URL}/api/auth/me`, {
            headers: { Cookie: `fugue_access=${match[1]}` },
            cache: "no-store",
          });
          if (meRes.ok) return meRes.json();
        }
      }
    }

    return null;
  } catch {
    return null;
  } finally {
    clearTimeout(timeout);
  }
}
