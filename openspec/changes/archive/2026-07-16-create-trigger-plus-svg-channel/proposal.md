# Proposal: create-trigger-plus-svg-channel

## Why

생성 트리거의 add-어포던스 plus 마크 저작 채널이 역할 내에서 갈린다. 생성 트리거 3곳 중 2곳(NavBar "+ 핀 생성"·MyPageClient "+ 새 보드")은 텍스트 글리프 `+`를 라벨 접두로 쓰고, 1곳(AddToBoardButton "새 보드 만들기")만 aria-hidden 인라인 SVG plus를 쓴다.

anti-patterns L1845(cycle 3615) baseline은 기능 UI 아이콘의 지배 채널을 "전수 SVG(aria-hidden·currentColor 균일)"로 판정했고, 동일 역할 아이콘의 역할 내 채널 갈림은 동 baseline의 명시 예외(실제 채널 비정합)다. 부수 a11y 갈림: 텍스트 `+`는 버튼 접근 이름에 포함되어(SR "플러스 핀 생성" 발화) 장식 아이콘 aria-hidden 관례(L515)와 어긋나는 반면 SVG plus는 접근 이름에서 제외된다.

## What Changes

- NavBar "+ 핀 생성" Link의 텍스트 `+` 접두를 aria-hidden 인라인 SVG plus로 교체 (AddToBoardButton:405 idiom: viewBox 24·currentColor·strokeWidth 2)
- MyPageClient "+ 새 보드" 버튼의 텍스트 `+` 접두를 동일 SVG plus로 교체
- 두 표면 모두 `inline-flex items-center gap-*` 병치로 라벨과 정렬. 접근 이름은 각각 "핀 생성"·"새 보드"가 된다.

## Capabilities

### Modified: pin

핀 생성 진입 트리거의 추가 기호가 장식 처리되어 접근 이름에 포함되지 않는 요구사항 추가.

### Modified: board

보드 생성 폼 열기 트리거의 추가 기호가 장식 처리되어 접근 이름에 포함되지 않는 요구사항 추가.

## Impact

- `apps/web/src/components/nav/NavBar.tsx` — CTA Link 1곳 (텍스트 접두 → SVG + flex 정렬 클래스)
- `apps/web/src/components/profile/MyPageClient.tsx` — 새 보드 버튼 1곳 (동일)
- 라우팅·상태·API 변경 없음. AddToBoardButton은 비접촉. MyPageClient 버튼의 aria-expanded/aria-controls 토글 동작 불변.
