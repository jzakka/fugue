## Why

DESIGN.md L19 'UI/Labels: Pretendard Variable 500'은 라벨 카테고리에 weight 500을 직접 명시한다. 그러나 `apps/web/src` 안의 `<label>` HTML 요소 8곳(PinCreateForm 5곳·VideoThumbnailPicker 1곳·ProfileEditForm 2곳)이 모두 `className="block text-sm text-text-muted mb-2"`로 weight를 명시하지 않아 Pretendard body 기본 weight(400)로 렌더링된다. 라벨/본문 위계가 동급으로 약화되어 폼 입력 위치 식별성이 감소한다. 사이클 21(display)·23(data) 카테고리 토큰화가 정착시킨 'DESIGN.md 명시 weight를 코드 SSoT에 반영' 패턴의 잔여 갭(labels 카테고리).

## What Changes

- `<label>` HTML 요소 8곳의 className에 `font-medium` 단어 1개씩 추가하여 weight 400 → 500 정렬.
- 매핑 범위는 `<label>` HTML 요소로 한정(aria-label·메뉴 라벨·버튼 텍스트 등 광의 'UI labels' 해석은 본 변경 범위 밖, 자의성 회피).
- 시각 변화: 폼 라벨 텍스트의 stroke가 약 100 단계 굵어짐. font-family·크기·색상은 그대로 Pretendard Variable / text-sm / text-text-muted 유지.
- 비포함: `<label>` 외의 텍스트 라벨(태그 칩 텍스트, 카테고리 칩, 카드 메타데이터 등)은 별도 카테고리 매핑이 필요하므로 본 변경에서 제외.

## Capabilities

### New Capabilities
- `design-tokens`: DESIGN.md의 텍스트 카테고리(Display·Body·UI/Labels·Data/Tags)별 weight를 UI 컴포넌트에 일관 적용하는 토큰 매핑 규칙. 본 변경에서는 라벨 카테고리(L19) 매핑을 코드 SSoT에 도입한다.

### Modified Capabilities
- (없음): 외부 관찰 가능한 행위(이벤트·API 응답·라우팅·상호작용 결과)는 변동 없다. DESIGN.md 명시 시각 토큰을 코드에 반영하는 변경.

## Impact

- 영향 파일 3개(`apps/web/src/app/pin/new/PinCreateForm.tsx`, `apps/web/src/components/pin/VideoThumbnailPicker.tsx`, `apps/web/src/components/profile/ProfileEditForm.tsx`) 총 8라인.
- 영향 페이지: `/pin/new`(핀 생성 폼), `/mypage/edit` 또는 ProfileEditForm 노출 경로(프로필 편집 폼).
- API·DB·라우팅 변경 없음.
- 의존성 변경 없음(Tailwind v4 기본 `font-medium` 유틸리티 사용).
- 롤백 절차: 단일 커밋 `git revert`.
