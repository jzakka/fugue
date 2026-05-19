## Why

`apps/web/src/app/search/page.tsx` L72-80의 검색어 빈 상태(`q.length === 0` 분기)가 EmptyState 공통 컴포넌트(`apps/web/src/components/feed/EmptyState.tsx`)를 거치지 않고 인라인 마크업으로 동일 구조(컨테이너 flex/center + 🐡 + 메시지 + 부연)를 복제 중. archive/2026-05-15-unify-empty-state-with-mascot이 6곳 빈 상태를 EmptyState로 통일했으나 search/page.tsx `q.length === 0` 분기가 그 정렬에서 누락되었고, DESIGN.md L14 '마스코트: 헤드셋 끼고 붓 든 복어. 로고/빈 상태/온보딩에서 활용.' SSoT 일관성이 1곳 어긋남.

## What Changes

- `apps/web/src/app/search/page.tsx` L72-80 인라인 9라인을 `<EmptyState message="검색어를 입력해주세요" description="작품, 크리에이터, 보드를 검색할 수 있습니다" />` 1줄로 교체
- `apps/web/src/app/search/page.tsx` import 섹션에 `import EmptyState from "@/components/feed/EmptyState";` 1줄 추가

## Capabilities

### New Capabilities

- `design-tokens`: DESIGN.md SSoT의 행위 계약을 코드에 일관 반영하는 영역. 본 변경은 빈 상태(empty state) 위계를 마스코트·메시지·부연으로 통일하는 요구사항을 추가한다.

### Modified Capabilities

(없음)

## Impact

- 영향 파일: `apps/web/src/app/search/page.tsx` 단일 파일 (1라인 import 추가 + 9라인 → 1라인 교체)
- 사용자 영향: 검색 페이지 진입 후 검색어 미입력 상태에서 vertical padding `py-20`(80px) → EmptyState 기본 `py-16`(64px)로 -16px 감소. 마스코트(🐡 text-5xl) · 메시지(text-text-muted text-sm) · 부연(text-text-dim text-xs) 구조는 그대로 유지. 다른 6곳(FeedContainer · SearchClient 결과 없음 · PinsGrid · MyPageClient · AddToBoardButton · boards/[id]/page) 빈 상태와 vertical padding 일관성 회복.
- 외부 영향 없음: `apps/api/` 및 `apps/web/` 외부 파일 미변경. EmptyState 컴포넌트 자체 미변경(API 그대로 사용).
- 롤백 절차: git revert로 단일 커밋 되돌리기.
