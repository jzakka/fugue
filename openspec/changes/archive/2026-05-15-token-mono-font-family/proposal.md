## Why

`DESIGN.md` L20-21은 mono 폰트를 명시한다:
- L20: "Data/Tags: Geist Mono — 태그, 메타데이터, 수치에 기술적 느낌 (tabular-nums 지원)"
- L21: "Code: Geist Mono"

그러나 `apps/web/src/app/globals.css`의 `@theme inline`은 `--font-display`(직전 archive)만 토큰화하고 mono 토큰은 정의하지 않는다. 그 결과 13개 파일 23곳이 inline `style={{ fontFamily: "'Geist Mono', monospace" }}`로 동일 값을 반복한다.

23곳의 의미를 검토한 결과 모두 DESIGN.md L20 "Data/Tags" 영역(태그 칩, 핀 카운트, 시간 표시, 도메인/사이트명, 미디어 타입 배지)에 속하며 코드 표시 영역은 없다.

## Anti-pattern 검토

`.fugue/anti-patterns.md` L15는 "토큰 추가 + Tailwind 기본 클래스 의미 덮어쓰기를 단일 항목으로 묶지 마라"고 한다. 그 위험의 핵심은 **광범위 시각 회귀**다.

본 change는:
- `font-mono` 클래스 현재 사용처: `grep -rnE "\bfont-mono\b" apps/web/src` 결과 **0건**
- 즉 `--font-mono`를 `'Geist Mono', monospace`로 덮어써도 시각 회귀 트리거 면이 없음
- DESIGN.md가 mono 영역 전체를 Geist Mono로 못 박았으므로(L20-21) 시스템 monospace를 별도 의미로 쓸 시나리오 자체가 디자인 시스템 외 영역

따라서 anti-pattern L15의 적용 대상이 아니다. 분리해서 새 토큰명(`--font-tags`)을 만드는 대안은 의미를 좁히기만 하고(코드/시간/도메인 영역이 "tags" 의미와 어긋남) 디자인 정합성을 떨어뜨린다.

## What Changes

- `apps/web/src/app/globals.css`의 `@theme inline` 블록 끝에 `--font-mono: 'Geist Mono', monospace;` 한 줄을 추가한다. Tailwind v4가 자동으로 `font-mono` 유틸리티를 생성한다(또는 기본값을 이 정의로 덮는다).
- 13개 파일 23곳에서 `style={{ fontFamily: "'Geist Mono', monospace" }}` 속성을 제거하고, 같은 요소의 `className` 끝에 `font-mono`를 추가한다.

영향 파일:
- `apps/web/src/app/pins/[id]/page.tsx` (2곳)
- `apps/web/src/app/pin/new/PinCreateForm.tsx` (7곳)
- `apps/web/src/app/search/SearchClient.tsx` (3곳)
- `apps/web/src/app/boards/[id]/page.tsx` (1곳)
- `apps/web/src/components/pin/VideoTrimModal.tsx` (2곳)
- `apps/web/src/components/board/BoardGrid.tsx` (1곳)
- `apps/web/src/components/board/AddToBoardButton.tsx` (1곳)
- `apps/web/src/components/profile/MyPageClient.tsx` (1곳)
- `apps/web/src/components/profile/ProfileHeader.tsx` (1곳)
- `apps/web/src/components/feed/PinCard.tsx` (1곳)
- `apps/web/src/components/feed/TagFilter.tsx` (2곳)
- `apps/web/src/components/nav/SearchBar.tsx` (1곳)

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: globals.css 1줄 추가 + 13개 파일의 inline style 23곳 제거 + className 1단어 23개 추가. 총 14개 파일.
- 사용자 영향: 시각 변화 없음. 폰트 패밀리 값은 동일(Geist Mono, fallback monospace). inline style → CSS 클래스 경로만 바뀐다.
- 성능: 23개 요소의 inline style 객체 생성 사라짐. 폰트 파일 추가 다운로드 없음(Geist Mono CDN은 이미 layout.tsx에서 로드).
- DESIGN.md 명세 일치: L20-21 mono 폰트 토큰화 완료. `--font-display`와 함께 `@theme inline`이 폰트 시스템의 단일 진실 원천 역할.

## Rollback

- `apps/web/src/app/globals.css`에서 `--font-mono` 한 줄 제거.
- 13개 파일의 className에서 `font-mono` 제거 + 기존 inline style 복원.
- 단일 커밋이므로 `git revert <commit>`으로 일괄 복귀.
