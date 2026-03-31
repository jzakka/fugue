"use client";

import { useCallback, useState } from "react";

export default function LogoutButton() {
  const [loading, setLoading] = useState(false);

  const handleLogout = useCallback(async () => {
    setLoading(true);
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      // best effort
    }
    window.location.href = "/login";
  }, []);

  return (
    <button
      onClick={handleLogout}
      disabled={loading}
      className="text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
    >
      {loading ? "..." : "로그아웃"}
    </button>
  );
}
