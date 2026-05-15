## Why

DESIGN.md L14는 마스코트의 활용 범위를 명시한다: `마스코트: 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu). 로고/빈 상태/온보딩에서 활용.`

현재 빈 상태 6곳은 시각 요소·메시지 사이즈가 모두 다르다:

| 위치 | 시각 요소 | 메시지 패턴 |
|---|---|---|
| `EmptyState.tsx:9-21` (피드) | 🐡 | sm 메시지 + 액션 |
| `SearchClient.tsx:407-415` (검색) | 🐡 | sm 메시지 + xs 부연 |
| `PinsGrid.tsx:121-124` (프로필 작품) | (없음) | lg 메시지만 |
| `MyPageClient.tsx:123-126` (프로필 보드) | (없음) | sm 메시지 |
| `AddToBoardButton.tsx:251-254` (모달 보드) | (없음) | sm 메시지 |
| `boards/[id]/page.tsx:94-119` (보드 상세) | grid SVG icon | sm 메시지 + xs 부연 |

2곳만 🐡 마스코트를 시도하고 3곳은 텍스트만, 1곳은 마스코트 자리에 grid SVG를 둔다. 메시지 사이즈도 sm/lg 혼재.

`EmptyState.tsx`는 현재 useRouter를 직접 호출해 "전체 보기" 액션을 hardcode한다. 이를 props 기반으로 일반화하면 6곳에서 재사용해 마스코트·여백·타이포 위계를 한 곳에서 통제할 수 있다.

## What Changes

1. `apps/web/src/components/feed/EmptyState.tsx`를 props 기반 공통 컴포넌트로 재설계:
   - `message: string` — 주 메시지 (text-sm + text-text-muted)
   - `description?: string` — 부연 (text-xs + text-text-dim)
   - `children?: ReactNode` — 옵션 액션 슬롯 (mt-4)
   - useRouter 제거. 액션은 호출자가 children으로 주입.
   - 마스코트: `🐡` 이모지 5xl + mb-4. 컨테이너 `py-16 text-center`.
2. 6곳 빈 상태 마크업을 `<EmptyState>` 사용으로 교체:
   - `FeedContainer.tsx:163` — useRouter는 FeedContainer 본문에서 호출하고 `<button>`을 EmptyState children으로 전달.
   - `SearchClient.tsx:407-415` — 인라인 마크업 → `<EmptyState>`.
   - `PinsGrid.tsx:121-124` — 인라인 → `<EmptyState>`.
   - `MyPageClient.tsx:123-126` — 인라인 → `<EmptyState>`.
   - `AddToBoardButton.tsx:251-254` — 인라인 → `<EmptyState>`.
   - `boards/[id]/page.tsx:94-119` — grid SVG 블록 제거, `<EmptyState>` 사용. server component에서 props만 넘기므로 children 없이 호출 가능.

설계 결정:
- 마스코트 자산: DESIGN.md L14가 명시하는 "헤드셋·붓 복어"의 전용 SVG/일러스트가 아직 없으므로 1차로 🐡 이모지로 통일. 일러스트 자산 도입은 별도 후보로 분리(`design-2026XXXX-mascot-illustration-asset`).
- 액션 슬롯: 호출자가 children으로 자유롭게 주입하면 server/client 경계 문제 없음. EmptyState 자체는 액션 비결정.
- 메시지 사이즈: text-sm + text-text-muted로 통일. PinsGrid의 기존 `text-lg`는 동등한 빈 상태에서 다른 화면과 위계가 안 맞으므로 sm으로 정렬.
- 컨테이너 padding: `py-16` (4xl). 기존 EmptyState는 py-20, SearchClient/PinsGrid는 py-16/py-20 혼재. py-16으로 통일.

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: EmptyState.tsx / FeedContainer.tsx / SearchClient.tsx / PinsGrid.tsx / MyPageClient.tsx / AddToBoardButton.tsx / boards/[id]/page.tsx 7 파일.
- 사용자 영향: 빈 상태 6곳 모두 동일한 마스코트(🐡 5xl) + sm 메시지 + 옵션 xs 부연 + 옵션 액션 구조. 화면 간 일관성 확보.
- 레이아웃 시프트: 없음(빈 상태는 라우트별 단일 영역).
- 성능: 영향 없음.
- 의존성·인프라·DB 마이그레이션 없음.
- 테스트 영향: `apps/web/src/components/feed/__tests__/FeedContainer.test.tsx:87`는 "이 분야의 작품이 아직 없어요" 텍스트를 그대로 검증 — 메시지 텍스트가 동일하므로 통과.

## Rollback

- 7 파일의 변경 라인을 git revert하면 즉시 이전 상태로 복귀.
