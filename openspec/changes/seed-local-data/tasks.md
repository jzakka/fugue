## 0. 스키마 정리

- [x] 0.1 `000005_drop_project_types` 마이그레이션 추가 (UP: DROP TABLE, DOWN: 원래 CREATE+INSERT 복원)

## 1. 시드 SQL 작성

- [x] 1.1 `apps/api/db/seed.sql` 생성 — 프로덕션 실행 방지 경고 주석 포함
- [x] 1.2 TRUNCATE CASCADE 문 추가 (works → auth_accounts → creators 순)
- [x] 1.3 크리에이터 5명 INSERT (고정 UUID, 다양한 역할 태그)
- [x] 1.4 auth_accounts 5건 INSERT (각 크리에이터별 1건)
- [x] 1.5 작품 12건 INSERT (음악, 영상편집, 미술, 프로그래밍, 시나리오 라이터 분야 모두 포함, 다양한 태그)

## 2. Makefile 타겟

- [x] 2.1 `apps/api/Makefile`에 `seed` 타겟 추가 (`psql` 로 seed.sql 실행)

## 3. 검증

- [x] 3.1 `make seed` 실행하여 데이터 투입 확인
- [x] 3.2 반복 실행 멱등성 확인
- [x] 3.3 5개 분야(음악, 영상편집, 미술, 프로그래밍, 시나리오 라이터) 모두 존재 확인
