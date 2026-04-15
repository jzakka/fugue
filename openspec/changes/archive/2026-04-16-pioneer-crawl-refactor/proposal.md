## Why

Pioneer의 `crawl()` 메서드가 링크 추출, 필터링, 중복 체크를 인라인으로 처리하고 있었다. 이미 빌드된 crawler 패키지와 FilterChain을 통합하고, Harvester의 fetchHTML stub을 해소하는 리팩터링.

## What Changes

- crawl() 링크 추출: regex parseLinks() → DOM 기반 ExtractLinksWithSelectors()
- 인라인 필터 → FilterChain.Apply() (도메인, 확장자, 경로 패턴, 중복)
- classifyURL() skip 패턴 제거, regexp 패키지 레벨 추출
- 복합 우선순위: NodeTypePriority + semanticPriorityModifier
- 공유 fetchHTMLShared 추출: Pioneer/Harvester 공용
- parseLinks(), toNullString() 삭제

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `bot`: URL 제외 책임이 classifyURL → PathPatternFilter로 이전. Harvester 실제 fetch 가능.

## Impact

- 파일 변경: pioneer.go, helpers.go, harvester.go, pioneer_test.go
- 하위 호환성: 외부 API 변경 없음
