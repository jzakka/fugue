## 1. timestamp className 교체 (1건)

- [x] 1.1 `apps/web/src/app/search/SearchClient.tsx:328` className `text-xs text-text-dim font-mono` → `text-2xs text-text-dim font-mono` (`text-xs` 단어 1개를 `text-2xs`로 교체).

## 2. 사후 검증

- [x] 2.1 `grep -nE 'text-2xs' apps/web/src --include='*.tsx'` 결과 3건 확인(이전 2건: VideoTrimModal L164·L222 + 신규 1건: SearchClient L328).
- [x] 2.2 `grep -nE 'created_at|toLocaleDateString' apps/web/src/app apps/web/src/components --include='*.tsx'` 결과에서 timestamp 표시 위치의 className에 `text-xs` 0건 확인(SearchClient L328이 신규 `text-2xs`로 정렬됨).
- [x] 2.3 라인 시프트 0(className 길이 동일: `text-xs` 7자 → `text-2xs` 8자, 1자 증가지만 라인 번호 영향 없음). 다른 후보/아카이브 항목의 라인 의존성에 영향 없음.
- [x] 2.4 변경 외 파일(globals.css, 다른 컴포넌트) 수정 0건 확인. git diff stat: 1 파일(SearchClient 1 라인 ±) 정확히 일치.
