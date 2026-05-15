## Why

`DESIGN.md` L17은 디스플레이/히어로 타이포를 "General Sans 700 — 기하학적이면서 개성 있음. 한글 대체: Pretendard Bold"로 명시하고, L22-25는 Pretendard / General Sans / Geist Mono 세 폰트의 로딩 명세를 둔다. 그러나 `apps/web/src/app/layout.tsx`는 Pretendard와 Geist Mono의 CDN 스타일시트만 로드하고 General Sans는 로드하지 않는다. 그 결과:

- `apps/web/src/app/pins/[id]/page.tsx:140,258`, `apps/web/src/app/pin/new/PinCreateForm.tsx:316`, `apps/web/src/app/search/SearchClient.tsx:233`, `apps/web/src/app/boards/[id]/page.tsx:57`, `apps/web/src/components/board/BoardGrid.tsx:16`, `apps/web/src/components/board/AddToBoardButton.tsx:208`, `apps/web/src/components/profile/MyPageClient.tsx:68` 8개 헤딩이 `style={{ fontFamily: "'General Sans', sans-serif" }}`을 선언한다.
- General Sans CSS가 로드되지 않으므로 브라우저는 `sans-serif`로 폴백해 시스템 기본 sans(Helvetica/Arial/Roboto)로 렌더링한다.
- DESIGN.md가 정의한 "Dark Gallery / Editorial" 디스플레이 타이포 정체성이 실제 화면에서 누락된다.

본 change는 General Sans의 500/700 weight를 글로벌 레이아웃에서 로드해 위 8개 사용처가 의도대로 렌더링되도록 한다.

## What Changes

- `apps/web/src/app/layout.tsx`의 `<head>`에 General Sans 500/700 weight를 가져오는 `<link rel="stylesheet">`을 한 줄 추가한다.
- 폰트 소스는 Fontshare(`https://api.fontshare.com/v2/css?f[]=general-sans@500,700&display=swap`)를 채택한다. 근거: General Sans(Indian Type Foundry)는 Google Fonts에 등록되어 있지 않으며 Fontshare가 1차 배포처다. DESIGN.md L22-25는 "Google Fonts or self-hosted"라고 적었지만 Google Fonts에 없어 "self-hosted" 옵션을 CDN(Fontshare)으로 해석한다. Pretendard·Geist Mono도 jsdelivr CDN으로 로드 중이라 일관된다.
- 기존 Pretendard / Geist Mono `<link>`은 변경하지 않는다.

## Capabilities

### New Capabilities
없음. 디자인 시스템은 OpenSpec capability로 등록되어 있지 않다(`openspec/specs/`에 web/design 관련 spec 없음).

### Modified Capabilities
없음.

## Impact

- 영향 코드: `apps/web/src/app/layout.tsx` 단일 파일, 1 라인 추가.
- 사용자 영향: 위 8개 페이지/컴포넌트의 헤딩이 시스템 sans 대신 General Sans로 렌더링된다. 다른 영역(본문, 태그, 모노스페이스)은 변경 없음.
- 성능: 폰트 파일 2개(500/700) 추가 다운로드. `display=swap`으로 FOIT 방지. Pretendard(다이내믹 서브셋) 대비 추가 약 80~120KB 예상(woff2).
- 의존성·인프라·DB 마이그레이션 없음.
- 한글 텍스트: General Sans는 라틴 글리프만 제공하므로 한글은 Pretendard로 폴백(이미 inline 스타일이 `sans-serif` fallback을 두고 있고 body의 font-family가 Pretendard Variable이라 자연스럽게 폴백된다). DESIGN.md L17 "한글 대체: Pretendard Bold"와 일치.

## Rollback

- `apps/web/src/app/layout.tsx`에서 추가한 `<link>` 한 줄을 git revert 또는 직접 제거하면 즉시 이전 상태로 복귀. 다른 파일 변경 없음.
