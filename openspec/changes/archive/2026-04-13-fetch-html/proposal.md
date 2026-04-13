## 왜 필요한가

Pioneer 크롤러의 `fetchHTML()` 메서드가 현재 "not implemented" 에러를 반환하는 스텁 상태입니다. 이로 인해 모든 URL을 건너뛰어 그래프 노드를 전혀 생성하지 못합니다. 실제 크롤링을 가능하게 하려면 타임아웃, user-agent, 리다이렉트 처리를 포함한 HTTP fetching 구현이 필요합니다.

## 무엇이 바뀌는가

- `apps/api/internal/bot/pioneer.go`의 `fetchHTML()` 메서드를 실제 HTTP 클라이언트로 구현
- 설정 가능한 타임아웃 (기본 10초), user-agent, 최대 리다이렉트 횟수 추가
- 일반적인 HTTP 에러 (404, 500, 타임아웃) 우아하게 처리
- 기본 HTML 검증 (응답 본문 비어있지 않은지)
- Pioneer가 실제로 사이트를 크롤하고 그래프 노드를 구축할 수 있게 함

## 영향 받는 Capability

### 새 Capability
<!-- 새 capability 없음 - 기존 bot-pioneer-crawler의 Fetcher 인터페이스 구현 -->

### 수정되는 Capability
- `bot-pioneer-crawler`: HTTPFetcher 구현 (타임아웃, 리다이렉트, 상태 코드 처리, 응답 검증)을 통해 기존 Fetcher 인터페이스 요구사항 충족

## 영향 범위

- **코드**: `apps/api/internal/bot/pioneer.go` (fetchHTML 메서드)
- **의존성**: Go 표준 라이브러리 `net/http` 사용 (새 외부 의존성 없음)
- **동작**: Pioneer가 이제 조용히 실패하는 대신 실제로 사이트를 크롤할 수 있음
- **테스트**: 실제 URL (unsplash.com) 및 에러 케이스 (타임아웃, 404) 테스트 필요
