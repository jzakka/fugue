# Fugue ERD

## 테이블 관계도

```
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│  auth_accounts  │       │    creators      │       │   pins (핀)     │
├─────────────────┤       ├─────────────────┤       ├─────────────────┤
│ id         (PK) │       │ id         (PK) │       │ id         (PK) │
│ creator_id (FK) │──N:1─→│ nickname        │←─1:N──│ creator_id (FK) │
│ provider        │       │ avatar_url      │       │ media_url       │
│ provider_id     │       │ email           │       │ media_type      │
│ email           │       │ created_at      │       │ url             │
│ created_at      │       │ updated_at      │       │ title           │
└─────────────────┘       └────────┬────────┘       │ description     │
                                   │                │ og_image        │
                              1:N  │                │ og_data    JSON │
                                   │                │ created_at      │
                          ┌────────▼────────┐       └────────┬────────┘
                          │     boards      │                │
                          ├─────────────────┤                │
                          │ id         (PK) │                │
                          │ creator_id (FK) │       ┌────────▼────────┐
                          │ name            │       │   board_pins    │
                          │ description     │       ├─────────────────┤
                          │ is_public       │◄──N:M─│ board_id  (FK)  │
                          │ created_at      │       │ pin_id    (FK)  │
                          │ updated_at      │       │ PK: (board_id,  │
                          └─────────────────┘       │     pin_id)     │
                                                    │ created_at      │
                                                    └─────────────────┘

┌─────────────────┐       ┌─────────────────┐
│      tags       │       │    pin_tags     │
├─────────────────┤       ├─────────────────┤
│ id         (PK) │←─1:N──│ pin_id    (FK)  │
│ name            │       │ tag_id    (FK)  │
│ slug            │       │ PK: (pin_id,    │
│ category        │       │     tag_id)     │
│ display_order   │       └─────────────────┘
└─────────────────┘
```

> **용어**: pins 테이블의 creator_id는 "핀한 사람"을 가리킨다. 원작자가 아닌 큐레이터.
> URL에 유니크 제약 없음 (여러 사람이 같은 URL을 핀할 수 있다).
> creators 테이블은 단순 계정 역할. 포트폴리오 기능 없음.
> 행동 이벤트(view, pin, board_add)는 RDB가 아닌 이벤트 파이프라인(S3)에 저장한다. 상세는 [architecture.md](architecture.md)의 "이벤트 파이프라인" 참조.

## 테이블

### pins
외부 창작물을 큐레이션한 핀. 핵심 엔티티.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | UUID PK | gen_random_uuid() |
| creator_id | UUID FK → creators | 핀한 유저 |
| media_url | VARCHAR(500) NOT NULL | S3 미디어 URL |
| media_type | VARCHAR(10) NOT NULL | 미디어 유형 (CHECK: image/audio/video) |
| url | VARCHAR(1000) | 원본 URL (선택) |
| title | VARCHAR(200) NOT NULL | 제목 |
| description | VARCHAR(500) | 설명 (선택) |
| og_image | VARCHAR(1000) | 대표 이미지 URL — `harvester-image-cache` capability에 의해 기록. 캐시 성공 시 object storage URL, 실패 시 원본 후보 URL, 후보 없음 시 NULL |
| og_data | JSONB | OG 메타데이터 + 추출/분류 증거. 키: `source`(항상 fetch URL), `extractor`(`generic` \| `script:<site_id>` \| `<adapter_name>`), `classifier.{pinnable, reason?}` (reason enum: `listing` \| `empty_body` \| `no_primary_media`), `media_candidates[]`, `lang`, `author`, `published_at`. **키 부재 계약**: `body_text`와 `canonical_url`은 포함되지 않음 — `body_text`는 `description`에, canonical은 `url`에만 저장된다 |
| created_at | TIMESTAMPTZ | |

