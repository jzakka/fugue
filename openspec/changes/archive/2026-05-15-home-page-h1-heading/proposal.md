# 메인 피드 페이지(`/`)에 페이지 단위 h1 헤딩 부여

## Backlog id

`design-20260515-home-page-h1-missing`

## 무엇을

`apps/web/src/app/page.tsx`의 `<main>` 첫 자식으로 시각적으로 숨겨진 `<h1 className="sr-only">작품 피드</h1>`를 추가한다.

```tsx
<main className="flex-1 pb-12">
  <h1 className="sr-only">작품 피드</h1>
  <Suspense>
    <FeedContainer ... />
  </Suspense>
</main>
```

## 왜

### 표준 근거

- **WCAG 2.1 SC 2.4.6 Headings and Labels (Level AA)** — "Headings and labels describe topic or purpose." 각 페이지의 주제는 헤딩으로 식별 가능해야 한다.
- **WAI-ARIA Authoring Practices — Heading Hierarchy** — 각 페이지는 페이지 주제를 식별하는 단일 h1 헤딩을 갖는 것이 권장된다. 스크린리더의 "headings rotor"가 페이지 주제를 첫 항목으로 안내할 수 있게 한다.

### 루프 정체성 근거

`prompts/loop-design.md` L7 — "apps/web/ 안의 ... 접근성(대비, 키보드 포커스, aria)" in-scope.

### 코드베이스 outlier 측정

다른 7개 페이지는 모두 페이지 단위 h1을 갖는다:

| 페이지 | h1 위치 | 텍스트 패턴 |
|------|------|------|
| `/pins/[id]` | `pins/[id]/page.tsx:135` | `pin.title` (엔티티 이름) |
| `/boards/[id]` | `boards/[id]/page.tsx:56` | `board.name` (엔티티 이름) |
| `/search` | `search/SearchClient.tsx:232` | "검색 결과" (페이지 주제) |
| `/pin/new` | `pin/new/PinCreateForm.tsx:314` | "새 핀 만들기" (페이지 액션) |
| `/mypage`·`/creators/[id]` | `profile/ProfileHeader.tsx:32` | `creator.nickname` (엔티티 이름) |
| `/login` | `login/page.tsx:41` | "로그인" (페이지 액션) |

`/` 피드 페이지만 `page.tsx`/`NavBar`/`FieldFilter`/`TagFilter`/`FeedContainer` 전체에 h1 0건. 1/8 outlier.

### h1 텍스트 결정 근거

"작품 피드"는 다른 페이지의 h1 텍스트 패턴(페이지 주제 명사)에 부합한다. "피드" 단독은 컨텍스트가 짧아 페이지 식별성이 약하고, "Fugue"는 NavBar 로고 텍스트와 중복된다. "추천 작품"은 알고리즘 추천만 표현해 정확성이 약하다(현재 피드는 단순 시간순 + 필터 조합). 사이트 도메인(`fugue/CLAUDE.md` L1 "크로스미디어 창작물 큐레이션 플랫폼") + 피드 시멘틱을 한 단어로 압축한 "작품 피드"를 선택.

### 사용자 영향

- **스크린리더(VoiceOver/NVDA/JAWS)**: "Web Item Rotor → Headings"로 페이지 진입 시 첫 헤딩이 "작품 피드"로 안내되어 페이지 주제를 즉시 인지. 현재는 NavBar 자식 헤딩(없음) 또는 페이지 콘텐츠 본문부터 나열되어 페이지 주제가 헤딩 트리에 부재.
- **시각 사용자**: 변경 없음. `sr-only` 유틸리티(Tailwind v4 기본 제공)는 `clip: rect(0,0,0,0); position: absolute; ... overflow: hidden;`로 시각적으로 완전히 숨김.
- **DESIGN.md L11 "Minimal — 타이포그래피와 여백" 의도**: 시각적으로 보이지 않으므로 의도 침범 없음.

## 어디까지

### 변경 파일

- `apps/web/src/app/page.tsx` — `<main>` 첫 자식으로 `<h1 className="sr-only">작품 피드</h1>` 한 줄 추가.

총 1줄 추가.

### 무엇을 하지 않는가

- **다른 페이지의 h1 텍스트 변경** — 7개 페이지의 기존 h1은 그대로 유지.
- **h2/h3 위계 정리** — 페이지 내부의 다음 위계 헤딩(예: TagFilter의 "인기 태그", FeedContainer의 카드 그룹) 표현은 별도 영역. 본 사이클은 h1 단일 추가에 한정.
- **시각적 h1 노출** — DESIGN.md L11 "Minimal" 의도와 충돌 가능성으로 `sr-only` 패턴만 사용. 시각적 페이지 타이틀이 디자인 의도라면 별도 후보로 분리.
- **media_type/tag 필터 적용 시 동적 h1 텍스트** — 복잡도 상승, 현재 다른 페이지도 정적 텍스트만 사용 중. 본 사이클 범위 아님.

## 롤백

`<h1 className="sr-only">작품 피드</h1>` 한 줄을 제거. `git diff`로 명확히 revert 가능.

## Anti-pattern 검토

- **L15** (token 의미 덮어쓰기 vs 추가 분리): N/A. `sr-only`는 Tailwind v4 기본 유틸리티(`overflow: hidden; clip: rect(0,0,0,0); ...`), 별도 토큰 정의 없이 즉시 사용 가능.
- **L16** (radius 등급 매핑 모호): N/A. radius 결정 아님.
