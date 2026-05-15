## Why

`DESIGN.md` L17은 디스플레이/히어로 타이포를 "General Sans 700"으로 명시한다. 그러나 `apps/web/src/app/globals.css`의 `@theme inline`(L40-56)은 색·border 토큰만 노출하고 폰트 패밀리 토큰은 정의하지 않는다. 그 결과 8개 파일이 같은 폰트 패밀리를 inline `style={{ fontFamily: "'General Sans', sans-serif" }}`로 직접 박아넣고 있다:

- `apps/web/src/app/pins/[id]/page.tsx:140,258`
- `apps/web/src/app/pin/new/PinCreateForm.tsx:316`
- `apps/web/src/app/search/SearchClient.tsx:234`
- `apps/web/src/app/boards/[id]/page.tsx:58`
- `apps/web/src/components/board/BoardGrid.tsx:16`
- `apps/web/src/components/board/AddToBoardButton.tsx:209`
- `apps/web/src/components/profile/MyPageClient.tsx:69`

디자인 시스템 관점에서:
- 폰트 패밀리 변경 시 8곳을 동시 수정해야 함.
- React inline style 객체가 디스플레이 헤딩마다 새로 생성됨.
- `globals.css`의 `@theme inline`이 디자인 토큰의 단일 진실 원천(SSoT) 역할을 하지 못함.

본 change는 `@theme inline`에 `--font-display: 'General Sans', sans-serif` 토큰을 추가해 Tailwind v4가 자동 생성하는 `font-display` 유틸리티를 노출하고, 8곳의 inline style을 className으로 일관화한다.

## What Changes

- `apps/web/src/app/globals.css`의 `@theme inline` 블록 끝에 `--font-display: 'General Sans', sans-serif;` 한 줄을 추가한다. Tailwind v4는 `--font-*` 네임스페이스 토큰을 자동으로 `font-<name>` 유틸리티로 생성한다.
- 위 8개 파일에서 `style={{ fontFamily: "'General Sans', sans-serif" }}` 속성을 제거하고, 같은 요소의 `className` 끝에 `font-display`를 추가한다.
- `font-mono`(Geist Mono) 케이스는 본 change 범위 밖. PinCard.tsx:182의 인라인 Geist Mono는 Tailwind 기본 `font-mono` 의미 덮어쓰기 가능성이 있어 별도 후보로 분리한다(backlog 노트 참조).
- General Sans 폰트 CDN 로드(layout.tsx)는 이미 archive 사이클에서 처리됨. 본 change는 토큰화 단독.

## Capabilities

### New Capabilities
없음. 디자인 시스템은 OpenSpec capability로 등록되어 있지 않다(`openspec/specs/`에 web/design 관련 spec 없음).

### Modified Capabilities
없음.

## Impact

- 영향 코드: globals.css 1줄 추가, 8개 파일의 inline style 제거 + className 1단어 추가. 총 9개 파일.
- 사용자 영향: 시각 변화 없음. 폰트 패밀리 값은 동일(General Sans, fallback sans-serif). inline style → CSS 클래스 경로만 바뀐다.
- 성능: 8개 컴포넌트의 inline style 객체 생성 사라짐(React 리렌더링에서 props identity 안정성 약간 개선). 폰트 파일 추가 다운로드 없음.
- 의존성·인프라·DB 변경 없음.
- DESIGN.md 명세 일치: L17 "Display/Hero: General Sans 700" 토큰화 완료. 한글 폴백은 body의 `font-family: 'Pretendard Variable'`이 그대로 처리.

## Rollback

- `apps/web/src/app/globals.css`에서 추가한 `--font-display` 한 줄을 git revert 또는 직접 제거.
- 8개 파일의 className에서 `font-display` 제거 + 기존 inline style 복원.
- 단일 커밋이므로 `git revert <commit>`으로 일괄 복귀 가능.