**Partial unique index** `pins_url_bot_unique`: 봇 creator_id(`00000000-0000-0000-0000-00000000f096`)가 생성한 Pin에 한해 `url`을 유일 제약으로 강제한다(`harvester-pin-document` change). 일반 사용자는 동일 URL을 중복 핀할 수 있으며 해당 row는 이 인덱스에 포함되지 않는다. PostgreSQL partial index predicate는 IMMUTABLE이어야 하므로 UUID는 하드코딩된 리터럴로 migration 과 `UpsertBotPinByURL` 쿼리 양쪽에 동일하게 존재한다(`apps/api/internal/bot/source.go` IMMUTABLE-sync policy 참고).

### tags
사전정의된 태그 목록.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | UUID PK | gen_random_uuid() |
| name | VARCHAR(100) NOT NULL | 태그 표시명 |
| slug | VARCHAR(100) NOT NULL UNIQUE | URL 친화적 식별자 |
| category | VARCHAR(50) | 태그 분류 |
| display_order | INTEGER NOT NULL DEFAULT 0 | 정렬 순서 |

### pin_tags
핀-태그 N:M 관계.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| pin_id | UUID FK → pins | ON DELETE CASCADE |
| tag_id | UUID FK → tags | ON DELETE CASCADE |
| PK | (pin_id, tag_id) | 중복 추가 방지 |

### boards
보드 (핀 컬렉션). Pinterest 보드와 동일한 컨셉.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | UUID PK | gen_random_uuid() |
| creator_id | UUID FK → creators | 보드 소유자 |
| name | VARCHAR(100) NOT NULL | 보드 이름 |
| description | VARCHAR(500) | 보드 설명 (선택) |
| is_public | BOOLEAN DEFAULT true | 공개 여부 |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

### board_pins
보드-핀 N:M 관계.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| board_id | UUID FK → boards | ON DELETE CASCADE |
| pin_id | UUID FK → pins | ON DELETE CASCADE |
| created_at | TIMESTAMPTZ | 보드에 추가된 시각 |
| PK | (board_id, pin_id) | 중복 추가 방지 |

## 크롤러 frontier 테이블

Pioneer/Harvester가 공유하는 영속 URL 큐. 복수 워커로 수평 확장해도 중복 fetch가 발생하지 않도록 Postgres 기반으로 보관한다. 상세한 claim 규약은 `scheduler-claim-api` 도입 이후 정의된다.

### pioneer_frontier
Pioneer가 fetch 대상 URL을 쌓는 큐. fetch 상태·실패 카운터·재fetch 스케줄을 보관.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | BIGSERIAL PK | 자동 증가 |
| normalized_url | TEXT NOT NULL | 정규화된 URL |
| url | TEXT NOT NULL | 원본 URL |
| url_hash | BYTEA NOT NULL | sha256(normalized_url) 32바이트, UNIQUE + CHECK(octet_length=32) |
| host | TEXT NOT NULL | 호스트명 (포트 제외, 원본 대소문자/`www.` 유지) |
| depth | INTEGER NOT NULL DEFAULT 0 | BFS depth |
| score | DOUBLE PRECISION NOT NULL DEFAULT 0 | 0.0~1.0 우선순위 가중치 |
| last_fetched_at | TIMESTAMPTZ | 마지막 fetch 성공 시각 |
| next_fetch_at | TIMESTAMPTZ NOT NULL DEFAULT now() | 다음 fetch 가능 시각 (claim 시 `now()+10m`로 lease marker 겸용) |
| fetch_error_count | INTEGER NOT NULL DEFAULT 0 | 누적 실패 횟수 (5 도달 시 claim 제외) |
| last_updated_at | TIMESTAMPTZ NOT NULL DEFAULT now() | application이 매 UPDATE 시 명시 세팅 |

Partial index `pioneer_frontier_claimable_idx`: `(score DESC, next_fetch_at ASC) WHERE fetch_error_count < 5`.

