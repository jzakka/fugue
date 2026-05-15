## 1. import 추가

- [x] 1.1 `Link from "next/link"` import 추가.

## 2. 최근 검색 아이템 button화

- [x] 2.1 L184-231 외곽 `<div>`를 검색어 button + 삭제 sibling button 구조로 재구성. 외곽 wrapper div는 group hover 컨테이너로 유지. 검색어 영역의 svg+span을 `<button type="button" onClick={() => { setQuery(q); handleSubmit(q); }}>`로 감싼다. 삭제 X 버튼은 그대로 sibling.

## 3. 결과 아이템 Link 교체

- [x] 3.1 L254-296 핀 결과 div → `<Link href={"/pins/" + pin.id} onClick={() => setOpen(false)}>`.
- [x] 3.2 L306-328 크리에이터 결과 div → `<Link href={"/creators/" + creator.id} onClick={() => setOpen(false)}>`.
- [x] 3.3 L336-372 보드 결과 div → `<Link href={"/boards/" + board.id} onClick={() => setOpen(false)}>`.

## 4. 전체 결과 보기 button화

- [x] 4.1 L378-384 div → `<button type="button" onClick={() => handleSubmit()} className="...">전체 결과 보기</button>`. block 레이아웃 보존을 위해 `w-full block` 추가.

## 5. 검증

- [x] 5.1 grep으로 `<div\s+onClick` 패턴이 SearchBar.tsx에 남아있지 않음 확인.
- [x] 5.2 변경된 파일이 SearchBar.tsx 단일임을 git diff로 확인.

## 6. 사후 기록

- [x] 6.1 `.fugue/decision-log.md`에 "SearchBar 드롭다운 키보드 a11y 정렬" 항목 1~3줄 추가.
- [x] 6.2 `.fugue/backlog-design.yaml`에서 항목 status를 `done`으로 + note 추가.
- [x] 6.3 change 디렉토리를 archive로 이동.
