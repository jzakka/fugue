# Proposal: img-onerror-degrade

## Why

외부 URL `<img>` 12표면 중 5곳(PinCard:88/:167/:178·SearchBar:284·ProfileEditForm:105)만 로드 실패 시 `display:none` degrade를 보유하고, 동종 역할의 나머지 7곳은 브라우저 broken-image glyph가 Dark Gallery 표면에 그대로 노출된다. 동일 엔티티(creator avatar)가 PinCard에서는 조용히 숨겨지고 핀 상세·프로필 헤더·검색 결과·NavBar에서는 깨진 아이콘으로 표시되는 역할 내 비정합이다. PinCard:88·SearchBar:284의 코드 코멘트가 스스로 실패 모드를 문서화("harvester image cache TTL eviction → broken-image glyph 방지")했는데, 동일 실패 모드인 BoardCover cover_images·pins/[id] media_url은 미처리 상태다. (backlog: design-20260716-img-onerror-degrade-partial, anti-patterns L91 예외 조항 "일부 `<img>`만 설정해 동종 이미지 간 어휘가 실제로 엇갈리는 격리 site = 등록 가능" 해당)

## What Changes

- 기존 지배 관례(onError → `display:none` hide)를 미보유 7곳에 동일하게 적용한다.
  - 클라이언트 컴포넌트 3곳: `SearchClient.tsx:328`(creator avatar), `BoardCover.tsx:34`(og_image 콜라주), `AddToBoardButton.tsx:313`(cover_images[0] 미니커버) — 기존 idiom과 동일한 인라인 onError 핸들러 추가.
  - 서버 컴포넌트 내 4개 img: `NavBar.tsx:61`(user avatar), `ProfileHeader.tsx:18`(creator avatar), `pins/[id]/page.tsx:43`(핀 미디어)·`:202`(creator avatar) — onError는 클라이언트 이벤트이므로 소형 클라이언트 컴포넌트 `HideOnErrorImage`(`components/ui/`)를 신설해 대체.
- 정상 로드 시 렌더 결과(클래스·속성·레이아웃)는 전부 불변. 실패 시에만 glyph 대신 숨김.
- 기존 보유 5곳은 손대지 않는다.

## Capabilities

### Modified Capabilities

- `profile`: 유저/크리에이터 아바타 이미지 로드 실패 시 깨진 이미지 표시를 노출하지 않고 숨긴다 (NavBar·ProfileHeader·검색 결과 크리에이터 카드·핀 상세 크리에이터 영역).
- `pin`: 핀 상세 이미지 미디어 로드 실패 시 깨진 이미지 표시를 노출하지 않고 숨긴다.
- `board`: 보드 커버 콜라주·미니커버 이미지 로드 실패 시 깨진 이미지 표시를 노출하지 않고 해당 슬롯 배경으로 degrade한다.

## Impact

- 변경 파일: `apps/web/src/components/nav/NavBar.tsx`, `apps/web/src/components/profile/ProfileHeader.tsx`, `apps/web/src/app/pins/[id]/page.tsx`, `apps/web/src/app/search/SearchClient.tsx`, `apps/web/src/components/board/BoardCover.tsx`, `apps/web/src/components/board/AddToBoardButton.tsx`, 신규 `apps/web/src/components/ui/HideOnErrorImage.tsx`
- 사용자 영향: 만료·삭제된 외부 URL(avatar_url/og_image/media_url)에서 깨진 이미지 아이콘이 사라짐. 정상 URL 렌더는 무변경.
- 롤백: 커밋 revert 1건 (신규 파일 1 + 기존 6파일 국소 수정).
