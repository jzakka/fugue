import ThemeToggle from "@/components/ui/ThemeToggle";
import LogoutButton from "@/components/auth/LogoutButton";
import SearchBar from "@/components/nav/SearchBar";
import { getAuthUser } from "@/lib/auth";
import Link from "next/link";

export default async function NavBar() {
  const user = await getAuthUser();

  return (
    <header>
      <nav className="sticky top-0 z-50 bg-bg border-b border-border px-6 py-4 flex items-center gap-6 backdrop-blur-sm">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 shrink-0">
          <div
            aria-hidden="true"
            className="w-8 h-8 bg-accent rounded-md flex items-center justify-center text-lg"
          >
            🐡
          </div>
          <span className="text-xl font-bold tracking-tight text-text-primary font-display">
            Fugue
          </span>
        </Link>

        {/* Search */}
        <SearchBar />

        {/* Actions */}
        <div className="flex items-center gap-4 ml-auto shrink-0">
          {user ? (
            <>
              <Link
                href="/pin/new"
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors"
              >
                <svg
                  aria-hidden="true"
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <line x1="12" y1="5" x2="12" y2="19" />
                  <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
                핀 생성
              </Link>
              <ThemeToggle />
              <div className="flex items-center gap-3">
                <Link
                  href="/mypage"
                  aria-label={user.nickname}
                  className="flex items-center gap-2 group"
                >
                  {user.avatar_url ? (
                    <img
                      src={user.avatar_url}
                      alt=""
                      className="w-9 h-9 rounded-full border-2 border-border object-cover group-hover:border-accent group-focus-visible:border-accent transition-colors"
                    />
                  ) : (
                    <div className="w-9 h-9 rounded-full bg-gradient-to-br from-accent to-accent-hover border-2 border-border group-hover:border-accent group-focus-visible:border-accent transition-colors" />
                  )}
                  <span className="text-sm text-text-primary hidden sm:block group-hover:text-accent group-focus-visible:text-accent transition-colors">
                    {user.nickname}
                  </span>
                </Link>
                <LogoutButton />
              </div>
            </>
          ) : (
            <>
              <ThemeToggle />
              <Link
                href="/login"
                className="px-4 py-2 border border-border rounded-full text-sm font-medium text-text-primary hover:border-accent focus-visible:border-accent transition-colors"
              >
                로그인
              </Link>
            </>
          )}
        </div>
      </nav>
    </header>
  );
}
