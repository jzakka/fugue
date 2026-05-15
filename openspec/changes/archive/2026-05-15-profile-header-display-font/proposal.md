## Why

직전 archive change `2026-05-15-token-display-font-family`에서 `--font-display` 토큰을 정의하고 동일한 디스플레이 패턴(`text-2xl sm:text-3xl font-bold tracking-tight`)을 가진 헤딩 8곳에 `font-display` 클래스를 적용했다. 그러나 같은 패턴을 가진 `apps/web/src/components/profile/ProfileHeader.tsx:32`의 `<h1>{creator.nickname}</h1>`는 사이클 20의 grep 출력에 잡히지 않았다 — 이 컴포넌트는 처음부터 inline `style={{ fontFamily: "'General Sans', sans-serif" }}`을 적용하지 않은 채로 같은 패턴의 헤딩을 두고 있었기 때문이다.

결과적으로 크리에이터 프로필 페이지(`apps/web/src/app/creators/[id]/page.tsx`가 사용)의 최상단 닉네임 헤딩만 디스플레이 폰트가 적용되지 않아 다른 페이지의 동일 위계 헤딩(보드 상세, 검색 결과, 핀 상세 등)과 폰트 패밀리가 불일치한다. DESIGN.md L17 "Display/Hero: General Sans 700" 위반.

## What Changes

- `apps/web/src/components/profile/ProfileHeader.tsx:32`의 `<h1 className="text-2xl sm:text-3xl font-bold tracking-tight">`에 `font-display` 클래스 한 단어를 추가한다.
- 그 외 동일 파일 내 다른 요소는 변경하지 않는다. ProfileHeader.tsx:50의 Geist Mono inline style은 별도 후보(`design-20260515-tags-mono-font-token-missing`)에서 처리한다.

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: `apps/web/src/components/profile/ProfileHeader.tsx` 단일 파일, 1단어 추가.
- 사용자 영향: 크리에이터 프로필 페이지 최상단 닉네임 헤딩이 시스템 sans-serif에서 General Sans로 렌더링된다. 사이트 전체의 디스플레이 헤딩이 동일 폰트로 정렬됨.
- 성능·의존성·DB 영향 없음. 폰트는 이미 layout.tsx에서 로드 중.

## Rollback

- 추가한 `font-display` 한 단어를 제거하면 즉시 이전 상태로 복귀. 단일 커밋이므로 `git revert <commit>`으로 일괄 복귀.
