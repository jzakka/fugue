## 1. lookup 헬퍼 도입

- [x] 1.1 `apps/api/internal/scheduler/url_scheduler.go`에 `hashLookupKey(rawKey string) ([]byte, bool)` 헬퍼 추가. 내부에서 `urlcanon.Canonical(rawKey)` 호출 후 결과가 빈 문자열이면 `(nil, false)` 반환(lookup skip 신호), 그 외는 `(sha256(canonical), true)` 반환.
- [x] 1.2 `hashKey`를 enqueue 경로(이미 정규화된 입력) 전용으로 doc-comment 정정. lookup 경로(SetStatus/RecordFetchError/RecordHarvestError 등)는 `hashLookupKey`를 사용하도록 안내 주석 추가.

## 2. SetStatus 정규화 적용

- [x] 2.1 `apps/api/internal/scheduler/postgres_scheduler.go` `SetStatus`의 `hash := hashKey(key)` 호출을 `hashLookupKey(key)`로 교체. `(nil, false)` 반환 시(정규화 불가) DB 호출을 skip하고 nil error 반환.
- [x] 2.2 `recordError`(RecordFetchError/RecordHarvestError 공용 구현)의 `hashKey(key)` 호출을 `hashLookupKey(key)`로 교체. 동일한 short-circuit 적용.

## 3. EnqueueHarvester lookup 정규화 검증

- [x] 3.1 `EnqueueHarvester` 내부의 hash 산출 경로를 점검: 정규화→hash 순서가 enqueue 경로와 동일한지 확인. 동일하면 변경 없음, 누락이 있으면 정규화 단계 추가.
- [x] 3.2 `EnqueueHarvester`가 `harvester_frontier_pins` 등 부수 테이블 lookup에 hash를 사용한다면, 동일 정규화 일관성 적용.

## 4. 회귀 테스트

- [x] 4.1 `apps/api/internal/scheduler/postgres_scheduler_test.go`에 `TestIntegration_SetStatus_FetchedNormalizesKey` 추가: `Enqueue("https://www.example.com/")` → `Dequeue()` → SetStatus(반환값, fetched) round-trip 후 row의 `last_fetched_at` non-NULL 검증.
- [x] 4.2 동일 패턴으로 fragment 제거 입력(`https://example.com/page#section`) round-trip 테스트 추가.
- [x] 4.3 `TestIntegration_RecordFetchError_NormalizesKey` 추가: 정규화 변형 입력으로 RecordFetchError가 row의 `fetch_error_count`를 1 증가시키는지 검증.
- [x] 4.4 멱등성 테스트: 이미 정규화된 입력이 SetStatus에 전달되어도 정상 매치되는지 확인(이중 정규화 안정성).
- [x] 4.5 정규화 불가 입력(빈 문자열, 파싱 실패 URL)에 대한 안전 경로 테스트: SetStatus / RecordFetchError / RecordHarvestError가 panic하거나 워커 프로세스를 종료시키지 않고 정상 반환하며, 어떤 frontier row도 변경되지 않는지 검증(warning 발생 여부는 구현 선택, 어서션 대상 아님).

## 5. EnqueueHarvester round-trip 테스트

- [x] 5.1 Pioneer 시뮬레이션 테스트: Dequeue가 반환한 raw URL을 EnqueueHarvester에 전달하는 시나리오에서 harvester_frontier row가 정확히 한 번만 생성되고 url_hash 충돌이 ON CONFLICT로 정상 처리되는지 검증.

## 6. 호출 사이트 검증

- [x] 6.1 `apps/api/internal/bot/pioneer_consumer.go`: SetStatus/RecordFetchError 호출 인자가 `Dequeue` 반환값임을 확인. 별도 변경 불필요(scheduler 내부 정규화로 자동 보호).
- [x] 6.2 `apps/api/internal/bot/harvester_consumer.go`: SetStatus/RecordHarvestError 호출 인자가 동일 계약을 만족하는지 점검.
- [x] 6.3 `cmd/bot/main.go` 등 다른 SetStatus 직접 호출 사이트가 있는지 grep으로 확인하고 동일 검증 적용.

## 7. 운영 검증

- [x] 7.1 빌드 통과(`go build ./...`).
- [x] 7.2 `go vet ./...` 통과.
- [x] 7.3 신규 + 기존 scheduler/bot 패키지 테스트 모두 통과.
- [x] 7.4 `pioneer pixiv` 90초 재실행 시 `WARN scheduler.set_status_fetched: unknown key` 로그가 0건임을 확인. 시드 row의 `last_fetched_at`이 non-NULL로 갱신되는지 DB 쿼리로 검증.
- [x] 7.5 `openspec validate pioneer-scheduler-key-contract` 통과.
