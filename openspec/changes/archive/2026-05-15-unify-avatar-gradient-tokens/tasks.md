## 1. className 교체 (5 파일)

- [x] 1.1 `apps/web/src/components/nav/NavBar.tsx:46` — `to-orange-400` → `to-accent-hover`.
- [x] 1.2 `apps/web/src/components/nav/SearchBar.tsx:322` — `to-orange-400` → `to-accent-hover`.
- [x] 1.3 `apps/web/src/components/profile/ProfileHeader.tsx:24` — `to-orange-400` → `to-accent-hover`.
- [x] 1.4 `apps/web/src/app/pins/[id]/page.tsx:214` — `to-orange-400` → `to-accent-hover`.
- [x] 1.5 `apps/web/src/app/search/SearchClient.tsx:325` — `to-orange-400` → `to-accent-hover`.

## 2. inline 스타일 → className 일관화 (PinCard)

- [x] 2.1 `apps/web/src/components/feed/PinCard.tsx:167` — inline `style={{ background: 'linear-gradient(135deg, var(--accent), #FF8A5C)' }}` 제거 후 className에 `bg-gradient-to-br from-accent to-accent-hover` 통합.

## 3. 검증

- [x] 3.1 `grep "to-orange-400|#FF8A5C" apps/web/src --include='*.tsx' --include='*.ts'` 결과 0건.
- [x] 3.2 `grep "to-accent-hover" apps/web/src --include='*.tsx'` 결과 6건(NavBar / SearchBar / ProfileHeader / PinCard / pins[id]/page / SearchClient 각 1건).

## 4. 사후 기록

- [x] 4.1 `.fugue/decision-log.md`에 항목 1~3줄 추가.
- [x] 4.2 `.fugue/backlog-design.yaml`에서 `design-20260515-avatar-gradient-orange-400` status를 `done`으로 변경.
