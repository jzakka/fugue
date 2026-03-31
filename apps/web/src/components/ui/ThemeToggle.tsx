"use client";

import { useEffect, useState } from "react";

export default function ThemeToggle() {
  const [isLight, setIsLight] = useState(false);

  useEffect(() => {
    const saved = localStorage.getItem("fugue-theme");
    if (saved === "light") {
      document.documentElement.classList.add("light");
      setIsLight(true);
    }
  }, []);

  function toggle() {
    const next = !isLight;
    setIsLight(next);
    if (next) {
      document.documentElement.classList.add("light");
      localStorage.setItem("fugue-theme", "light");
    } else {
      document.documentElement.classList.remove("light");
      localStorage.setItem("fugue-theme", "dark");
    }
  }

  return (
    <button
      onClick={toggle}
      className="w-9 h-9 rounded-full bg-surface border border-border text-text-muted hover:border-accent hover:text-accent transition-colors flex items-center justify-center text-base cursor-pointer"
      aria-label={isLight ? "다크 모드로 전환" : "라이트 모드로 전환"}
    >
      {isLight ? "☀️" : "🌙"}
    </button>
  );
}
