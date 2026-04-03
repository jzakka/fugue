# Tasks

MVP 구현 태스크 관리. 큐레이션 모델 피벗 이후 기준.

## 구조

```
tasks/
├── phase1-foundation/    # 기반 정리 + 핀 CRD
├── phase2-boards/        # 보드 + 프로필 간소화
├── phase3-recommendation/# 추천 + 연관 작품
└── README.md
```

## 상태

- `[ ]` 미착수
- `[~]` 진행 중
- `[x]` 완료

## Phase 개요

| Phase | 내용 | 의존성 |
|-------|------|--------|
| 1 | DB 마이그레이션 + 핀 CRD + OG fetch + 프론트 핀 등록 | 없음 |
| 2 | 보드 CRUD + 프로필 간소화 | Phase 1 |
| 3 | interactions + 추천 피드 + 연관 작품 | Phase 1, 2 |
