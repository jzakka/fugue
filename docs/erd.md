# Fugue ERD

## 테이블 관계도

```
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│  auth_accounts  │       │    creators      │       │  works (핀)     │
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
                                   │                │ created_at      │
                          ┌────────▼────────┐       └────────┬────────┘
                          │     boards      │                │
                          ├─────────────────┤                │
                          │ id         (PK) │                │
                          │ creator_id (FK) │       ┌────────▼────────┐
                          │ name            │       │   board_pins    │
                          │ description     │       ├─────────────────┤
                          │ is_public       │◄──N:M─│ board_id  (FK)  │
                          │ created_at      │       │ work_id   (FK)  │
                          │ updated_at      │       │ PK: (board_id,  │
                          └─────────────────┘       │     work_id)    │
                                                    │ created_at      │
                          ┌─────────────────┐       └─────────────────┘
                          │  interactions   │
                          ├─────────────────┤
                          │ id         (PK) │
                          │ user_id    (FK) │  → creators.id
                          │ work_id    (FK) │  → works.id
                          │ type            │  'view' | 'pin' | 'board_add'
                          │ created_at      │
                          └─────────────────┘
```

> **용어**: works 테이블의 creator_id는 "핀한 사람"을 가리킨다. 원작자가 아닌 큐레이터.
> URL에 유니크 제약 없음 (여러 사람이 같은 URL을 핀할 수 있다).
> creators 테이블은 단순 계정 역할. 포트폴리오 기능 없음.

## 새 테이블

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
| work_id | UUID FK → works | ON DELETE CASCADE |
| created_at | TIMESTAMPTZ | 보드에 추가된 시각 |
| PK | (board_id, work_id) | 중복 추가 방지 |

### interactions
유저 행동 기록. 추천 엔진의 입력 데이터. 추후 ML 학습 데이터로 활용.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | UUID PK | gen_random_uuid() |
| user_id | UUID FK → creators | |
| work_id | UUID FK → works | |
| type | VARCHAR(20) NOT NULL | 'view', 'pin', 'board_add' |
| created_at | TIMESTAMPTZ | |

인덱스: `(user_id, created_at DESC)`, `(work_id)`, `(type)`

## 설계 결정

| 결정 | 선택 | 이유 |
|------|------|------|
| PK 타입 | UUID (gen_random_uuid()) | 순차 ID는 리소스 열거 공격에 취약 |
| 계정 병합 | auth_accounts 분리 테이블 | 1인이 Google+Discord 동시 사용 |
| 병합 로직 | 이메일 기반 자동 병합 | 같은 이메일이면 같은 creator에 auth_account 추가 |
| 큐레이션 모델 | 소유권 없는 핀 | 외부 API로 소유권 검증 불가. 소유권 문제 자체를 제거 |
| URL 유니크 | 제약 없음 | 큐레이션이므로 여러 사람이 같은 작품을 핀할 수 있음 |
| interactions 보존 | 무제한 (MVP) | ML 학습 데이터로 활용. 추후 파티셔닝 고려 |
| board_pins PK | Composite (board_id, work_id) | 같은 핀을 같은 보드에 중복 추가 방지 |
| 마이그레이션 | golang-migrate | Go 생태계 표준, up/down 쌍 |

DDL 원본: `apps/api/db/migrations/`
