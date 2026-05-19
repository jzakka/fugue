# proposal

## 변경 대상
- `apps/web/src/app/search/SearchClient.tsx:292` `<h2>` "핀" className `font-semibold` → `font-bold`
- `apps/web/src/app/search/SearchClient.tsx:306` `<h2>` "크리에이터" className `font-semibold` → `font-bold`
- `apps/web/src/app/search/SearchClient.tsx:344` `<h2>` "보드" className `font-semibold` → `font-bold`

총 3건의 동일 단어 치환. 시각 변경은 검색 결과 페이지의 세 섹션 헤딩 글자 굵기가 600 → 700으로 한 단계 강해짐.

## DESIGN.md 인용
- L17 "Display/Hero: General Sans 700 — 기하학적이면서 개성 있음"
  - 700 = bold. display 카테고리 헤딩의 표준 weight 명시.

## 코드 SSoT 인용
- 다른 h2 5건 모두 `font-bold` 보유:
  - `apps/web/src/app/pins/[id]/page.tsx:249`
  - `apps/web/src/components/pin/VideoTrimModal.tsx:128`
  - `apps/web/src/components/board/BoardGrid.tsx:14`
  - `apps/web/src/components/board/AddToBoardButton.tsx:210`
  - `apps/web/src/components/profile/MyPageClient.tsx:67`
- 페이지 h1 6건도 모두 `font-bold`. 헤딩 전반 14건 중 11/14 = 79%가 `font-bold` 표준.
- SearchClient 검색 결과 섹션 h2 3건만 `font-semibold` outlier.

## 사이클 48 archive와의 관계
사이클 48 `archive/2026-05-15-h2-display-font-tracking` 처리 시 SearchClient h2 3곳에 `font-display tracking-tight` 두 클래스를 추가했으나, weight 정렬(`font-semibold` → `font-bold`)은 "코드 SSoT 근거가 더 약해 별도 후보로 분리"라고 decision-log L33에서 명시해 분리한 잔여 갭. 본 변경은 그 분리해둔 후속 처리.

## 사용자 영향
- 검색 페이지(`/search`) 진입 후 'all' 탭에서 노출되는 세 섹션 헤딩("핀", "크리에이터", "보드")의 글자 굵기가 한 단계 강해짐.
- 같은 페이지의 페이지 헤딩 h1("'{query}' 검색 결과", L232 `font-bold`)과 weight 일치 → 헤딩 위계 내부 일관성 회복.
- 다른 페이지의 h2(예: 마이페이지 "보드", 핀 상세 "더 많은 핀")와 weight 일치 → 화면 간 일관성 회복.
- 검색 페이지 외 다른 화면에는 영향 없음.

## 변경 범위
- 1개 파일: `apps/web/src/app/search/SearchClient.tsx`
- 3개 라인
- `apps/api/` 미접근. 다른 컴포넌트 미접근.

## 롤백 절차
`git revert` 또는 SearchClient.tsx 3개 라인의 `font-bold`를 `font-semibold`로 되돌림.

## anti-pattern 자기 검사
- L15(Tailwind 기본 의미 덮어쓰기 vs 신규 토큰 추가): 적용 안됨. `font-bold`는 Tailwind 기본 유틸리티(`font-weight: 700`)로 의미 덮어쓰기 아님.
- L16(DESIGN.md radius scale 외 매직값의 자의적 등급 매핑): 적용 안됨. h2는 헤딩 카테고리이고 DESIGN.md L17이 display 카테고리 weight 700을 직접 명시.

## 자의적 해석 여지 사전 차단
- "section heading vs data group label" 카테고리 차이로 SearchClient h2를 약한 weight로 처리할 수도 있다는 해석 여지가 있으나, MyPageClient의 "보드"·BoardGrid의 "보드"·pins/[id]의 "더 많은 핀" 모두 동일한 의미의 데이터 카테고리 그룹 라벨이면서 `font-bold` 사용. SearchClient만 다르게 처리할 명세 근거 없음.
