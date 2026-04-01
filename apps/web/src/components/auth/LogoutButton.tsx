"use client";

import { useCallback, useState } from "react";

export default function LogoutButton() {
  const [loading, setLoading] = useState(false);

  const [error, setError] = useState(false);

  const handleLogout = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await fetch("/api/auth/logout", { method: "POST" });
      if (!res.ok) throw new Error("logout failed");
      window.location.href = "/login";
    } catch {
      setLoading(false);
      setError(true);
    }
  }, []);

  return (
    <button
      onClick={handleLogout}
      disabled={loading}
      className="text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
    >
      {loading ? "..." : error ? "재시도" : "로그아웃"}
    </button>
  );
}
