## 1. globals.css에 scrollbar-hide utility 정의

- [x] 1.1 `apps/web/src/app/globals.css` 파일 끝부분(기존 `.skeleton-shimmer .bg-surface-elevated { ... }` 룰 다음)에 `.scrollbar-hide { -ms-overflow-style: none; scrollbar-width: none; }` 룰 추가
- [x] 1.2 같은 위치에 `.scrollbar-hide::-webkit-scrollbar { display: none; }` 룰 추가

## 2. 검증

- [x] 2.1 `grep -nE '\.scrollbar-hide' apps/web/src/app/globals.css` 결과가 2건(룰 헤더 2개)이고 모두 `@theme inline` 블록 밖에 위치함을 확인 — L104·L109 확인, `@theme inline` 블록은 L40-61.
- [x] 2.2 `grep -rn 'scrollbar-hide' apps/web/src --include='*.tsx'` 결과가 기존 5건(FieldFilter L31·TagFilter L44·SearchClient L237/L255·PinCreateForm L541) 그대로이며 추가/제거가 없음을 확인 — 5건 유지 확인.
- [x] 2.3 5곳 사용처 가로 스크롤 영역(메인 피드 필터·태그, 검색 페이지 카테고리 탭·태그, 핀 등록 폼 태그 카테고리 탭)에 OS 기본 가로 스크롤바가 노출되지 않으며 가로 스크롤 자체(터치 스와이프·키보드 화살표) 동작이 유지됨을 dev 서버에서 시각 확인 — 환경 제약으로 직접 확인 불가 시 그 사실 보고 — Ralph 루프 환경에서 dev 서버 기동·브라우저 시각 검증 불가. 코드 검증(2.1·2.2)으로 정의·사용처 정합성 확인됨.
