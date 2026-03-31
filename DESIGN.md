# Design System — Fugue

## Product Context
- **What this is:** 창작물 기반 협업 매칭 플랫폼. 사람이 아닌 "작품"을 매칭한다.
- **Who it's for:** 인디~프로 크리에이터 (일러스트레이터, 뮤지션, 작가, 영상 크리에이터 등)
- **Space/industry:** Creative portfolio / collaboration (Behance, Dribbble, ArtStation 카테고리)
- **Project type:** Web app (Next.js App Router + Go API)

## Aesthetic Direction
- **Direction:** Industrial/Editorial — "Dark Gallery"
- **Decoration level:** Minimal — 타이포그래피와 여백이 모든 걸 한다
- **Mood:** 갤러리나 콘서트홀. 어두운 벽 위에 작품이 빛나는 공간. UI는 보이지 않는 만큼 심플하게, 작품만 눈에 들어오는 구조.
- **Reference sites:** Behance, Dribbble, Pinterest (masonry layout), SoundCloud (audio card patterns)
- **Mascot:** 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu). 로고/빈 상태/온보딩에서 활용.

## Typography
- **Display/Hero:** General Sans 700 — 기하학적이면서 개성 있음. 한글 대체: Pretendard Bold
- **Body:** Pretendard Variable — 한글+라틴 모두 지원, 가독성 최상
- **UI/Labels:** Pretendard Variable 500
- **Data/Tags:** Geist Mono — 태그, 메타데이터, 수치에 기술적 느낌 (tabular-nums 지원)
- **Code:** Geist Mono
- **Loading:**
  - Pretendard: `https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/dist/web/variable/pretendardvariable-dynamic-subset.min.css`
  - General Sans: Google Fonts or self-hosted
  - Geist Mono: `https://cdn.jsdelivr.net/npm/geist@1.2.0/dist/fonts/geist-mono/style.css`
- **Scale:**
  - 3xl: 42px / 2.625rem (hero)
  - 2xl: 32px / 2rem (page title)
  - xl: 24px / 1.5rem (section title)
  - lg: 18px / 1.125rem (card title large, text excerpt title)
  - md: 15px / 0.9375rem (body)
  - sm: 13px / 0.8125rem (secondary text, card descriptions)
  - xs: 12px / 0.75rem (creator name, meta)
  - 2xs: 11px / 0.6875rem (timestamps, duration)
  - 3xs: 10px / 0.625rem (tags, category labels)

## Color
- **Approach:** Restrained — 액센트는 사용자 액션(CTA, 호버, 선택)에만 써서 작품과 경쟁하지 않는다
- **Primary accent:** #E85A2A (burnt vermillion) — 따뜻하고 창작적. Behance(blue), Dribbble(pink), SoundCloud(orange)와 차별화. 복어 마스코트의 에너지와 어울림.
- **Accent hover:** #FF6B3D
- **Accent subtle:** rgba(232, 90, 42, 0.12) — 태그 배경, 선택 상태
- **Background:** #0C0C0C (dark mode), #F5F5F0 (light mode)
- **Surface:** #161616 / #FFFFFF — 카드 배경
- **Surface elevated:** #1E1E1E / #FAFAFA — 드롭다운, 모달
- **Surface hover:** #242424 / #F0F0F0
- **Border:** #2A2A2A / #E0E0E0
- **Text primary:** #E8E8E8 / #1A1A1A
- **Text muted:** #888888 / #777777
- **Text dim:** #555555 / #AAAAAA
- **Semantic:** success #34C759, warning #FFB800, error #FF3B30, info #5AC8FA
- **Dark mode:** 기본값. 작품은 어두운 배경에서 더 빛난다. 갤러리 벽처럼.
- **Light mode:** 토글로 전환 가능. CSS custom properties로 구현.

## Spacing
- **Base unit:** 4px
- **Density:** Comfortable
- **Scale:**
  - 2xs: 2px
  - xs: 4px
  - sm: 8px
  - md: 16px
  - lg: 24px
  - xl: 32px
  - 2xl: 48px
  - 3xl: 64px

## Layout
- **Approach:** Masonry grid — 핀터레스트식, 미디어 타입별 카드 높이가 자연스럽게 달라진다
- **Grid:** 4 columns (desktop), 3 (tablet), 2 (mobile), 1 (small mobile)
- **Breakpoints:** sm: 500px, md: 800px, lg: 1200px
- **Column gap:** 16px (md spacing)
- **Max content width:** 제한 없음 (masonry가 화면을 채움)
- **Border radius:**
  - sm: 6px (inputs, alerts)
  - md: 10px (cards)
  - lg: 16px (modals, panels)
  - full: 9999px (buttons, chips, avatars, search bar)

## Card System
작품 피드의 핵심. 미디어 타입별 카드 변주:

- **Image card:** og_image 썸네일 + 제목 + 크리에이터 아바타/이름 + 태그. 이미지 종횡비 그대로 유지.
- **Audio card:** 웨이브폼 시각화 + 미묘한 그라디언트 배경 + 재생 버튼 + 트랙 제목/아티스트 + 재생시간. 그라디언트는 accent 계열.
- **Text card:** 장르 라벨(소설, 시, 에세이) + 제목 + 본문 발췌(최대 4줄) + 읽는 시간. 타이포그래피가 비주얼 역할.
- **Video card:** 썸네일 + 재생 아이콘 오버레이 + 길이 표시.
- **Hover state:** translateY(-2px) + box-shadow 확대 + accent border. 150ms ease.

## Motion
- **Approach:** Minimal-functional — 작품 감상을 방해하지 않는 선
- **Easing:** enter(ease-out) exit(ease-in) move(ease-in-out)
- **Duration:** micro(50-100ms) short(150-250ms) medium(250-400ms)
- **Card hover:** translateY(-2px), 200ms ease
- **Page transitions:** 없음 (즉시 로드)
- **Skeleton loading:** 카드 자리에 shimmer 효과

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-01 | Dark Gallery aesthetic | 작품이 주인공. 어두운 배경이 모든 미디어 타입의 작품을 가장 잘 띄운다. |
| 2026-04-01 | Burnt vermillion (#E85A2A) accent | 경쟁 플랫폼과 차별화. Behance(blue), Dribbble(pink), SoundCloud(orange) 모두와 다른 색. |
| 2026-04-01 | 미디어타입별 카드 변주 | 혼합 피드의 핵심 차별점. 이미지/음악/글이 각각 자체적으로 아름다운 카드를 가진다. |
| 2026-04-01 | Pretendard for body | 한글+라틴 동시 지원, 가변 폰트로 성능 우수. 한국어 크리에이터 플랫폼에 최적. |
| 2026-04-01 | Masonry layout (CSS columns) | Pinterest가 증명한 패턴. 다양한 종횡비의 콘텐츠를 자연스럽게 배치. |
| 2026-04-01 | Dark mode as default | 갤러리 벽 효과. 라이트 모드는 토글로 전환 가능. |
