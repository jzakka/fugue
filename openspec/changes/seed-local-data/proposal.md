## Why

로컬에서 `docker-compose up` + 마이그레이션 후 DB가 비어 있어서, 탐색/추천/프로필 등 기능 개발 시 매번 수동으로 데이터를 넣어야 한다. 시드 데이터를 Makefile 한 줄로 넣을 수 있으면 개발 속도가 올라간다.

## What Changes

- `apps/api/db/seed.sql` 시드 파일 추가 (creators + auth_accounts + works)
- `apps/api/Makefile`에 `seed` 타겟 추가
- 크리에이터 5명, 작품 10~15건 정도의 현실적인 샘플 데이터
- 다양한 분야(음악, 영상편집, 미술, 프로그래밍, 시나리오 라이터)와 태그 조합 포함

## Capabilities

### New Capabilities
- `seed-data`: 로컬 개발용 시드 SQL과 Makefile 타겟

### Modified Capabilities

(없음)

## Impact

- `apps/api/db/seed.sql` 신규 파일
- `apps/api/Makefile` 타겟 추가
- 프로덕션 영향 없음 (로컬 전용)
