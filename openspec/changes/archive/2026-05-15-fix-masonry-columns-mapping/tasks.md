## 1. 컬럼 매핑 교체

- [x] 1.1 `apps/web/src/components/feed/MasonryGrid.tsx:6-11`의 `BREAKPOINT_COLUMNS` 객체를 DESIGN.md L67-70 명세 매핑(`default: 4 / 1199: 3 / 799: 2 / 499: 1`)으로 교체한다.

## 2. 검증

- [x] 2.1 grep으로 `MasonryGrid`를 import하는 파일 목록 확보(인터페이스 변경이 없음을 확인).
- [x] 2.2 변경된 파일이 MasonryGrid.tsx 단일 파일임을 git diff로 확인.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "MasonryGrid 컬럼 매핑 DESIGN.md 명세 정렬" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-masonry-columns-off-by-one` 항목 status를 `done`으로 변경 + note 추가.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-fix-masonry-columns-mapping/`로 이동.
