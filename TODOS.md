# TODOS

## 사이트별 CSS Selector 기반 필터 규칙

**Priority:** Medium
**Added:** 2026-04-13 (eng review)
**Depends on:** 링크 필터 체인 PR 완료

현재 범용 필터만 있는데, 사이트마다 DOM 구조가 다르므로 사이트별 selector 규칙을 `bot_sites` 테이블에 저장하면 필터링 정확도 향상 가능. Link 구조체의 `[]Selector` 데이터를 활용하여 사이트별 특정 영역의 링크만 추출하거나 제외하는 규칙 적용. `bot_sites`에 `content_selector` 또는 `exclude_selectors` 컬럼 추가 필요.

## robots.txt / rel=nofollow 준수 레이어

**Priority:** Medium
**Added:** 2026-04-13 (eng review)
**Depends on:** 링크 필터 체인 PR 완료

크롤링 예절과 법적 리스크 감소를 위해 robots.txt 파싱 + rel=nofollow 링크 제외. 링크 필터 체인에 `RobotsFilter`를 추가하면 자연스러움. Go용 robots.txt 파서 라이브러리 필요 (e.g., `github.com/temoto/robotstxt`). 매 사이트 첫 크롤 시 robots.txt를 한 번 fetch하여 캐시.

## Graph Visualization: D3 CDN 오프라인 미지원

**Priority:** Medium
**Added:** 2026-04-14 (QA ISSUE-005, deferred)
**File:** `apps/api/cmd/bot-visualize/template.html:7`

D3.js를 CDN(`d3js.org`)에서만 로딩. 오프라인이나 프록시 환경에서 graph.html이 동작하지 않음. 인라인 번들링 또는 onerror fallback 검토 필요.

## Graph Visualization: 노드 라벨에 URL 패턴 미표시

**Priority:** Low
**Added:** 2026-04-14 (QA ISSUE-006, deferred)
**File:** `apps/api/cmd/bot-visualize/template.html:333`

노드 라벨이 `node_type`만 표시 (listing, detail 등). 333개 노드 중 80%+가 같은 텍스트. URL 패턴(`/ranking.php?mode=daily`)을 표시하면 가독성 향상.

## Graph Visualization: 키보드/스크린 리더 미지원

**Priority:** Low
**Added:** 2026-04-14 (QA ISSUE-007, deferred)
**File:** `apps/api/cmd/bot-visualize/template.html`

SVG 노드에 `tabindex`, `role`, `aria-label` 없음. 마우스 전용 인터랙션. 개발자 도구이므로 우선도 낮음.

## DB 마이그레이션 24번 실패 (bot_pioneer_runs 테이블 미존재)

**Priority:** High
**Added:** 2026-04-14 (QA 중 발견)
**File:** `apps/api/db/migrations/000024_add_bot_run_indexes.up.sql`

마이그레이션 24번이 `bot_pioneer_runs` 테이블 인덱스를 생성하려 하나, 해당 테이블 생성 마이그레이션이 없음. 24번 이후 마이그레이션(25번 `sample_url` 추가 등)이 전부 블로킹됨.

## 사용자 관심사 기반 추천 핀

**Priority:** High
**Added:** 2026-05-14 (기능 스펙 합의)

메인 피드를 사용자 취향에 맞춰 정렬한다. 사용자 입장에서 추가되는 변화는 둘:

1. 회원가입 마지막 단계에 "관심 분야 고르기" 화면이 한 번 등장한다.
2. 메인 피드의 정렬이 본인 취향 중심으로 바뀐다. 가입 직후부터 효과가 나타나고, 서비스를 쓸수록 정확해진다.

### 관심사 신호 (세 가지를 합산)

- 본인이 등록한 핀의 태그 (`pins` + `pin_tags`, 기존 활용 중)
- 사용자가 액션을 취한 핀의 태그 (`interactions` 테이블, 기존 존재하나 추천에 미활용)
  - 액션 타입별 가중치 적용 (board_add > pin > view)
- 온보딩에서 직접 선택한 관심 태그 (`creator_interests`, 신규 테이블)

### 핵심 설계 원칙

추천 시점에는 통일된 함수 하나(`GetUserTagInterests`)만 호출한다. 세 신호 합산은 함수 내부에서 한 SQL(CTE 3개 + UNION ALL + GROUP BY)로 처리한다. 추천 로직은 점수가 어디서 왔는지 모른다. 신호 추가 시 추천 로직은 손대지 않는다.

### 데이터 모델 변경

- 신규 테이블 `creator_interests(creator_id, tag_id, created_at)` (PK: creator_id + tag_id). `ON DELETE CASCADE`로 사용자 탈퇴 시 자동 정리.
- 기존 `interactions` 테이블 그대로 활용. 스키마 변경 없음.
- 기존 인덱스 `idx_interactions_user_time(user_id, created_at DESC)` 활용. 신규 인덱스 불필요.

### 콜드 스타트 분기 변경

현재: 본인 핀 10개 미만이면 무조건 최신순 폴백.
변경 후: 본인 핀이 적어도 온보딩 관심 태그가 있으면 personalized 진행. 둘 다 없을 때만 최신순 폴백.

### API 변경

- 신규: `POST /api/creators/me/interests` (온보딩 시 1회 저장용)
- 기존: `GET /api/feed` 응답 스키마 동일. 내부 추천 로직만 교체.

### 작업 범위

- DB 마이그레이션 1개 (`creator_interests` 테이블)
- 신규 SQL 쿼리 `GetUserTagInterests` (3-CTE UNION ALL 합산)
- `feed/handler.go`: `GetUserTagFrequency` 호출을 `GetUserTagInterests`로 교체. cold-start 분기 조건 수정.
- 온보딩 저장 API 핸들러 + 라우터 등록.
- 온보딩 화면 (별도 결정. 이번 작업 포함 여부 미정)

### 본 스펙 범위에서 제외

- 관심사 수정 화면/API (요청 범위 밖)
- 시간 감쇠 (오래된 액션 가중치 감소)
- 다양성 보정 (같은 크리에이터 연속 노출 방지, 분야 쏠림 방지)
- 이미 본 핀 제외
- 가중치 A/B 테스트

### 결정 대기 항목

1. 온보딩 화면을 이번 작업에 포함할지, 백엔드만 먼저 갖춰둘지
2. 온보딩에서 선택받을 태그 개수의 최소/최대 (예: 최소 3개, 최대 10개)
