<!--
Status: Section 1~5 항목은 본 change의 in-scope 작업이며 archive 판정 대상이다. 현재 Section 1~5 모든 태스크(Task 4.10 Pioneer 쓰기 → Harvester 읽기 round-trip 통합 테스트, Task 5.2 정적 검사 (a)(b)(c) 포함)가 구현 완료 상태이며 archive 판정 준비가 된다. Section 6은 본 change가 유발한 인접 문서 불일치를 추적하기 위한 후속 포인트이며 archive 체크 대상이 아니다(항목이 `[ ]`로 남아 있어도 본 change archive를 블록하지 않는다). 설계 수정이 발생하면 해당 항목을 `[ ]`로 내리고 재구현 후 다시 `[x]`로 올린다.
-->

## 1. Fetcher 인터페이스 정리

- [x] 1.1 `apps/api/internal/bot` 하위에 공통 `Fetcher` 인터페이스(`Fetch(url) ([]byte, error)`)가 단일 위치에 정의되어 있는지 확인하고, Harvester가 동일 인터페이스에 의존하도록 정리한다. Pioneer는 별도 시그니처를 유지하되 HTTP 경계 helper만 공유한다 (선행: 없음)
- [x] 1.2 기존 Harvester가 직접 HTTP 클라이언트를 호출하는 경로가 있다면 인터페이스 호출로 치환한다(파싱/스크립트 실행기 시그니처는 변경하지 않는다) (선행: Task 1.1)

## 2. CompositeFetcher 구현

- [x] 2.1 `CompositeFetcher` 구조체(`o ObjectStorageFetcher`, `h HTTPFetcher`)를 추가한다. 참조: `apps/api/fuguebot_pseudo.go` 라인 97–112 (선행: Task 1.1)
- [x] 2.2 `CompositeFetcher.Fetch(url)` 구현: ObjectStorage 조회 → 에러 시 HTTP fetch 폴백, 출처 무관하게 동일 바이트열을 반환한다 (선행: Task 2.1)
- [x] 2.3 ObjectStorage 조회 실패 케이스(키 없음 / 만료 / 네트워크 / 권한 / 내부 에러)를 모두 동일하게 단일 "사용 불가"로 처리하여 폴백 분기로 라우팅한다. 에러 종류는 로그/메트릭에만 기록하고 fetch 동작에는 영향을 주지 않는다 (선행: Task 2.2)
- [x] 2.4 `ObjectStorageFetcher` 내부에서 스냅샷 본문의 **압축 해제**를 수행하여, 호출자(CompositeFetcher/Harvester)에게는 압축되지 않은 원본 HTML 바이트열만 반환한다 (선행: Task 2.1)
- [x] 2.5 `ObjectStorageFetcher`가 스냅샷 키를 계산할 때 `apps/api/internal/bot/snapshot` 패키지의 공용 함수(`SnapshotKey`, `HashNormalizedURL`)를 import해 재사용한다. Harvester 쪽에서 키 포맷·해시 함수를 재구현하지 않는다. **또한 `SnapshotKey`의 첫 인자(normalized URL)는 Pioneer 쓰기 경로가 동일 시점에 사용하는 URL 정규화 함수의 출력과 비트 단위로 동일해야 한다 — 즉 쓰기 경로가 참조하는 정규화 심볼을 Harvester도 그대로 호출해야 하며, 서로 다른 정규화 함수로 대체하지 않는다(design.md Decision 5 보강 참조)** (선행: Task 2.1)
- [x] 2.6 스냅샷 키의 시간 세그먼트는 `time.Now().UTC()`를 사용하도록 구현하며, 테스트 주입 가능하도록 clock을 분리한다(디자인 Decision 5a) (선행: Task 2.5)

## 3. Harvester 통합

