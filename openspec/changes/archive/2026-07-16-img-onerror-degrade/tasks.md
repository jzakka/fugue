# Tasks: img-onerror-degrade

## 1. 클라이언트 컴포넌트 공용 이미지 신설

- [x] 1.1 `apps/web/src/components/ui/HideOnErrorImage.tsx` 신설 — `"use client"`, `React.ImgHTMLAttributes<HTMLImageElement>` 스프레드 + `alt` 기본값 `""` 명시, onError 시 `e.currentTarget.style.display = "none"` (design.md Decision 2 코드 그대로)

## 2. 서버 컴포넌트 4개 img 교체

- [x] 2.1 `apps/web/src/components/nav/NavBar.tsx:61` user avatar `<img>` → `<HideOnErrorImage>` (src/alt/className 불변)
- [x] 2.2 `apps/web/src/components/profile/ProfileHeader.tsx:18` creator avatar `<img>` → `<HideOnErrorImage>` (속성 불변)
- [x] 2.3 `apps/web/src/app/pins/[id]/page.tsx:43` 이미지 미디어 `<img>` → `<HideOnErrorImage>` (속성 불변)
- [x] 2.4 `apps/web/src/app/pins/[id]/page.tsx:202` creator avatar `<img>` → `<HideOnErrorImage>` (속성 불변)

## 3. 클라이언트 컴포넌트 3곳 인라인 onError 추가

- [x] 3.1 `apps/web/src/app/search/SearchClient.tsx:328` creator avatar에 인라인 onError hide 추가 (design.md Decision 1 코드)
- [x] 3.2 `apps/web/src/components/board/BoardCover.tsx:34` 콜라주 슬롯 img에 인라인 onError hide 추가
- [x] 3.3 `apps/web/src/components/board/AddToBoardButton.tsx:313` 미니커버 img에 인라인 onError hide 추가

## 4. 검증

- [x] 4.1 `cd apps/web && npm run lint && npm run typecheck && npm test -- --run` 통과
- [x] 4.2 실 브라우저 QA (backlog qa_plan): 부재였던 7곳 각각 CDP로 img src를 깨진 URL로 치환 → broken glyph 미노출(숨김) 확인, 정상 렌더 회귀 없음, 기존 보유 5곳 무변경, 콘솔 에러 0
