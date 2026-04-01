import { redirect } from "next/navigation";
import { getAuthUser } from "@/lib/auth";
import LoginButtons from "./LoginButtons";

const ERROR_MESSAGES: Record<string, string> = {
  unknown_provider: "알 수 없는 로그인 방법입니다",
  invalid_state: "세션이 만료되었습니다. 다시 시도해 주세요",
  exchange_failed: "인증에 실패했습니다. 다시 시도해 주세요",
  profile_failed: "프로필 정보를 가져올 수 없습니다",
  account_failed: "계정 처리 중 오류가 발생했습니다",
  token_failed: "로그인 처리 중 오류가 발생했습니다",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string; redirect?: string }>;
}) {
  // Validate the token, not just cookie existence — a corrupted or expired
  // cookie should not bounce users away from /login
  const user = await getAuthUser();
  if (user) {
    redirect("/");
  }

  const params = await searchParams;
  const errorCode = params.error;
  const errorMessage = errorCode
    ? ERROR_MESSAGES[errorCode] || "로그인에 실패했습니다. 다시 시도해 주세요"
    : null;
  const redirectTo = params.redirect || "";

  return (
    <div className="min-h-screen bg-bg flex items-center justify-center px-4">
      <main className="w-full max-w-[400px] bg-surface rounded-2xl p-8 border border-border">
        {/* Logo */}
        <div className="flex flex-col items-center mb-8">
          <div className="w-16 h-16 bg-accent rounded-xl flex items-center justify-center text-3xl mb-4">
            🐡
          </div>
          <h1 className="text-2xl font-bold text-text-primary tracking-tight">
            작품으로 만나다
          </h1>
          <p className="text-sm text-text-muted mt-2">
            소셜 계정으로 시작하세요
          </p>
        </div>

        {/* OAuth Buttons */}
        <LoginButtons redirectTo={redirectTo} />

        {/* Error Message */}
        {errorMessage && (
          <p role="alert" aria-live="polite" className="mt-4 text-center text-sm text-error">
            {errorMessage}
          </p>
        )}
      </main>
    </div>
  );
}