- [x] 3.1 Harvester 실행 경로(`Harvester.Run` 또는 동등 위치)에서 주입되는 Fetcher를 `CompositeFetcher`로 교체한다 (선행: Task 2.2)
- [x] 3.2 `CompositeFetcher.Fetch`가 최종 에러를 반환하는 경우 해당 노드의 파싱을 건너뛰고 워커 실행 통계(in-memory)의 fetch 실패 카운터를 1 증가시킨다. 이 카운터는 `harvester_frontier.harvest_error_count` DB 컬럼과는 구분되는 별개 집계다(design.md Decision 3 참조) (선행: Task 3.1)
- [x] 3.3 단일 노드 실패가 다른 노드의 처리를 중단시키지 않도록 루프 제어를 검증한다 (선행: Task 3.2)

## 4. 테스트

- [x] 4.1 단위 테스트: 스냅샷 hit 시 HTTPFetcher가 호출되지 않음을 검증한다(목 객체로 호출 횟수 확인) (선행: Task 2.2)
- [x] 4.2 단위 테스트: ObjectStorage가 "키 없음" 에러를 반환할 때 HTTP 폴백이 호출되어 정상 본문을 반환함을 검증한다 (선행: Task 2.3)
- [x] 4.3 단위 테스트: ObjectStorage가 만료 에러를 반환할 때 HTTP 폴백이 호출됨을 검증한다 (선행: Task 2.3)
- [x] 4.4 단위 테스트: ObjectStorage가 네트워크/권한/내부(5xx) 에러를 반환해도 즉시 실패하지 않고 HTTP 폴백을 시도함을 검증한다 — 즉 모든 실패 유형이 동일하게 "사용 불가"로 처리됨을 확인한다 (선행: Task 2.3)
- [x] 4.5 단위 테스트: 압축된 스냅샷 본문이 `ObjectStorageFetcher` 내부에서 해제되어 호출자에게는 원본 HTML 바이트열로 반환됨을 검증한다 (선행: Task 2.4)
- [x] 4.6 단위 테스트: ObjectStorage와 HTTP 모두 실패할 때 `CompositeFetcher.Fetch`가 에러를 반환함을 검증한다 (선행: Task 2.2)
- [x] 4.7 단위 테스트: 동일한 UTC 일자에 쓰인 스냅샷은 hit하고, 다른 UTC 일자 키 입력은 miss로 처리되어 HTTP 폴백으로 수렴함을 clock 주입으로 검증한다(Decision 5a) (선행: Task 2.6)
- [x] 4.8 통합 테스트: Harvester 실행 중 한 노드의 이중 실패가 발생해도 후속 노드 처리가 계속되며, 워커 실행 통계의 fetch 실패 카운터가 정확히 1 증가함을 검증한다 (선행: Task 3.3)
- [x] 4.9 통합 테스트: 정상 경로(스냅샷 hit)에서 외부 HTTP 트래픽이 발생하지 않음을 검증한다(HTTPFetcher 호출 카운트 = 0) (선행: Task 3.1)
- [x] 4.10 통합 테스트: Pioneer 쓰기 → Harvester 읽기 **round-trip 정합성**을 검증한다. 동일 UTC 일자 내에 Pioneer 경로가 특정 URL로 스냅샷을 Put한 직후, Harvester 경로가 같은 URL로 Get하면 본문이 바이트 단위로 회수되고 HTTP 폴백이 호출되지 않아야 한다. 이 테스트는 양 측이 동일한 URL 정규화 함수를 사용함을 행위적으로 보장한다(design.md Decision 5 보강) (선행: Task 2.5, 3.1)

## 5. 관측성 및 검증

