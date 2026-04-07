## Why

`seed.sql`이 `seed_tags.sql`을 전제 조건으로 참조하지만 해당 파일이 존재하지 않아 tags 테이블이 비어 있다. 태그가 필수인 핀 등록 기능이 100% 차단되며, 기존 시드 핀들도 태그 없이 생성되어 태그 기반 검색/필터가 작동하지 않는다.

## What Changes

- `apps/api/db/seed_tags.sql` 생성: Fugue 플랫폼의 사전 정의 태그 데이터 (음악, 일러스트, 영상, 글, 코드 등 크로스미디어 분야에 맞는 카테고리별 태그)
- `Makefile` seed 파이프라인 수정: `seed_tags.sql` → `seed.sql` 순서로 실행되도록 변경
- `seed.sql`에서 `-- Prerequisite` 주석을 실제 의존성으로 전환

## Capabilities

### New Capabilities

- `tag-seed-data`: 사전 정의 태그의 초기 데이터셋 정의 (카테고리, 이름, slug, 표시 순서)

### Modified Capabilities

(없음 — 코드 변경 없이 데이터만 추가. pin spec의 태그 관련 요구사항은 이미 구현 완료.)

## Impact

- **DB**: tags 테이블에 사전 정의 태그 INSERT, pin_tags 테이블에 시드 핀-태그 연결 데이터 정상 삽입
- **Makefile**: seed 타겟 실행 순서 변경
- **영향 범위**: 핀 생성 UI, 태그 필터, 검색 결과의 태그 표시, 피드 필터링
- **코드 변경**: 없음 (순수 데이터 + 빌드 파이프라인)
