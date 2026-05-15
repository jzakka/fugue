# FeedContainer 에러 박스를 표준 alert 패턴으로 정렬

## Backlog id

`design-20260515-feedcontainer-error-pattern-outlier`

## 무엇을

`apps/web/src/components/feed/FeedContainer.tsx:185` 에러 표시 div의 className을 사이트 표준 alert 패턴으로 교체.

- **Before:** `mb-4 p-4 bg-surface rounded-md border-l-3 border-error text-sm`
- **After:** `mb-4 p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error`

자식 요소(에러 메시지 텍스트 + "다시 시도" 버튼) 마크업/로직은 손대지 않는다.

## 왜

### DESIGN.md 근거

- **L50 — Semantic 토큰**: `error #FF3B30`이 토큰으로 정의되어 있고, `--color-error`가 `@theme inline`에 등록되어 `bg-error/{N}`, `border-error/{N}`, `text-error` 유틸리티가 이미 사용 가능.
- **L74 — Radius 스케일**: `sm: 6px (inputs, alerts)` — alert는 6px radius로 명시. Tailwind v4 `rounded-md`는 기본값으로 6px를 렌더하지만 코드베이스는 `rounded-[6px]`로 22곳에서 일관 표기, `rounded-md`는 단 2곳(FeedContainer 에러 + NavBar 로고)에만 잔존.

### 패턴 일관성 근거

코드베이스의 5개 에러 박스가 모두 다음 표준 패턴을 사용 중:

| 파일 | 패턴 |
|------|------|
| `PinCreateForm.tsx:319` | `p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error` |
| `BoardActions.tsx:65` | `p-2 bg-error/10 border border-error/30 rounded-[6px] text-xs text-error` |
| `MyPageClient.tsx:81` | `p-2 bg-error/10 border border-error/30 rounded-[6px] text-xs text-error` |
| `ProfileEditForm.tsx:55` | `p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error` |
| `AddToBoardButton.tsx:237` | `bg-error/10 border border-error/30 text-error` (parent에서 padding/radius 부여) |

FeedContainer만 외톨이:
- `bg-surface` → 에러 톤(`bg-error/10`) 없음. 시각적으로 에러로 인지되지 않음.
- `border-l-3 border-error` → `border-l-3`은 Tailwind v4 기본 스케일(`border-l-2`, `border-l-4`만 정의)에 없는 **무효 유틸리티**. 좌측 보더가 실제로는 렌더링되지 않음.
- `rounded-md` → 코드베이스 표기 컨벤션(`rounded-[6px]`)과 어긋남. 렌더 픽셀은 같지만 표기 일관성 깨짐.
- 메시지 텍스트에 `text-error` 미적용 → default `text-primary`로 렌더되어 색 신호 없음.

## 어디까지

### 변경 파일

- `apps/web/src/components/feed/FeedContainer.tsx` — className 1줄(L185)만 교체.

### 사용자 영향

피드 페이지(`/`)에서 페이지 페이지네이션·태그 필터링·미디어 타입 필터링 등이 실패해 에러가 노출될 때, 그 박스가 다음 5곳의 에러 박스와 동일한 시각적 톤(연한 에러 배경 + 에러 보더 + 에러 컬러 텍스트)으로 렌더된다. 좌측 보더가 더 이상 무효 클래스 의존이 아니라 4면 보더가 정상 표시된다.

### 무엇을 하지 않는가

- 다른 5개 에러 박스는 손대지 않는다. 이미 표준 패턴이다.
- `EmptyState`, `FeedContainer`의 빈 상태(L210 근방) 마크업은 손대지 않는다 — 별개 트랙(빈 상태)에서 이미 처리됨.
- "다시 시도" 버튼 스타일(`text-accent hover:underline`)은 유지. 액션 버튼 컨벤션은 별도 후보.

## 롤백

`apps/web/src/components/feed/FeedContainer.tsx:185`을 `mb-4 p-4 bg-surface rounded-md border-l-3 border-error text-sm`으로 되돌린다. 단일 라인 reverting이라 `git checkout -- apps/web/src/components/feed/FeedContainer.tsx` 또는 commit revert로 즉시 복원.

## Anti-pattern 검토

- **L15** (token 의미 덮어쓰기 vs 추가 분리): 해당 없음. 신규 토큰 추가 아님, `bg-error/border-error/text-error` 모두 기존 정의 토큰 사용.
- **L16** (radius 등급 매핑 모호): 해당 없음. DESIGN.md L74가 "alerts → sm:6px"로 직접 등급을 명시했고 본 요소는 명백한 alert 박스.
