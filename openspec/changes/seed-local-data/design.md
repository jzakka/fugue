## Context

Fugue 로컬 개발 환경은 docker-compose로 PostgreSQL을 띄우고 golang-migrate로 스키마를 생성한다. 현재 마이그레이션 후 DB가 비어 있어 기능 테스트가 불편하다. creators, auth_accounts, works 테이블에 현실적인 샘플 데이터를 넣는 시드 스크립트가 필요하다.

## Goals / Non-Goals

**Goals:**
- `make seed` 한 줄로 샘플 데이터 투입
- 크리에이터 5명 × 분야별 작품 2~3건 = 10~15건의 현실적 데이터
- 멱등성: 여러 번 실행해도 안전 (TRUNCATE 후 INSERT)
- 초기 분야: 음악, 영상편집, 미술, 프로그래밍, 시나리오 라이터
- 다양한 분야/태그 조합으로 추천 로직 테스트 가능

**Non-Goals:**
- 대량 데이터 생성 (벤치마크/부하 테스트용)
- 프로덕션 시드 데이터
- 자동화된 fixture 프레임워크

## Decisions

### 1. 단일 SQL 파일: `db/seed.sql`

하나의 SQL 파일에 TRUNCATE + INSERT를 순서대로 나열. Go 코드나 별도 도구 없이 `psql`로 직접 실행.

**대안**: Go seed 커맨드, testfixtures 라이브러리. 현 단계에서 오버엔지니어링이라 제외.

### 2. 고정 UUID 사용

시드 데이터에 고정 UUID를 사용하여 재현 가능성 확보. 디버깅 시 특정 크리에이터/작품을 ID로 바로 조회 가능.

### 3. TRUNCATE CASCADE로 멱등성 확보

시드 실행 시 기존 데이터를 TRUNCATE CASCADE로 정리한 후 INSERT. 여러 번 실행해도 동일한 결과.

## Risks / Trade-offs

- **[TRUNCATE는 기존 데이터 삭제]** → 로컬 전용이므로 허용. 프로덕션에서 실행 방지를 위해 시드 파일 상단에 경고 주석.
- **[고정 UUID는 충돌 가능]** → 충분히 고유한 값 사용. 로컬 전용이므로 실질적 위험 없음.
