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
