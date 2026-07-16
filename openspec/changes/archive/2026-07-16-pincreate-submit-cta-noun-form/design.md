# Design — pincreate-submit-cta-noun-form

## Context

apps/web 폼 제출 CTA 6곳 중 PinCreateForm.tsx:665 "등록하기"만 ~하기형이고 나머지 5곳은 명사형이다. 발견 근거는 backlog `design-20260716-pincreate-submit-cta-noun-form` evidence 참조.

## Goals / Non-Goals

- Goal: 제출 CTA 유휴 라벨을 지배 관례(명사형)로 정렬한다.
- Non-Goal: busy 라벨("등록 중...", "처리 중...") 변경, 버튼 스타일/구조/속성 변경, 타 폼 CTA 변경.

## Decisions

### Decision 1 — 소수(1곳)를 다수(5곳)로 정렬

"등록하기" → "등록". 대안(5곳을 ~하기형으로 정렬)은 변경 폭 5배 + 지배 관례 역행이라 기각. c3745 선례(소수→다수 방향)와 동일.

### Decision 2 — 명사 어간 보존

기존 busy 라벨 "등록 중..."이 이미 어간 "등록"을 사용하므로 유휴 라벨도 "등록"으로 두면 타 표면과 동일한 "X" ↔ "X 중..." 짝이 된다. 다른 명사(예: "게시", "발행")로의 교체는 어휘 변경이라 범위 밖(보수 원칙: 기존 어간 보존).

### 변경 코드

```tsx
// apps/web/src/app/pin/new/PinCreateForm.tsx:665
{submitting ? "등록 중..." : optimizing ? "처리 중..." : "등록"}
```

## Risks

- 없음에 가까움: 문자열 1행, 레이아웃은 px-6 py-2.5 고정 패딩이라 2자 축소에도 자연 수축만 발생. QA에서 렌더 확인.
