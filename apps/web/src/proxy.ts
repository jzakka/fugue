import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PUBLIC_PATH_PREFIXES = ["/login", "/health", "/api"];
const PUBLIC_EXACT_PATHS = ["/"];

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Allow public paths and static assets
  if (
    PUBLIC_EXACT_PATHS.includes(pathname) ||
    PUBLIC_PATH_PREFIXES.some((p) => pathname.startsWith(p)) ||
    pathname.startsWith("/_next") ||
    pathname.includes(".")
  ) {
    // Don't redirect /login users — even if they have a cookie, it might
    // be invalid. Let the login page handle the redirect if auth is valid.
    return NextResponse.next();
  }

  // Check for auth cookie (existence only, not validity)
  const token = request.cookies.get("fugue_access");
  if (!token) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|.*\\.png$).*)"],
};
