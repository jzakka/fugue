## Why
seed.sql의 pin_tags INSERT문이 tags 테이블의 slug로 태그를 조회하지만, seed_tags.sql이 먼저 실행되지 않으면 tags 테이블이 비어있어 pin_tags가 0건 삽입된다. 결과적으로 모든 핀의 태그가 빈 배열로 표시되어 태그 기반 필터링/추천/검색이 동작하지 않는다.

## What Changes
- seed.sql에 seed_tags.sql 실행 의존성을 명확히 하거나, seed.sql 내에서 태그 시드를 포함
- Makefile의 seed 타겟에서 seed_tags.sql → seed.sql 순서 보장

## Capabilities
### New Capabilities
_(없음)_
### Modified Capabilities
_(없음 — 시드 데이터 정합성 복원)_

## Impact
- DB: seed.sql, seed_tags.sql 실행 순서 보장
- 개발 환경에서 태그 연결 데이터 정상화
