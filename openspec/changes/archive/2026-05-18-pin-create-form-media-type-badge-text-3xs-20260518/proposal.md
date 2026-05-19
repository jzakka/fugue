## Why

핀 생성 폼의 파일 업로드 후 메타 정보 영역에 표시되는 미디어 타입 배지(`mediaType` 변수 반환값 — "이미지/오디오/비디오/(원본 mime)" 한국어 카테고리 라벨)는 작품의 미디어 카테고리를 노출하는 라벨이다. DESIGN.md L35는 'tags, category labels' 카테고리에 3xs(10px)를 명시하는데, 현재 `apps/web/src/app/pin/new/PinCreateForm.tsx:388` 배지 `<span>`은 자체 text 크기 utility 없이 부모 L386 div의 `text-xs`(12px / 'creator name, meta' 카테고리)를 상속해 카테고리 매핑이 어긋난다. archive/2026-05-18-pin-detail-media-type-badge-text-3xs-20260518가 동일 카테고리(미디어 타입 배지)를 `pins/[id]/page.tsx:130`에서 정렬한 후속 잔여 1건.

## What Changes

- `apps/web/src/app/pin/new/PinCreateForm.tsx:388` 미디어 타입 배지 `<span>` className 끝에 `text-3xs` 1단어 추가
- 자식 span 자체 utility로 부모 `text-xs` 상속 끊고 배지만 10px 적용
- font-mono / px-2 / py-0.5 / bg-accent-subtle / text-accent / rounded-full 모두 유지
- 부모 L386 div utility(`text-xs text-text-muted flex items-center gap-2`) 미수정
- 부모 div의 다른 자식(파일명 L387 / trim 정보 L391-395 / 사이즈 비교 L396-402)은 부모 `text-xs` 상속 그대로 유지

## Capabilities

### New Capabilities
- `design-tokens`: DESIGN.md 타이포 스케일 카테고리(L33-35)와 코드 글자 크기 utility 매핑 정합. 미디어 타입 배지가 작품의 미디어 카테고리 라벨로 'category labels' 카테고리에 속하므로 3xs(10px) 매핑이 정확함을 행위 계약으로 명시. archive/2026-05-15-text-scale-tokens-2xs-3xs·archive/2026-05-18-pin-detail-tag-chip-text-3xs-20260518·archive/2026-05-18-pin-detail-media-type-badge-text-3xs-20260518과 동일 capability에 누적.

### Modified Capabilities
- (없음)

## Impact

- 변경 파일: `apps/web/src/app/pin/new/PinCreateForm.tsx` 1개 라인 +1 utility 추가
- 사용자 영향: 핀 생성 폼에서 파일 업로드 후 메타 정보 영역의 미디어 타입 배지(예: '이미지')가 12px → 10px로 축소되어 부모 영역의 파일명·trim·사이즈 데이터와 위계 분리 강화
- API/의존성/마이그레이션: 없음
- 토큰: `--text-3xs: 0.625rem`은 globals.css `@theme inline`에 archive/2026-05-15-text-scale-tokens-2xs-3xs에서 정의 완료, 즉시 사용 가능
- 부모 영역 회귀: 부모 utility 미수정·부모 자식 텍스트 utility 미수정으로 0
