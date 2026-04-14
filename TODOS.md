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
