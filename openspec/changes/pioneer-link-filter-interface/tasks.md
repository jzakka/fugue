## 1. crawler 패키지 — Link 타입 정의

- [ ] 1.1 `apps/api/internal/bot/crawler/link.go` 파일 생성. `package crawler` 선언과 파일 헤더 주석 추가.
- [ ] 1.2 `Selector` 구조체 정의: `TagName string`, `ID string`, `Class string` 필드. GoDoc 주석으로 DOM 요소의 태그명/ID/Class를 표현함을 명시.
- [ ] 1.3 `Link` 구조체 정의: `URL string`, `Selectors []Selector` 필드. GoDoc 주석으로 `Selectors`가 DOM 루트에서 `<a>` 태그까지의 조상 경로임을 명시.

## 2. bot 패키지 — LinkFilter 인터페이스 및 FilterChain 정의

- [ ] 2.1 `apps/api/internal/bot/link_filter.go` 파일 생성. `package bot` 선언과 `crawler` 패키지 import (`github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler`).
- [ ] 2.2 `LinkFilter` 인터페이스 정의: `Filter(links []crawler.Link) []crawler.Link` 단일 메서드. GoDoc 주석으로 필터 계약을 설명.
- [ ] 2.3 `FilterChain` 구조체 정의: `filters []LinkFilter` 필드 (unexported).
- [ ] 2.4 `NewFilterChain(filters ...LinkFilter) *FilterChain` 생성자 함수 구현.
- [ ] 2.5 `func (c *FilterChain) Apply(links []crawler.Link) []crawler.Link` 메서드 구현. 등록된 필터를 순차 적용하고, 필터가 없으면 입력을 그대로 반환. nil 입력 시 안전하게 nil 반환.

## 3. 검증

- [ ] 3.1 `go build ./internal/bot/crawler/...` 및 `go build ./internal/bot/...` 성공 확인. 컴파일 에러 없음을 검증.
- [ ] 3.2 `go vet ./internal/bot/crawler/...` 및 `go vet ./internal/bot/...` 경고 없음 확인.
- [ ] 3.3 `go.mod`의 module path(`github.com/chungsanghwa/fugue/apps/api`)와 import path 일치 여부 확인
