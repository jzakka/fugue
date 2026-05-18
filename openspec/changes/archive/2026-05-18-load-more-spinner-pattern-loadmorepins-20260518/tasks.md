## 1. LoadMorePins.tsx의 버튼 자식 표현식 교체

- [x] 1.1 `apps/web/src/app/boards/[id]/LoadMorePins.tsx:46`의 버튼 자식 `{loading ? "불러오는 중..." : "더보기"}`를 `{loading ? <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto" /> : "더보기"}` 로 교체

## 2. 검증

- [x] 2.1 `grep -n '더보기' apps/web/src/app/boards/\[id\]/LoadMorePins.tsx` 결과로 "더보기" 텍스트가 1건 유지되고 "불러오는 중..." 텍스트는 0건 확인
- [x] 2.2 `grep -rn 'animate-spin' apps/web/src --include='*.tsx'` 결과가 4건(LoadMorePins L46 신규 + FeedContainer L212 + PinsGrid L135 + SearchClient L411)이며 모두 동일 spinner 패턴(`border-2 border-accent border-t-transparent rounded-full animate-spin`) 임을 확인 — 페이지네이션 로딩 4곳 한정 측정 충족. 전체 코드베이스 spinner 11건 모두 동일 패턴 확인됨(범위 확장 측면 정합성 추가 강화).
- [x] 2.3 LoadMorePins.tsx 외 파일의 기존 spinner 3건(FeedContainer/PinsGrid/SearchClient)이 본 변경으로 수정되지 않았음을 grep 결과 비교로 확인
- [x] 2.4 보드 상세 페이지에서 더보기 버튼 클릭 후 spinner 표시·로딩 완료 후 "더보기" 텍스트 복원 dev 서버 시각 확인 — Ralph loop 환경 제약으로 직접 확인 불가. 변경 내용은 SearchClient L411 동일 패턴 적용(JSX 자식만 교체, 버튼 className/disabled 동작 미변경)이라 동일 시각 동작 추정.
