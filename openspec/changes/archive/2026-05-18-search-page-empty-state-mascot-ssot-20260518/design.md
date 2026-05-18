## Context

`apps/web/src/components/feed/EmptyState.tsx`는 archive/2026-05-15-unify-empty-state-with-mascot에서 도입된 빈 상태 SSoT 컴포넌트. 외곽 `flex flex-col items-center justify-center py-16 text-center` + 마스코트(🐡 `text-5xl mb-4`) + 메시지(`text-text-muted text-sm mb-1`) + 부연(`text-text-dim text-xs`) + 액션 슬롯(`mt-4`)로 구성. 그 사이클에서 6곳 인라인 빈 상태를 EmptyState로 교체했음 (FeedContainer · SearchClient 결과 없음 · PinsGrid · MyPageClient · AddToBoardButton · boards/[id]/page).

`apps/web/src/app/search/page.tsx` L72-80은 검색 페이지 진입 시 `q.length === 0` 분기에서 노출되는 빈 상태인데, archive 정렬 당시 누락되어 인라인 마크업으로 EmptyState 구조를 복제 중이다. 인라인 마크업은 EmptyState 기본 구조와 거의 동일하지만 vertical padding만 `py-20`(80px)으로 어긋나 있다. `text-5xl` grep으로 EmptyState.tsx 외 유일하게 매칭되는 사용처가 이 1곳이다.

## Goals / Non-Goals

**Goals:**

- `apps/web/src/app/search/page.tsx`의 검색어 빈 상태가 EmptyState SSoT 컴포넌트를 경유하도록 정렬한다.
- DESIGN.md L14 '마스코트: 빈 상태/온보딩 활용' 명시를 코드 SSoT(EmptyState 컴포넌트)에 일관 반영한다.
- 다른 6곳 빈 상태와 vertical padding(`py-16`) · 마스코트 크기(`text-5xl`) · 메시지/부연 위계를 통일한다.

**Non-Goals:**

- EmptyState 컴포넌트 API 확장(size variant · padding variant 등)은 본 변경 범위 밖이다 (anti-pattern L18에서 보류).
- 페이지 본문 외 컨텍스트(드롭다운·툴팁·인라인 알림)의 빈 상태 처리는 본 변경 범위 밖.
- `q.length > 0` 분기(SearchClient 내부 결과 없음 빈 상태)는 이미 archive/2026-05-15-unify-empty-state-with-mascot에서 정렬되었으므로 본 변경 대상 아님.

## Decisions

### Decision 1: EmptyState 컴포넌트 호출로 교체

`apps/web/src/app/search/page.tsx` L72-80 인라인 9라인을 다음으로 교체:

```tsx
<EmptyState
  message="검색어를 입력해주세요"
  description="작품, 크리에이터, 보드를 검색할 수 있습니다"
/>
```

또한 import 섹션(L1-6)에 `import EmptyState from "@/components/feed/EmptyState";` 1줄 추가.

**이유**: EmptyState API는 `{ message, description?, children? }`로 본 페이지가 필요한 메시지·부연 텍스트를 그대로 수용. 마스코트(🐡) · 컨테이너 정렬(flex flex-col items-center justify-center text-center) · 위계 클래스(text-sm text-text-muted / text-xs text-text-dim) 모두 EmptyState 내부에서 동일하게 처리.

**대안 검토**:
- 인라인 마크업 유지 + `py-20` → `py-16` 직접 교체만 적용: SSoT 우회 상태는 그대로 남아 6곳 + 1곳 분기가 지속됨. anti-pattern 누적 위험.
- EmptyState에 `padding` prop 도입: API 확장은 본 변경 범위를 벗어남 + 다른 6곳 사용처 회귀 위험.

### Decision 2: 메시지/부연 텍스트는 기존 문구 그대로 유지

`"검색어를 입력해주세요"` / `"작품, 크리에이터, 보드를 검색할 수 있습니다"` 두 문자열은 본 변경에서 수정하지 않는다.

**이유**: 본 변경은 SSoT 컴포넌트 경유 정렬이며 문구 결정은 다른 트랙(콘텐츠/카피)에 속한다. 사용자 결정 침범 회피.

## Risks / Trade-offs

- **시각 회귀 (vertical padding)**: `py-20`(80px) → `py-16`(64px)로 -16px 감소. → 완화: 본 변경의 명시적 목적이 다른 6곳 빈 상태와 통일이므로 회귀가 아닌 정렬. 시각 비교는 EmptyState 기본 사용처 6곳과 동일하게 렌더링됨을 확인하는 것으로 충분.
- **DESIGN.md vertical rhythm 명세 미존재**: DESIGN.md가 빈 상태 vertical padding 수치를 직접 명시하지 않음. → 완화: EmptyState 컴포넌트 자체가 archive/2026-05-15-unify-empty-state-with-mascot에서 SSoT로 결정되었고, 본 변경은 그 결정을 1곳에 추가 적용하는 후속 정렬이므로 자의적 해석 아님.
- **import 순서/형식 회귀**: 새 import 추가 시 기존 import 그룹과 순서가 어긋날 수 있음. → 완화: 같은 alias prefix(`@/components/...`) 그룹에 알파벳 순으로 삽입.

## Migration Plan

1. `apps/web/src/app/search/page.tsx` 상단 import 섹션에 EmptyState import 1줄 추가.
2. L72-80의 인라인 빈 상태 마크업을 EmptyState 호출로 교체.
3. lint/type 검증은 변경 범위가 단일 컴포넌트 호출 교체라 별도 자동화 도구 없이도 시각 비교만으로 충분.
4. 롤백: 단일 커밋이므로 `git revert <commit>` 실행으로 즉시 원복 가능.
