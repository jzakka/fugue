import ThemeToggle from "@/components/ui/ThemeToggle";
import LogoutButton from "@/components/auth/LogoutButton";
import { getAuthUser } from "@/lib/auth";
import Link from "next/link";

export default async function NavBar() {
  const user = await getAuthUser();

  return (
    <header>
      <nav className="sticky top-0 z-50 bg-bg border-b border-border px-6 py-4 flex items-center gap-6 backdrop-blur-sm">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 shrink-0">
          <div className="w-8 h-8 bg-accent rounded-md flex items-center justify-center text-lg">
            🐡
          </div>
          <span className="text-xl font-bold tracking-tight text-text-primary">
            Fugue
          </span>
        </Link>

        {/* Search */}
        <div className="flex-1 max-w-md relative">
          <span className="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm opacity-40">
            🔍
          </span>
          <input
            type="text"
            placeholder="작품, 크리에이터, 태그 검색..."
            className="w-full py-2.5 pl-10 pr-4 bg-surface border border-border rounded-full text-sm text-text-primary placeholder:text-text-dim outline-none focus:border-accent transition-colors"
            disabled
          />
        </div>

        {/* Actions */}
        <div className="flex items-center gap-4 ml-auto shrink-0">
          {user ? (
            <>
              <button className="px-4 py-2 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover transition-colors cursor-pointer">
                + 작품 올리기
              </button>
              <ThemeToggle />
              <div className="flex items-center gap-3">
                <Link href="/mypage" className="flex items-center gap-2">
                  {user.avatar_url ? (
                    <img
                      src={user.avatar_url}
                      alt={user.nickname}
                      className="w-9 h-9 rounded-full border-2 border-border object-cover"
                    />
                  ) : (
                    <div className="w-9 h-9 rounded-full bg-gradient-to-br from-accent to-orange-400 border-2 border-border" />
                  )}
                  <span className="text-sm text-text-primary hidden sm:block">
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
                className="px-4 py-2 border border-border rounded-full text-sm font-medium text-text-primary hover:border-accent transition-colors"
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
