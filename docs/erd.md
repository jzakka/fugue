# Fugue ERD

## 테이블 관계도

```
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│  auth_accounts  │       │    creators      │       │   pins (핀)     │
├─────────────────┤       ├─────────────────┤       ├─────────────────┤
│ id         (PK) │       │ id         (PK) │       │ id         (PK) │
│ creator_id (FK) │──N:1─→│ nickname        │←─1:N──│ creator_id (FK) │
│ provider        │       │ avatar_url      │       │ url             │
│ provider_id     │       │ email           │       │ title           │
│ email           │       │ created_at      │       │ description     │
│ created_at      │       │ updated_at      │       │ field           │
└─────────────────┘       └────────┬────────┘       │ tags       []   │
                                   │                │ og_image        │
                              1:N  │                │ og_data    JSON │
                                   │                │ pin_count       │
                          ┌────────▼────────┐       │ created_at      │
                          │     boards      │       └────────┬────────┘
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
| url | VARCHAR(1000) NOT NULL | 원본 URL |
| title | VARCHAR(200) NOT NULL | 제목 |
| description | VARCHAR(500) | 설명 (선택) |
| field | VARCHAR(50) NOT NULL | 분야 |
| tags | TEXT[] NOT NULL | 스타일 태그 (1~5개) |
| og_image | VARCHAR(1000) | OG 썸네일 URL |
| og_data | JSONB | OG 메타데이터 전체 |
| pin_count | INTEGER NOT NULL DEFAULT 0 | 같은 URL 핀 수 (denormalized) |
| created_at | TIMESTAMPTZ | |

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
| pin_count | denormalized 컬럼 | N+1 방지. INSERT/DELETE 시 같은 URL 기준 재계산 |
| 마이그레이션 | golang-migrate | Go 생태계 표준, up/down 쌍 |

DDL 원본: `apps/api/db/migrations/`
