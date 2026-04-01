import { NextRequest, NextResponse } from "next/server";

const API_URL = process.env.API_URL || "http://localhost:8080";

// Proxy refresh requests to the Go API, forwarding all cookies.
// This solves the path-scoped cookie problem: fugue_refresh has Path=/api/auth
// and is not visible to SSR page requests, but IS sent to /api/auth/* routes.
// The browser calls this Next.js route, which forwards to the Go API.
export async function POST(request: NextRequest) {
  const cookieHeader = request.headers.get("cookie") || "";

  try {
    const res = await fetch(`${API_URL}/api/auth/refresh`, {
      method: "POST",
      headers: { Cookie: cookieHeader },
      cache: "no-store",
    });

    // Forward the response (including Set-Cookie headers for new tokens)
    const body = await res.text();
    const response = new NextResponse(body, { status: res.status });

    // Forward Set-Cookie headers from Go API
    const setCookies = res.headers.getSetCookie();
    for (const cookie of setCookies) {
      response.headers.append("Set-Cookie", cookie);
    }

    return response;
  } catch {
    return NextResponse.json({ error: "refresh failed" }, { status: 502 });
  }
}
