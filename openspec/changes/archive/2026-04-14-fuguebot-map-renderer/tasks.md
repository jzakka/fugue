## 1. 노드 타입 통합

- [x] 1.1 `domain.go`에서 NodeType 상수를 list/detail/skip 3종으로 정리. Priority에서 gallery/category case를 list로 통합
- [x] 1.2 `pioneer.go`의 classifyURL에서 gallery/category 패턴 매칭의 반환값을 list로 변경
- [x] 1.3 `pioneer_test.go`의 분류 테스트 기대값을 list로 업데이트
- [x] 1.4 `go test ./internal/bot/...` 통과 확인

## 2. 사이트별 필터링 UI

- [x] 2.1 `template.html`의 stats-panel에 사이트 리스트 UI 추가: 라디오 버튼 생성. 각 항목에 이름(domain에서 TLD 제거)과 domain 표시
- [x] 2.2 사이트 선택 시 필터링 로직 구현: 선택된 site_id로 노드/엣지 필터링, D3 simulation 재시작
- [x] 2.3 Stats (Nodes, Edges)를 필터링된 카운트로 업데이트
- [x] 2.4 페이지 로드 시 첫 번째 사이트 자동 선택

## 3. Coverage 삭제

- [x] 3.1 `template.html`에서 coverage 관련 UI 요소 및 CSS 제거
- [x] 3.2 `types.go`에서 coverage 관련 struct 및 필드 제거
- [x] 3.3 `repository.go`에서 coverage 집계 함수 및 로직 제거. script 존재 확인은 유지
- [x] 3.4 `main.go`에서 coverage 콘솔 출력 및 site 필터 시 coverage 재계산 호출 제거

## 4. DB 기존 데이터 호환

- [x] 4.1 `template.html`의 색상/필터링 로직에서 listing, gallery, category 타입을 모두 list와 동일하게 취급

## 5. 색상 체계 및 Legend 업데이트

- [x] 5.1 `template.html`의 노드 색상을 list(파란색)/detail(초록색) 2종으로 변경
- [x] 5.2 Legend 패널을 List/Detail + Script Status로 업데이트

## 6. 검증

- [x] 6.1 `go build ./...` 컴파일 에러 없음 확인
- [ ] 6.2 `make show-map` 실행 후 사이트 필터링 동작 확인 (수동)
