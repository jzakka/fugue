# PinCard 카드 푸터에 `creator.avatar_url` 조건부 렌더 추가

## Backlog id

`design-20260515-pincard-avatar-url-ignored`

## 무엇을

`apps/web/src/components/feed/PinCard.tsx`의 카드 푸터 작은 아바타 원(L171 단일 div) 자리에, 다른 5곳에서 일관되게 쓰이는 조건부 패턴(`{creator.avatar_url ? <img/> : <div gradient/>}`)을 적용한다. 사이즈(w-5 h-5 = 20px)는 그대로 유지.

| 파일 | 라인 | 동작 |
|------|------|------|
| `apps/web/src/components/feed/PinCard.tsx` | 171 | 단일 div를 `{pin.creator.avatar_url ? <img src={pin.creator.avatar_url} alt="" className="w-5 h-5 rounded-full shrink-0 object-cover" loading="lazy" onError={hide} /> : <div className="w-5 h-5 rounded-full shrink-0 bg-gradient-to-br from-accent to-accent-hover" />}` 형태로 교체 |

## 왜

### DESIGN.md 근거

DESIGN.md L82 — "Image card: og_image 썸네일 + 제목 + **크리에이터 아바타**/이름 + 태그". 카드 변주 명세는 image/audio/video/text 카드의 공통 요소로 "크리에이터 아바타"를 명시한다. 현재 PinCard 카드 푸터는 image/video 카드의 푸터 영역(L154-177)에서 크리에이터 표시를 담당하지만, 실제 `pin.creator.avatar_url` 값은 한 번도 참조되지 않고 누구나 동일한 accent 그라디언트 원이 노출된다.

### 패턴 일관성 증거

`grep -rn "avatar_url" apps/web/src --include="*.tsx"` 결과 5곳에서 동일 조건부 패턴이 측정됨:

- `apps/web/src/components/nav/NavBar.tsx:39-47` — `w-9 h-9` 본인 아바타
- `apps/web/src/components/nav/SearchBar.tsx:314-322` — `w-7 h-7` 드롭다운 크리에이터 결과
- `apps/web/src/app/search/SearchClient.tsx:315-323` — `w-12 h-12` 검색 결과 크리에이터 카드
- `apps/web/src/app/pins/[id]/page.tsx:200-208` — `w-10 h-10` 핀 상세 페이지 크리에이터
- `apps/web/src/components/profile/ProfileHeader.tsx:17-22` — 크리에이터 프로필 헤더

다섯 사이트 모두 `{creator.avatar_url ? <img src={creator.avatar_url} ... /> : <div className="... bg-gradient-to-br from-accent to-accent-hover ..." />}` 구조. PinCard.tsx:171만 무조건 폴백 div만 렌더:

```tsx
<div className="w-5 h-5 rounded-full shrink-0 bg-gradient-to-br from-accent to-accent-hover" />
```

`grep "creator\." apps/web/src/components/feed/PinCard.tsx` 결과 `pin.creator.nickname` 2건만, `avatar_url` 참조 0건.

### 사용자 체감

아바타를 업로드한 크리에이터의 핀이 피드(/)·검색 결과 그리드·보드 상세 그리드 등 PinCard가 렌더되는 모든 화면에서 항상 동일한 accent 그라디언트로 표현된다. 동일 핀을 클릭해 상세(`/pins/{id}`)로 진입하면 같은 데이터에 대해 실제 아바타가 보인다(같은 데이터, 다른 표현). 피드가 본 서비스의 최고 빈도 표면이므로 크리에이터 시각 식별 비용이 커진다.

## 어디까지

### 변경 파일

- `apps/web/src/components/feed/PinCard.tsx` (L171 한 줄을 조건부 JSX로 교체, 약 3~5줄 증가)

다른 5곳 패턴 사이트는 이미 올바르므로 손대지 않는다.

### 사용자 영향

- 아바타 업로드한 크리에이터: 카드 푸터의 20px 원이 본인 아바타 이미지로 표시됨.
- 아바타 미업로드 크리에이터: 기존과 동일한 accent 그라디언트 원 폴백 유지.
- 이미지 로드 실패 시 `onError`로 img를 숨겨 그라디언트가 자연스럽게 백업되도록 한다(다른 사이트와 동일한 안전망 패턴: PinCard.tsx:166의 favicon 폴백, ProfileEditForm.tsx:90의 avatar 미리보기 폴백).
- 사이즈(w-5 h-5 = 20px)·shrink-0·rounded-full 동일 유지 → 레이아웃 시프트 없음.

### 무엇을 하지 않는가

- 다른 5곳 패턴 사이트의 사이즈를 통일하지 않는다(DESIGN.md가 avatar 사이즈 등급을 명시하지 않음 → 자의적 통일 방지).
- PinCard 카드 푸터 favicon 표시(L160-170) 변경하지 않는다.
- AudioSection(L57-78)의 크리에이터 표시는 별도 — 거기엔 아바타가 아예 없고 nickname만 있음. 별도 후보로 분리.
- alt 텍스트 정책: 카드 푸터의 작은 아바타는 nickname을 옆에 함께 노출하므로 `alt=""`(decorative)로 둔다. 동일 정책이 SearchBar.tsx:317에서 사용됨.

## 롤백

PinCard.tsx:171의 조건부 JSX를 원래 단일 div로 되돌린다. `git diff`로 단일 파일 깔끔 revert.

## Anti-pattern 검토

- **L15** (token 의미 덮어쓰기 vs 추가 분리): 해당 없음. Tailwind 기본 클래스 의미 변경 아니고, 신규 토큰 추가도 아님(조건부 렌더링 추가).
- **L16** (radius 등급 매핑 모호): 해당 없음. `rounded-full`은 DESIGN.md L77이 "avatars"에 명시한 등급(full=9999px)으로 매핑 명확.
