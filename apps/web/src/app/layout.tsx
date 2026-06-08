import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Fugue — 창작물 기반 협업 매칭",
  description: "사람이 아닌 작품을 매칭합니다. 이미지, 음악, 글 — 분야를 넘어 작품끼리 만나는 곳.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ko" className="h-full antialiased">
      <head>
        <link
          rel="stylesheet"
          href="https://cdn.jsdelivr.net/npm/geist@1.2.0/dist/fonts/geist-mono/style.css"
        />
        <link
          rel="stylesheet"
          href="https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/dist/web/variable/pretendardvariable-dynamic-subset.min.css"
        />
        <link
          rel="stylesheet"
          href="https://api.fontshare.com/v2/css?f[]=general-sans@500,700&display=swap"
        />
      </head>
      <body className="min-h-full flex flex-col">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[100] focus:px-3 focus:py-2 focus:bg-accent focus:text-white focus:rounded-[6px] focus:text-sm focus:font-semibold"
        >
          메인 콘텐츠로 건너뛰기
        </a>
        {children}
      </body>
    </html>
  );
}