- [x] 5.1 fetch 출처(스냅샷/HTTP) 및 ObjectStorage 실패 사유가 로그/메트릭으로 식별 가능한지 확인한다(행위 아님, 운영 관찰 및 알람 임계치 산정의 근거) (선행: Task 2.3)
- [x] 5.2 자동화 검증: `apps/api/internal/bot/snapshot` 패키지의 공용 키 함수를 Harvester 읽기 측이 import해 사용하고 있음을 정적 검사로 확인한다. 구체적으로 (a) `ObjectStorageFetcher.Fetch` 구현(혹은 그 호출 트리)이 `snapshot.SnapshotKey` 또는 `snapshot.HashNormalizedURL`을 참조하며, (b) `apps/api/internal/bot` 하위의 **프로덕션 Go 파일(`*.go`, `_test.go` 제외)** 중 `snapshot/` 서브패키지 외의 파일에서 `crypto/sha256` import와 `snapshots/%s/%s.html.gz` 문자열 리터럴(및 그 동등 포맷)이 등장하지 않음을 grep 기반 검사로 확인한다. 추가로 (c) `ObjectStorageFetcher.Fetch` 및 그 호출 트리가 Pioneer 쓰기 경로(`pioneer_consumer.go`)가 `SnapshotKey` 입력 계산에 호출하는 동일한 URL 정규화 심볼을 호출하며, 그 외 정규화 함수로 대체하지 않음을 grep/정적 검사로 확인한다(Decision 5 보강). 테스트 파일(`_test.go`)은 예상 키를 역계산할 수 있어 검사 대상에서 제외한다 (선행: Task 2 전체 완료)
- [x] 5.3 `openspec validate harvester-snapshot-first-fetch --strict`를 통과한다 (선행: Task 1~4 전체 완료)

## 6. 본 change 범위 외 후속 추적

<!--
이 섹션의 항목은 본 change archive 판정의 체크 대상이 아니다(in-scope task는 Section 1~5).
본 change가 유발한 인접 문서 불일치를 추적 포인트로 남겨, 별도 후속 change에서 해소되도록 한다.
다만 archive PR 설명의 운영 체크리스트에는 §6.1의 후속 스텁 change 생성 항목을 포함시켜 절차적 준수를 유도한다(블로커는 아니며, design.md Decision 5b "후속 스텁 모니터링 책임" 문단에 1차 책임자와 에스컬레이션 경로가 명시되어 있다).
-->

- [ ] 6.2 [후속] Decision 1b("HTTP 경계 helper 공유")의 **Pioneer 측 정합**. `apps/api/internal/bot/pioneer_consumer.go`의 `DefaultConsumerFetcher`는 현재 10s timeout / 5 redirects / 5MB / FugueBot User-Agent 를 `fetchHTMLShared`를 호출하지 않고 인라인으로 중복 구현하고 있어, 값은 일치하지만 helper 공유는 깨져 있다(drift 리스크). Harvester 측은 `HTTPFetcher.Fetch`가 `fetchHTMLShared`를 직접 호출하므로 본 change 범위 안에서는 계약 일치. 본 change가 Decision 1b를 계약으로 확정했으므로 후속 스텁 change(가칭 `pioneer-http-helper-sync`)에서 `DefaultConsumerFetcher`가 `fetchHTMLShared` 또는 그에 상응하는 공용 설정을 호출하도록 정렬한다. 본 change는 Harvester 읽기 측에 국한되며 사전 상태의 Pioneer drift는 archive를 블록하지 않는다 (선행: 본 change 머지 후)
- [ ] 6.1 [후속] `openspec/specs/harvester/spec.md` 311행의 "`harvester-snapshot-first-fetch` capability" 표현 — 실제로는 `bot` capability에 통합된 상태 — 을 "`bot` capability의 snapshot-first 경로"로 교정하는 후속 스텁 change(가칭 `harvester-mainloop-snapshot-wording-sync`; 최종 이름은 스텁 change 생성 시점에 결정)를 생성한다. 본 change MODIFIED 범위에서 `harvester` capability를 건드리지 않는 이유는 design.md Decision 5b 참조. **트리거 강제**: 본 change archive PR 설명에 "후속 wording-sync 스텁 change 생성 링크"를 필수 체크리스트 항목으로 포함시켜, 머지 직후 같은 사이클(동일 스프린트 또는 1주 이내) 내 생성이 절차적으로 보장되도록 한다. 스텁 change가 열리지 않은 채 1주를 초과하면 운영 이슈로 에스컬레이션한다 (선행: 본 change 머지 후)
