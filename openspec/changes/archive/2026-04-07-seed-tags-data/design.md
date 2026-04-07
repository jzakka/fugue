## Context

Fugue는 크로스미디어 큐레이션 플랫폼으로, 핀 생성 시 사전 정의된 태그를 필수로 선택해야 한다. tags 테이블 스키마와 API 엔드포인트, 프론트엔드 UI는 모두 구현 완료 상태이나, 실제 태그 데이터가 DB에 존재하지 않아 핀 등록 기능이 차단되어 있다.

현재 상태:
- `000011_create_tags.up.sql`: tags, pin_tags 테이블 생성 마이그레이션 존재
- `seed.sql`: `-- Prerequisite: run seed_tags.sql first` 주석 존재, pin_tags INSERT가 tags 참조
- `seed_tags.sql`: **파일 미존재**
- Makefile `seed` 타겟: `seed.sql`만 실행

## Goals / Non-Goals

**Goals:**
- Fugue의 크로스미디어 정체성에 맞는 태그 데이터셋 정의 (음악, 일러스트, 영상, 글, 코드)
- `make dev` 한 번으로 태그 포함 전체 시드가 완료되도록 파이프라인 수정
- 기존 시드 핀-태그 연결이 정상 동작하도록 보장

**Non-Goals:**
- 태그 관리 어드민 UI 구현
- 태그 CRUD API 추가 (현재 읽기 전용으로 충분)
- 태그 자동완성이나 인기 태그 정렬 로직

## Decisions

### 1. 태그 카테고리 체계

Fugue의 5대 분야(음악, 일러스트, 영상, 글, 코드)에 맞춰 카테고리를 설계한다. 추가로 분야를 넘나드는 공통 태그를 위한 `공통` 카테고리를 둔다.

| 카테고리 | 대상 |
|---------|------|
| 음악 | 장르, 악기, 분위기 등 음악 관련 태그 |
| 일러스트 | 화풍, 주제, 기법 등 시각 예술 태그 |
| 영상 | 영상 유형, 기법 태그 |
| 글 | 문학 장르, 형식 태그 |
| 코드 | 기술 스택, 프로젝트 유형 태그 |
| 공통 | 분야 무관 태그 (collaboration, commission 등) |

**대안:** 영문 카테고리명 사용 → 기각. UI가 한국어이므로 카테고리도 한국어로 통일.

### 2. 태그 네이밍 규칙

- `name`: 한국어 또는 영문 고유명사 (UI 표시용). 예: "신스팝", "pixel-art"
- `slug`: 영문 kebab-case (API/URL 참조용). 예: "synthpop", "pixel-art"
- 영어가 자연스러운 장르명/기술 용어는 영문 유지 (예: "lo-fi", "Unity", "Three.js")
- 한국어가 자연스러운 것은 한국어 사용 (예: "배경화", "시나리오")

### 3. seed 실행 순서

`Makefile`의 `seed` 타겟에서 `seed_tags.sql` → `seed.sql` 순서로 실행한다. `seed.sql`의 TRUNCATE CASCADE가 pin_tags, pins를 먼저 비우므로 tags 데이터 충돌 없음. seed_tags.sql은 자체적으로 tags 테이블을 TRUNCATE 후 INSERT한다.

**대안:** seed.sql 안에 태그 데이터를 합치기 → 기각. 태그 데이터는 독립적으로 관리하는 것이 유지보수에 유리.

### 4. seed.sql의 기존 slug 참조 유지

seed.sql의 `INSERT INTO pin_tags ... FROM tags WHERE slug IN (...)` 구문을 그대로 활용한다. seed_tags.sql에서 해당 slug들이 모두 정의되도록 보장하면 된다.

필요한 slug 목록 (seed.sql에서 참조):
`synthpop`, `dreamy`, `indie`, `electronic`, `cyberpunk`, `beatmaking`, `fantasy`, `background-art`, `illustration`, `concept-art`, `character-design`, `kawaii`, `album-art`, `commission`, `music-video`, `motion-graphics`, `collaboration`, `typography`, `showreel`, `game`, `unity`, `pixel-art`, `web-app`, `threejs`, `interactive`, `voice-drama`, `scenario`, `romance`, `visual-novel`

## Risks / Trade-offs

- **[태그 목록 확장성]** 초기 태그셋이 너무 적으면 사용자가 적절한 태그를 못 찾음 → 분야당 8-15개로 시작, 사용 패턴 보고 추가
- **[seed 멱등성]** `make dev`를 여러 번 실행해도 안전해야 함 → `TRUNCATE tags CASCADE` + `INSERT`로 매번 깨끗하게 시작
- **[기존 시드 핀 깨짐]** seed_tags.sql에 seed.sql이 참조하는 slug가 빠지면 pin_tags가 0건 → seed.sql의 slug 전수 확인 필수
