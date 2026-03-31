# Fugue ERD

## 테이블 관계도

```
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│  auth_accounts  │       │    creators      │       │     works       │
├─────────────────┤       ├─────────────────┤       ├─────────────────┤
│ id         (PK) │       │ id         (PK) │       │ id         (PK) │
│ creator_id (FK) │──N:1─→│ nickname        │←─1:N──│ creator_id (FK) │
│ provider        │       │ bio             │       │ url             │
│ provider_id     │       │ roles      []   │       │ title           │
│ email           │       │ contacts  JSON  │       │ description     │
│ created_at      │       │ avatar_url      │       │ field           │
└─────────────────┘       │ created_at      │       │ tags       []   │
                          │ updated_at      │       │ og_image        │
                          └─────────────────┘       │ og_data    JSON │
                                                    │ created_at      │
┌─────────────────┐                                 └─────────────────┘
│  project_types  │
├─────────────────┤
│ id         (PK) │  (독립 테이블, 추천 시 참조)
│ name            │
│ required_fields │
└─────────────────┘
```

## 설계 결정

| 결정 | 선택 | 이유 |
|------|------|------|
| PK 타입 | UUID (gen_random_uuid()) | 순차 ID는 리소스 열거 공격에 취약. PG 16은 extension 없이 지원 |
| 계정 병합 | auth_accounts 분리 테이블 | 1인이 Google+Discord+Twitter 동시 사용. 컬럼 방식은 확장 불가 |
| 병합 로직 | 이메일 기반 자동 병합 | 같은 이메일이면 같은 creator에 auth_account 추가. 이메일 없으면 별도 계정 |
| 프로젝트 유형 | DB 테이블 (시드 데이터) | 하드코딩 대비 유연. 앱 시작 시 메모리 로드 또는 Redis 캐싱 |
| 마이그레이션 | golang-migrate | Go 생태계 표준, up/down 쌍, CLI + 라이브러리 지원 |

DDL 원본: `apps/api/db/migrations/`
