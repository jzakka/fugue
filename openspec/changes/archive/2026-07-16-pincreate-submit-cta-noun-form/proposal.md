# 제출 CTA 어형 정렬 — PinCreateForm "등록하기" → "등록"

## Why

폼 제출 CTA census 결과 어형이 갈린다:

- **명사형 5곳 (지배 관례)**: "저장" (BoardActions.tsx:100, ProfileEditForm.tsx:129), "생성" (MyPageClient.tsx:124), "생성 및 추가" (AddToBoardButton.tsx:386), "선택 완료" (VideoTrimModal.tsx:307)
- **~하기형 1곳 (이탈)**: "등록하기" (PinCreateForm.tsx:665)

부수 논거 — busy 라벨 짝 대칭: 타 표면은 "X" ↔ "X 중..." 짝(저장↔저장 중..., 생성↔생성 중...)인데 본 표면만 "등록하기" ↔ "등록 중..." 비대칭. "등록"으로 정렬 시 짝이 복원된다.

DESIGN.md는 CTA 어형을 규정하지 않으나, role-identical 카피의 지배 관례 정렬은 루프 범위(디자인 시스템 일관성)이며 c3737(어체 정합 PR #4730)·c3745(실패 문형 정렬 PR #4736) 선례와 동일 계열이다. c24593 버튼 순서 FP 판정(컨텍스트-상관 분기·양립 관례)과 달리 본 건은 동일 클래스(폼 제출 CTA) 내 무상관 단독 이탈이다.

## What Changes

- `apps/web/src/app/pin/new/PinCreateForm.tsx:665` 제출 버튼 유휴(idle) 라벨 "등록하기" → "등록" (1행, 문자열만)

## Capabilities

### Modified: pin

핀 생성 폼 제출 버튼 라벨이 서비스 공통 제출 CTA 명사형 관례를 따른다.

## Impact

- 사용자 영향: /pin/new 제출 버튼 라벨 2자 축소. 기능·레이아웃·busy 상태("등록 중...", "처리 중...")·disabled·aria-busy 무변경.
- 롤백: 커밋 revert로 즉시 복원 가능 (문자열 1행).
