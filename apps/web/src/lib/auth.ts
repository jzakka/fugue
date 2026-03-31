import { cookies } from "next/headers";

export interface AuthUser {
  id: string;
  nickname: string;
  avatar_url: string;
  email: string;
}

const INTERNAL_API_URL = process.env.API_URL || "http://localhost:8080";

async function tryRefresh(refreshToken: string): Promise<string | null> {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/api/auth/refresh`, {
      method: "POST",
      headers: { Cookie: `fugue_refresh=${refreshToken}` },
      cache: "no-store",
    });
    if (!res.ok) return null;
    // The refresh endpoint sets new cookies via Set-Cookie headers.
    // Extract the new access token from the response cookies.
    const setCookie = res.headers.get("set-cookie");
    if (!setCookie) return null;
    const match = setCookie.match(/fugue_access=([^;]+)/);
    return match ? match[1] : null;
  } catch {
    return null;
  }
}

export async function getAuthUser(): Promise<AuthUser | null> {
  const cookieStore = await cookies();
  const token = cookieStore.get("fugue_access")?.value;
  const refreshToken = cookieStore.get("fugue_refresh")?.value;

  if (!token && !refreshToken) return null;

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);

  try {
    // Try with access token first
    if (token) {
      const res = await fetch(`${INTERNAL_API_URL}/api/auth/me`, {
        headers: { Cookie: `fugue_access=${token}` },
        signal: controller.signal,
        cache: "no-store",
      });
      if (res.ok) return res.json();
    }

    // Access token expired or missing, try refresh
    if (refreshToken) {
      const newToken = await tryRefresh(refreshToken);
      if (newToken) {
        const res = await fetch(`${INTERNAL_API_URL}/api/auth/me`, {
          headers: { Cookie: `fugue_access=${newToken}` },
          cache: "no-store",
        });
        if (res.ok) return res.json();
      }
    }

    return null;
  } catch {
    return null;
  } finally {
    clearTimeout(timeout);
  }
}
