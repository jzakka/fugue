## Why
PinCard 컴포넌트에서 `<Link>` (→`<a>`) 안에 ExternalLinkIcon의 `<a>` 태그가 중첩되어 HTML 규격 위반. Next.js가 hydration 에러를 발생시키고, 개발 모드에서 "3 Issues" 배지가 표시된다.

## What Changes
- PinCard의 ExternalLinkIcon을 `<a>` 대신 `<button>` + `window.open()`으로 변경하거나, 카드 전체 `<Link>` 래핑을 `<div>` + onClick 핸들러로 변경

## Capabilities
### New Capabilities
_(없음)_
### Modified Capabilities
_(없음 — 기존 동작 유지, HTML 규격 준수)_

## Impact
- Frontend: PinCard.tsx 수정
