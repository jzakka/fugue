# Design: PinsGrid noscript 페이지네이션 폴백 정렬

## Context

- 무한스크롤(IntersectionObserver) 표면 전수 2곳: FeedContainer(피드 `/`)·PinsGrid(프로필 `/creators/[id]`).
- FeedContainer는 JS 비활성 폴백으로 `<noscript>` 내 '다음 페이지' 링크를 제공하고(FeedContainer.tsx:224-235), 서버 페이지가 `offset` searchParam을 해석해 다음 배치를 서버 렌더한다(app/page.tsx:65-68). PinsGrid에는 두 축 모두 없다.

## Goals

- PinsGrid에 FeedContainer 동형 noscript 폴백을 추가하고, creators/[id] 서버 페이지에 offset searchParam 지원을 신설한다.
- JS 활성 사용자 경로의 시각·동작을 일절 바꾸지 않는다(보수 원칙 a).

## Decisions

### Decision 1 — 서버 페이지 offset searchParam (app/page.tsx 동형)

`creators/[id]/page.tsx`의 Props에 `searchParams: Promise<{ offset?: string }>`를 추가하고 다음과 같이 해석한다:

```tsx
const sp = await searchParams;
const offset = sp.offset ? parseInt(sp.offset, 10) || 0 : 0;
```

`fetchPins({ creator_id: id, limit: 20, offset }, { serverSide: true })`로 전달한다. 기각 대안: cursor 기반 — 피드가 offset 기반이므로 아키타입 대칭을 위해 offset을 따른다.

### Decision 2 — PinsGrid `initialOffset` prop (FeedContainer:18,42 동형)

```tsx
initialOffset = 0  // prop, optional
const offsetRef = useRef(initialOffset + initialPins.length);
```

offset=20으로 서버 렌더 시 initialPins는 21-40번째 핀이고 offsetRef는 40이 된다. 클라이언트 필터 reload는 offsetRef를 0으로 리셋하므로 영향 없음(noscript 콘텐츠는 JS 비활성 시에만 렌더되므로 클라이언트 상태 변화와 교차하지 않는다).

### Decision 3 — noscript 링크는 offset만 전달

FeedContainer는 URL 유래 필터(media_type·tags)를 noscript href에 병기하지만, PinsGrid의 미디어타입 필터는 클라이언트 상태(useState) 전용이라 JS 비활성 시 사용 자체가 불가능하다. 따라서 noscript 링크는 `?offset=N`만 전달한다(보수 원칙 d — 서버 필터 param 신설은 범위 밖). 마크업·클래스는 FeedContainer:226-233을 그대로 미러링한다:

```tsx
<noscript>
  {hasMore && (
    <div className="flex justify-center py-8">
      <a
        href={`?${noscriptParams.toString()}`}
        className="px-6 py-3 bg-surface border border-border rounded-full text-sm text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors"
      >
        다음 페이지
      </a>
    </div>
  )}
</noscript>
```

## Risks

- offset 음수/비정수 입력: `parseInt || 0` 가드로 0 폴백(app/page.tsx:68 동형).
- 회귀 위험: JS 활성 시 noscript는 브라우저가 렌더하지 않으므로 시각 회귀 없음. offset 미지정 시 기존 코드 경로와 동일(offset=0).
