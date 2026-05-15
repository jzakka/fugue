## 1. 구현

- [x] 1.1 `apps/web/src/components/feed/PinCard.tsx:171`의 단일 그라디언트 div를 조건부 JSX로 교체. 조건이 true면 `<img src={pin.creator.avatar_url} alt="" className="w-5 h-5 rounded-full shrink-0 object-cover" loading="lazy" onError={hide} />`, false면 기존 그라디언트 div를 그대로 폴백.

## 2. 검증

- [x] 2.1 `grep -n "creator.avatar_url" apps/web/src/components/feed/PinCard.tsx` 결과 L171·L173 2건 확인.
- [x] 2.2 본 사이클은 PinCard.tsx 단일 파일만 변경됨을 git diff로 확인(다른 변경 파일은 이전 사이클 누적분).
- [x] 2.3 변경 후 카드 푸터 영역 div 구조가 favicon(L160-170) + avatar 조건부(L171-183) + nickname(L184-186) 3요소 순서를 유지함을 직접 확인.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "PinCard 푸터 아바타 조건부 렌더" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-pincard-avatar-url-ignored` 항목 status를 `done`으로 변경 + note 보강.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-pincard-avatar-url-conditional/`로 이동.
