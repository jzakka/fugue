## Why

Pioneer 크롤러의 BFS 순회 로직과 HTTP 페이지 조회 로직의 결합도를 끊어서 단위 테스트가 가능하도록 리팩토링한다. 현재 Pioneer는 BFS 탐색, HTTP 조회, 페이지 분류가 강하게 결합되어 있어 각 로직을 독립적으로 테스트하기 어렵다. Fetcher 인터페이스를 도입하여 BFS 알고리즘과 페이지 조회 방법을 분리하고, 테스트 시에는 파일 시스템 기반 fixture를 사용하고 프로덕션에서는 HTTP 클라이언트를 사용하도록 한다.

## What Changes

- BFS 순회 로직을 독립적인 `Crawler` 컴포넌트로 추출
- `Fetcher` 인터페이스를 도입하여 페이지 조회 방법을 추상화
- 테스트용 `FileFetcher` 구현 (파일 시스템 기반 HTML fixture 사용)
- 프로덕션용 `HTTPFetcher` 구현 (실제 HTTP 요청)
- 테스트 HTML fixture를 생성하여 BFS 동작을 단위 테스트로 검증
- 페이지 타입 분류 로직은 기존 Pioneer 코드에 그대로 유지 (이번 변경 범위 아님)

## Capabilities

### New Capabilities
<!-- 없음 - 기존 기능의 리팩토링 -->

### Modified Capabilities
- `bot-pioneer-crawler`: BFS 탐색 로직을 독립 컴포넌트로 추출하고 Fetcher 인터페이스를 통해 페이지 조회 방법을 추상화하여 단위 테스트 가능하게 개선

## Impact

- `apps/api/internal/bot/crawler/` 패키지 추가 (독립 컴포넌트)
- `apps/api/internal/bot/pioneer.go` 수정 (Crawler 컴포넌트 사용하도록 리팩토링)
- 단위 테스트용 testdata HTML fixture 파일 추가
- 기존 Pioneer의 외부 동작은 변경되지 않음 (내부 구조 개선)