### harvester_frontier
Pioneer가 fetch에 성공한 URL을 Harvester 소비용으로 fanout해 쌓는 큐. harvest 상태·실패 카운터·snapshot 참조를 보관.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | BIGSERIAL PK | 자동 증가 |
| normalized_url | TEXT NOT NULL | 정규화된 URL |
| url | TEXT NOT NULL | 원본 URL |
| url_hash | BYTEA NOT NULL | sha256(normalized_url) 32바이트, UNIQUE + CHECK(octet_length=32) |
| host | TEXT NOT NULL | 호스트명 |
| snapshot_key | TEXT | Pioneer가 저장한 HTML snapshot 참조 키 |
| score | DOUBLE PRECISION NOT NULL DEFAULT 0 | 우선순위 가중치 |
| harvested_at | TIMESTAMPTZ | 처리 완료 시각 (NOT NULL이면 partial index 제외) |
| next_harvest_at | TIMESTAMPTZ NOT NULL DEFAULT now() | 다음 harvest 가능 시각 (claim lease 겸용) |
| harvest_error_count | INTEGER NOT NULL DEFAULT 0 | 누적 실패 횟수 |
| last_updated_at | TIMESTAMPTZ NOT NULL DEFAULT now() | application이 매 UPDATE 시 명시 세팅 |

Partial index `harvester_frontier_claimable_idx`: `(score DESC, next_harvest_at ASC) WHERE harvested_at IS NULL AND harvest_error_count < 5`.

### harvester_frontier_pins
`harvester_frontier` 1 row ↔ 여러 Pin의 조인 테이블. ScriptAdapter가 N개 Pin을 생성할 수 있으므로 1:N.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| frontier_id | BIGINT NOT NULL FK → harvester_frontier | ON DELETE CASCADE |
| pin_id | UUID NOT NULL FK → pins | ON DELETE CASCADE (pins.id가 UUID) |
| PK | (frontier_id, pin_id) | 중복 링크 방지 |

## 이벤트 데이터 (S3)

행동 이벤트는 PostgreSQL이 아닌 S3에 Parquet 형식으로 저장한다.
RDB에 쓰기 부하를 주지 않고, 대규모 배치 분석과 ML 학습에 적합한 구조.

### 이벤트 스키마

| 필드 | 타입 | 설명 |
|------|------|------|
| event_id | STRING | UUID |
| user_id | STRING | creators.id |
| pin_id | STRING | pins.id |
| event_type | STRING | 'view', 'pin', 'board_add' |
| timestamp | TIMESTAMP | ISO 8601 |
| context | JSON | 확장 가능한 컨텍스트 (source, session_id, field 등) |

### S3 파티셔닝

```
s3://fugue-events/year=YYYY/month=MM/day=DD/hour=HH/events-NNN.parquet
```

상세 파이프라인은 [architecture.md](architecture.md)의 "이벤트 파이프라인" 참조.

## 설계 결정

| 결정 | 선택 | 이유 |
|------|------|------|
| PK 타입 | UUID (gen_random_uuid()) | 순차 ID는 리소스 열거 공격에 취약 |
| 계정 병합 | auth_accounts 분리 테이블 | 1인이 Google+Discord 동시 사용 |
| 병합 로직 | 이메일 기반 자동 병합 | 같은 이메일이면 같은 creator에 auth_account 추가 |
| 큐레이션 모델 | 소유권 없는 핀 | 외부 API로 소유권 검증 불가. 소유권 문제 자체를 제거 |
| URL 유니크 | 제약 없음 | 큐레이션이므로 여러 사람이 같은 작품을 핀할 수 있음 |
| 이벤트 저장 | S3 (Kinesis Firehose 경유) | RDB 부하 방지, 대규모 배치 분석/ML 학습에 적합, 내구성 11 9's |
| board_pins PK | Composite (board_id, pin_id) | 같은 핀을 같은 보드에 중복 추가 방지 |
| 태그 방식 | 사전정의 태그 (tags 테이블) | 자유 텍스트 대신 일관성 있는 분류 체계. pin_tags로 N:M 연결 |
| media_type | CHECK 제약 (image/audio/video) | 미디어 유형 명시. field 컬럼 제거 |
| 마이그레이션 | golang-migrate | Go 생태계 표준, up/down 쌍 |

DDL 원본: `apps/api/db/migrations/`
