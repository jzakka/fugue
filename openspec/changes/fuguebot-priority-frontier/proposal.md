## Why

현재 fuguebot은 사이트 단위 그래프와 사이트별 파싱 스크립트 생성을 중심으로 설계되어 있어, 웹 전체를 느슨하게 탐색하며 고가치 페이지를 선별하는 크롤러로 확장하기 어렵다. `apps/api/fuguebot_pseudo.go`가 제안한 것처럼 Pioneer와 Harvester를 공용 우선순위 frontier 위에서 동작하게 재정의해야 탐색, 재시도, 병렬 처리, 인덱싱을 같은 모델로 운영할 수 있다.

## What Changes

- **BREAKING** Pioneer의 역할을 “사이트맵 순회 + 스크립트 생성”에서 “URL fetch, 링크 추출, 필터링, frontier 업데이트, raw fetch 결과 상태 관리”로 재정의한다.
- **BREAKING** Harvester의 역할을 “Pioneer가 만든 사이트 그래프 BFS 순회 + 스크립트 실행”에서 “frontier에서 수확 대기 URL을 선별, Pin을 검색 document로 생성/갱신, 이미지 캐시 저장”으로 재정의한다.
- **BREAKING** 탐색 계약에서 BFS/DFS를 제거하고, 점수와 인덱스 친화적인 선별 조건을 가진 priority frontier 기반 스케줄링으로 변경한다.
- Pioneer와 Harvester가 fetch 결과 상태를 공유하도록 하여, 저장소 캐시 우선 fetch와 HTTP fallback fetch를 지원한다.
- raw HTML은 무기한 보관을 전제하지 않고, 장기 TTL fetch snapshot과 Pin/링크/메타데이터를 분리해 저장한다.
- 사이트 경계는 스케줄링 힌트로만 사용하되, 간단한 allow/deny 키워드 규칙과 `robots.txt` 준수 하에서 사이트 간 링크 발견을 허용한다.

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `bot`: Pioneer/Harvester의 책임, frontier 스케줄링, fetch 공유, 수확 및 인덱싱 행위를 priority-frontier 모델로 변경

## Impact

- 파일 변경: `apps/api/internal/bot/` 전반, 특히 `pioneer.go`, `harvester.go`, repository/db layer, crawler fetcher 계층
- 데이터 모델 변경: 기존 site/node/edge 중심 구조에서 frontier 선별 조건, fetch snapshot, Pin 검색 메타데이터 중심 모델 추가 또는 재구성
- 운영 영향: CLI 실행 방식, 병렬 worker 설계, 인덱스 설계, 저장 비용 정책(raw HTML TTL) 재정의
