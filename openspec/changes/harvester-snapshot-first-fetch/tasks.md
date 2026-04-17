## 1. Fetcher 인터페이스 정리

- [ ] 1.1 `apps/api/internal/bot` 하위에 공통 `Fetcher` 인터페이스(`Fetch(url) ([]byte, error)`)가 단일 위치에 정의되어 있는지 확인하고, Pioneer/Harvester가 모두 동일 인터페이스에 의존하도록 정리한다
- [ ] 1.2 기존 Harvester가 직접 HTTP 클라이언트를 호출하는 경로가 있다면 인터페이스 호출로 치환한다(파싱/스크립트 실행기 시그니처는 변경하지 않는다)

## 2. CompositeFetcher 구현

- [ ] 2.1 `CompositeFetcher` 구조체(`o ObjectStorageFetcher`, `h HTTPFetcher`)를 추가한다 (참조: `apps/api/fuguebot_pseudo.go` 라인 86-97)
- [ ] 2.2 `CompositeFetcher.Fetch(url)` 구현: ObjectStorage 조회 → 에러 시 HTTP fetch 폴백, 출처 무관하게 동일 바이트열을 반환한다
- [ ] 2.3 ObjectStorage 조회 실패 케이스(키 없음 / TTL 만료 / 저장소 에러)를 모두 동일하게 "miss"로 처리하여 폴백 분기로 라우팅한다
- [ ] 2.4 ObjectStorage 본문이 gzip 등으로 압축되어 있다면 `ObjectStorageFetcher` 내부에서 해제하여, 호출자에게는 원본 HTML 바이트열로 반환한다

## 3. Harvester 통합

- [ ] 3.1 Harvester 실행 경로(`Harvester.Run` 또는 동등 위치)에서 주입되는 Fetcher를 `CompositeFetcher`로 교체한다
- [ ] 3.2 `CompositeFetcher.Fetch`가 최종 에러를 반환하는 경우 해당 노드의 파싱을 건너뛰고 실행 통계의 `harvest_error_count`를 1 증가시킨다
- [ ] 3.3 단일 노드 실패가 다른 노드의 처리를 중단시키지 않도록 루프 제어를 검증한다

## 4. 테스트

- [ ] 4.1 단위 테스트: 스냅샷 hit 시 HTTPFetcher가 호출되지 않음을 검증한다(목 객체로 호출 횟수 확인)
- [ ] 4.2 단위 테스트: ObjectStorage가 "키 없음" 에러를 반환할 때 HTTP 폴백이 호출되어 정상 본문을 반환함을 검증한다
- [ ] 4.3 단위 테스트: ObjectStorage가 만료(expired) 에러를 반환할 때 HTTP 폴백이 호출됨을 검증한다
- [ ] 4.4 단위 테스트: ObjectStorage가 네트워크/권한 에러를 반환해도 즉시 실패하지 않고 HTTP 폴백을 시도함을 검증한다
- [ ] 4.5 단위 테스트: ObjectStorage와 HTTP 모두 실패할 때 `CompositeFetcher.Fetch`가 에러를 반환함을 검증한다
- [ ] 4.6 통합 테스트: Harvester 실행 중 한 노드의 이중 실패가 발생해도 후속 노드 처리가 계속되며, 실행 통계의 `harvest_error_count`가 정확히 1 증가함을 검증한다
- [ ] 4.7 통합 테스트: 정상 경로(스냅샷 hit)에서 외부 HTTP 트래픽이 발생하지 않음을 검증한다(HTTPFetcher 호출 카운트 = 0)

## 5. 관측성 및 검증

- [ ] 5.1 fetch 출처(snapshot/http) 및 ObjectStorage 실패 사유가 로그/메트릭으로 식별 가능한지 확인한다(운영 임계치 산정의 근거)
- [ ] 5.2 `pioneer-snapshot-storage`가 정의한 키 규약·TTL·gzip 포맷과 `ObjectStorageFetcher` 읽기 측이 정합함을 코드 리뷰로 확인한다
- [ ] 5.3 `openspec validate harvester-snapshot-first-fetch --strict`를 통과한다
