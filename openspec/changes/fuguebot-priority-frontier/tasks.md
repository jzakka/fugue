## 1. Frontier 및 저장 모델 추가

- [ ] 1.1 frontier URL 상태를 저장할 테이블과 sqlc 쿼리를 추가한다
- [ ] 1.2 fetch snapshot 메타데이터와 TTL을 저장할 테이블/쿼리를 추가한다
- [ ] 1.3 Pin을 searchable document로 다루기 위한 메타데이터 저장 모델을 정의한다
- [ ] 1.4 frontier claim/update를 위한 repository 인터페이스와 구현을 추가한다
- [ ] 1.5 `normalized_url` unique index와 Pioneer claim용 partial index를 추가한다
- [ ] 1.6 Harvester claim용 partial index를 추가한다

## 2. Fetch 계층 재구성

- [ ] 2.1 object storage 우선, HTTP fallback을 지원하는 공용 fetcher를 구현한다
- [ ] 2.2 Pioneer fetch 경로를 공용 fetcher + snapshot 저장 로직으로 교체한다
- [ ] 2.3 Harvester fetch 경로를 snapshot 우선 조회 + HTTP fallback으로 교체한다
- [ ] 2.4 fetch retry, backoff, 쿼리 기반 선별 조건을 공통 유틸리티로 정리한다

## 3. Pioneer를 priority frontier worker로 전환

- [ ] 3.1 Pioneer의 입력을 site root 기반 BFS 큐에서 `last_fetched_at IS NULL` 중심 frontier claim 방식으로 변경한다
- [ ] 3.2 링크 추출 후 필터링/점수 계산 결과를 frontier upsert로 반영한다
- [ ] 3.3 allow/deny 키워드 규칙과 robots 준수 로직을 frontier 입력 단계에 추가한다
- [ ] 3.4 페이지-링크 관계 저장을 URL 단위 그래프로 갱신한다
- [ ] 3.5 Pioneer가 fetch 성공/실패에 따라 frontier 선별 컬럼을 갱신하도록 수정한다

## 4. Harvester를 document indexing worker로 전환

- [ ] 4.1 Harvester의 입력을 graph BFS에서 `pin_id IS NULL` 중심 frontier 소비 방식으로 변경한다
- [ ] 4.2 HTML에서 Pin 생성용 구조화 데이터를 추출하는 extractor 계층을 구현한다
- [ ] 4.3 content candidate 판별, Pin 생성 여부 결정, 이미 인덱싱된 URL 재수확 제외 로직을 추가한다
- [ ] 4.4 대표 이미지 캐시 저장과 searchable Pin 메타데이터 저장을 구현한다

## 5. 레거시 경로 정리 및 호환성 확보

- [ ] 5.1 site/node/script 중심 README와 CLI 설명을 priority frontier 모델로 갱신한다
- [ ] 5.2 기존 script executor 기반 경로를 fallback adapter로 격리하거나 비활성화한다
- [ ] 5.3 obsolete한 pattern merge/BFS 전용 로직을 제거하거나 legacy 모듈로 이동한다

## 6. 검증 및 운영 준비

- [ ] 6.1 frontier claim 동시성과 중복 선점 방지 테스트를 추가한다
- [ ] 6.2 snapshot TTL(기본 1년) 만료 및 HTTP fallback 동작 테스트를 추가한다
- [ ] 6.3 Pioneer/Harvester 통합 플로우 테스트를 추가한다
- [ ] 6.4 마이그레이션 절차와 롤백 포인트를 문서화한다
