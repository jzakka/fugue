## 1. EmptyState SSoT 경유 정렬

- [x] 1.1 `apps/web/src/app/search/page.tsx` import 섹션에 `import EmptyState from "@/components/feed/EmptyState";` 1줄 추가
- [x] 1.2 `apps/web/src/app/search/page.tsx` L72-80 인라인 빈 상태 9라인을 `<EmptyState message="검색어를 입력해주세요" description="작품, 크리에이터, 보드를 검색할 수 있습니다" />` 1줄로 교체

## 2. 검증

- [x] 2.1 diff에서 변경 파일이 `apps/web/src/app/search/page.tsx` 단일 파일임을 확인
- [x] 2.2 `grep -rln 'text-5xl' apps/web/src --include='*.tsx'` 결과에 search/page.tsx가 더 이상 매칭되지 않고 EmptyState.tsx만 남는지 확인
- [x] 2.3 검색 페이지 진입 후 검색어 미입력 상태에서 마스코트(🐡) + 메시지 + 부연 구조가 다른 6곳 빈 상태와 동일한 `py-16` vertical padding으로 렌더링됨을 시각 확인
