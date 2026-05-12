## ADDED Requirements

### Requirement: SetStatus/RecordFetchError/RecordHarvestError lookup은 정규화 일관성을 보장한다

URLScheduler는 `SetStatus(key, ...)`, `RecordFetchError(key, ...)`, `RecordHarvestError(key, ...)` 호출 시 `key` 인자를 `url_hash` 산출 직전에 `Enqueue` 시 적용한 동일 정규화 단계를 거쳐야 한다(SHALL). 다시 말해, 동일한 raw URL 문자열에 대해 다음 두 hash가 항상 일치해야 한다(SHALL):

- `Enqueue(rawURL)` 결과로 frontier row에 저장된 `url_hash`
- 이후 `SetStatus(rawURL, ...)` / `RecordFetchError(rawURL, ...)` / `RecordHarvestError(rawURL, ...)`가 lookup에 사용하는 hash

이로써 `Dequeue`가 반환한 URL 문자열을 호출자가 그대로 위 세 메서드에 넘기더라도 동일 frontier row가 매치된다(SHALL). 정규화가 URL 형태를 변경하는 입력(예: 호스트의 `www.` 접두 제거, fragment 제거, trailing slash 통일 등) 모두에 대해 이 invariant가 성립해야 한다(SHALL).

#### Scenario: www. 제거 입력에 대한 round-trip
- **WHEN** 호출자가 `Enqueue(QueuePioneer, "https://www.example.com/")`를 호출하여 frontier에 row가 생성된 후, `Dequeue(QueuePioneer)`가 반환한 URL을 그대로 `SetStatus(returnedURL, "fetched", nil)`에 전달할 때 (정규화기가 `www.`를 제거하여 `normalized_url=https://example.com/`로 저장한다고 가정)
- **THEN** SetStatus는 동일 frontier row를 매치하고 `last_fetched_at`을 non-NULL로 갱신한다. 이후 lease 만료 시 동일 row가 재-claim되지 않는다.

#### Scenario: fragment 제거 입력에 대한 round-trip
- **WHEN** 호출자가 `Enqueue(QueuePioneer, "https://example.com/page#section")`로 row를 생성한 후, `Dequeue` 결과를 그대로 `RecordFetchError(returnedURL, "http_5xx")`에 전달할 때 (정규화기가 fragment를 제거하여 `normalized_url=https://example.com/page`로 저장한다고 가정)
- **THEN** RecordFetchError는 동일 frontier row를 매치하고 `fetch_error_count`를 1로 증가시킨다.

#### Scenario: 이미 정규화된 입력의 멱등 round-trip
- **WHEN** 호출자가 정규화된 URL `"https://example.com/"`을 Enqueue한 후 동일 URL을 SetStatus에 전달할 때
- **THEN** SetStatus는 동일 frontier row를 매치한다(이중 정규화로 인한 변형이 발생하지 않음).

#### Scenario: 정규화 불가 입력의 안전한 처리
- **WHEN** SetStatus / RecordFetchError / RecordHarvestError가 정규화 불가 입력(빈 문자열 또는 파싱 실패 URL)으로 호출될 때
- **THEN** 호출은 panic하거나 워커 프로세스를 종료시키지 않고 정상 반환하며, 어떤 frontier row도 변경되지 않는다.

---

### Requirement: EnqueueHarvester lookup도 정규화 일관성을 보장한다

`EnqueueHarvester(rawURL, snapshotKey)`가 내부적으로 `harvester_frontier`의 기존 row를 lookup/UPSERT 하기 위해 `url_hash`를 산출하는 모든 경로는 위와 동일한 정규화 일관성을 따라야 한다(SHALL). 호출자가 `Dequeue`의 raw URL을 그대로 `EnqueueHarvester`에 전달하더라도 정규화가 형태를 변경하는 입력에 대해 무한 INSERT 충돌이나 silent miss가 발생하지 않아야 한다(SHALL).

#### Scenario: 정규화 변형 입력에 대한 EnqueueHarvester round-trip
- **WHEN** 호출자가 정규화가 형태를 변경하는 raw URL(예: `"https://www.example.com/article"`)로 `EnqueueHarvester`를 호출할 때
- **THEN** harvester_frontier에 해당 URL에 대응하는 row가 정확히 1개 생성된다.

#### Scenario: 동일 raw URL로 EnqueueHarvester를 두 번 호출
- **WHEN** 호출자가 동일한 raw URL로 `EnqueueHarvester`를 두 번 연속 호출할 때(정규화 결과가 첫 호출과 동일)
- **THEN** harvester_frontier에는 해당 URL에 대응하는 row가 여전히 정확히 1개만 존재하고, 두 번째 호출은 에러 없이 정상 반환된다.
