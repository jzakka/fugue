# proposal

## why
- VideoTrimModal(`apps/web/src/components/pin/VideoTrimModal.tsx`)은 modal dialog인데 열려 있는 동안 배경 body 스크롤이 차단되지 않는다. wheel/trackpad/터치로 스크롤하면 모달 뒤 `/pin/new` 폼이 같이 움직여 모달의 격리 의도와 충돌한다.
- 동일한 모달 컴포넌트인 `apps/web/src/components/board/AddToBoardButton.tsx:116-122`은 `useEffect(() => { document.body.style.overflow = 'hidden'; return () => { document.body.style.overflow = ''; }; }, [])` 패턴으로 body scroll을 잠근다. 두 모달 모두 사이클 25에서 `role="dialog"` 트리플(`role`·`aria-modal`·`aria-labelledby`)을 부여받았는데 dismiss 키보드 경로는 사이클 56에서, body scroll lock은 본 사이클에서 후속 보강한다.
- 사이클 56 archive(`openspec/changes/archive/2026-05-15-video-trim-modal-esc-dismiss`)의 backlog note L628이 "body scroll lock(AddToBoardButton L116-122)·overlay click 닫기 보강은 별도 후보로 분리"라고 본 후보를 명시적으로 예약했다.

## what
- VideoTrimModal에 body scroll lock useEffect 한 블록 추가 (AddToBoardButton L116-122 패턴 1:1 복제).

## scope
- 변경 파일: `apps/web/src/components/pin/VideoTrimModal.tsx` 1개.
- 변경 범위: useEffect 블록 1개 추가 (5줄). deps `[]`.
- 비포함: overlay click 닫기, focus trap, initial focus, AddToBoardButton 측 변경.

## references
- WAI-ARIA Authoring Practices Guide Dialog (Modal) Pattern — modal이 열려 있을 때 배경 콘텐츠 비활성화.
- 코드 SSoT 선례: `apps/web/src/components/board/AddToBoardButton.tsx:116-122`.
- 사이클 56 archive note(`.fugue/backlog-design.yaml` L628): 별도 후보 분리 선포.
