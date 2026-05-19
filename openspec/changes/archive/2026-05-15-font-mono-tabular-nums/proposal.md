# font-mono 사용처에 tabular-nums 일괄 활성화

## 배경

DESIGN.md L20은 모노 폰트의 역할을 다음과 같이 정의한다:

> Data/Tags: Geist Mono — 태그, 메타데이터, 수치에 기술적 느낌 (tabular-nums 지원)

직전 archive change `2026-05-15-token-mono-font-family`가 23곳의 inline `fontFamily: "'Geist Mono', monospace"`를 `font-mono` 유틸리티로 정리했으나, `font-variant-numeric: tabular-nums` 활성화는 누락되어 있다. 사전 grep `tabular-nums|font-variant-numeric` 결과 0건.

영향을 받는 숫자 표시 위치(현재 비례 폭으로 렌더링됨):
- `BoardGrid.tsx:29` / `AddToBoardButton.tsx:290` / `MyPageClient.tsx:136` / `ProfileHeader.tsx:48` / `boards/[id]/page.tsx:65` — `{pin_count} pins`
- `VideoTrimModal.tsx:144,202` — 시간 표시 (`fmt(start)` / `fmt(end)` / `fmt(videoDuration)`)
- `SearchClient.tsx:328` — `created_at` 날짜 표시

## 변경 범위

`apps/web/src/app/globals.css`에 1개 CSS 룰 추가:

```css
.font-mono {
  font-variant-numeric: tabular-nums;
}
```

`@theme inline` 블록 밖에 두는 이유: Tailwind v4의 `@theme inline`은 `--font-*` 토큰만 `font-family`로 자동 매핑하며 `font-variant-numeric` 같은 보조 속성은 토큰 시스템 밖. `.font-mono` 클래스에 직접 룰을 부착해 유틸리티 사용처 모두에 일관 적용.

## 사용자 영향

- 시각: pin 카운트 / 시간 / 날짜의 자릿수가 균등 폭으로 정렬되어 리스트에서 "1 pins" vs "10 pins" vs "100 pins" 같은 흔들림이 사라진다.
- 비숫자 사용처(태그 칩 등): Geist Mono가 본래 모노스페이스 폰트라서 가시 차이가 거의 없음.

## anti-pattern L15 검토

L15는 "토큰 추가 + Tailwind 기본 클래스 의미 덮어쓰기를 단일 항목으로 묶지 마라"다. 본 변경은:
- 새 토큰을 추가하지 않는다(@theme 블록 외부 CSS 룰).
- `.font-mono`는 이미 직전 archive change가 `Geist Mono`로 의미를 정렬했기 때문에, 이번에 보조 속성을 부착하는 것은 같은 의미를 강화할 뿐 새로운 회귀를 도입하지 않는다.
- 사전 grep `\bfont-mono\b` 사용처 23곳 모두 Geist Mono 의도 → tabular 숫자 활성화가 의도와 충돌하지 않음.

L15 적용 대상 아님.

## 롤백 절차

`apps/web/src/app/globals.css`에서 `.font-mono { font-variant-numeric: tabular-nums; }` 블록 제거.
