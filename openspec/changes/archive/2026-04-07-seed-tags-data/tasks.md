## 1. 태그 시드 데이터 작성

- [x] 1.1 `apps/api/db/seed_tags.sql` 생성: TRUNCATE tags CASCADE + 카테고리별 태그 INSERT (음악, 일러스트, 영상, 글, 코드, 공통). seed.sql이 참조하는 29개 slug 전부 포함 확인.
- [x] 1.2 태그 display_order 설정: 카테고리 내에서 자주 쓸 태그가 앞에 오도록 순서 부여

## 2. Makefile 수정

- [x] 2.1 Makefile `seed` 타겟에서 `seed_tags.sql` → `seed.sql` 순서로 실행하도록 변경
- [x] 2.2 seed.sql의 `-- Prerequisite: run seed_tags.sql first` 주석 제거 (실제 의존성으로 대체됨)

## 3. 검증

- [x] 3.1 `make dev-stop && make dev` 실행하여 에러 없이 기동 확인
- [x] 3.2 `SELECT count(*) FROM tags` 및 `SELECT count(*) FROM pin_tags` 로 데이터 존재 확인
- [x] 3.3 브라우저에서 핀 생성 페이지 접근 → 카테고리 탭과 태그 목록이 정상 표시되는지 확인
- [x] 3.4 `make seed`를 2회 연속 실행하여 멱등성 확인
