## 1. srcset 첫 후보 URL 파싱 구현

- [x] 1.1 `apps/api/internal/bot/extractor.go`에 `firstSrcsetURL(raw string) string` 헬퍼 추가 — 콤마로 첫 후보 분리 후 `strings.Fields`로 첫 토큰 추출, 비면 빈 문자열 반환
- [x] 1.2 `handleSource`의 `src == ""` 분기에서 `getAttr(n, "srcset")` 원문 대신 `firstSrcsetURL(getAttr(n, "srcset"))` 사용
- [x] 1.3 `src` 속성이 있는 경우, video/audio `<source>`, 디스크립터 없는 단일 URL srcset의 기존 동작 보존 확인

## 2. 테스트

- [x] 2.1 `extractor_test.go`에 `<picture><source srcset="a.webp 1x, b.webp 2x" type="image/webp"><img src="fallback.jpg"></picture>` fixture 케이스 추가 — `media_candidates`에 절대화된 첫 후보 URL(`.../a.webp`)이 들어가고 디스크립터/콤마가 없음을 검증
- [x] 2.2 회귀: `src` 있는 `<source>`, 단일 URL srcset(디스크립터 없음), video/audio `<source>` 케이스가 기존과 동일하게 수집되는지 검증
- [x] 2.3 `cd apps/api && go vet ./... && go build ./... && go test ./...` 통과

## 3. 실 환경 QA

- [x] 3.1 fixture HTML을 실제 HTTP 서버(httptest)로 서빙하고 production `GenericExtractor.Extract`로 페치+추출하여 `media_candidates`에 유효 절대 URL만 수집됨을 관찰(수정 전 깨진 URL 재현 → 수정 후 교정)
- [x] 3.2 회귀: 기존 미디어 수집 케이스(`<img>`/`<video>` src) 무회귀 확인
