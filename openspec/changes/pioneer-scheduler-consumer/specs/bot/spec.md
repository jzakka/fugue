## REMOVED Requirements

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
**Reason**: "BFS로 사이트를 탐색"한다는 표현은 Pioneer가 사이트 루트를 시작점으로 사이트 내부를 너비 우선으로 하강한다는 **단일 프로세스/인메모리 세션** 모델을 전제한다. Pioneer는 이제 `URLScheduler`의 얇은 consumer로 재정의되며, 다음 URL은 frontier에서 `Dequeue`로 받고 링크를 `Enqueue`로 되돌려놓는다. 큐 자료구조와 순회 전략은 `URLScheduler` 구현체와 frontier의 우선순위 스코어가 결정하므로, Pioneer 자체의 요구사항으로 "BFS"를 고정하는 것은 타당하지 않다.

기존 6개 scenario의 처리 내역은 다음과 같다.

| Scenario | 처리 | 근거 |
|----------|------|------|
| DOM 기반 링크 추출 | **보존**(무영향) | `bot` spec의 독립 requirement "DOM ancestor selector를 포함하여 링크를 추출한다"가 그대로 담당 |
| 필터 체인을 통한 링크 필터링 | **보존**(무영향) | `bot` spec의 독립 requirement "필터 체인이 순서대로 필터를 적용한다"가 그대로 담당 (필터 정책 세부는 `pioneer-link-filter-policy`에서 이관·확장) |
| 복합 우선순위 계산 | **이관**(후속) | `pioneer-link-filter-policy`에서 score 보정 책임으로 재정의(선행 change) |
| 이미 방문한 링크에 대한 엣지 생성 | **영구 삭제** | 새 모델에는 "사이트 세션" 개념이 없고 graph edge를 유지하지 않는다. DECISIONS.md §2의 frontier-only 모델에 따라 의도적 폐기 |
| 부모 관계 추적 및 엣지 생성 | **영구 삭제** | 동일 근거(의도적 폐기). 노드 간 부모-자식 관계는 URLScheduler 모델의 책임이 아니다 |
| 최대 노드 수 제한 준수 | **부분 대체** | 사이트 단위 quota 자체는 폐기된다. 대신 워커 단위 예산은 `pioneer-worker-budget`(성공 Dequeue 100회 후 종료)이, host 단위 속도 제한은 `scheduler-host-token-bucket`이 각각 다른 관점에서 담당 |

**Migration**: 본 change가 정의하는 `pioneer` capability의 "Pioneer 메인 루프는 Dequeue → fetch → snapshot → parse → filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus 반복이다" 및 관련 requirement 집합이 Pioneer의 탐색 모델을 대체한다. "DOM 기반 링크 추출"과 "필터 체인을 통한 링크 필터링"은 `bot` spec의 독립 requirement가 계속 담당한다. "복합 우선순위 계산"은 `pioneer-link-filter-policy`로 이관된다. "최대 노드 수 제한"은 사이트 단위 quota가 의도적으로 폐기되고 워커 단위 예산(`pioneer-worker-budget`)/host 단위 속도 제한(`scheduler-host-token-bucket`)으로 관점이 전환된다. "이미 방문한 링크에 대한 엣지 생성"과 "부모 관계 추적 및 엣지 생성"은 graph edge 유지 자체를 폐기하므로 복원되지 않는다(의도적 trade-off).

---

### Requirement: 스냅샷 저장 실패는 fail-open으로 처리한다
**Reason**: 본 change는 Pioneer가 `URLScheduler`의 얇은 consumer로 재정의되면서 snapshot 저장 실패를 "관측 누락(fail-open)"이 아니라 "명시적 실패 보고 대상(fail-close)"으로 재정의한다. 새 `pioneer` capability의 "Pioneer는 실패 시 SetStatus + RecordFetchError 둘 다 호출한다" requirement는 HTTP fetch는 성공했으나 snapshot 저장에 실패한 경우 `SetStatus(url, "fetch_failed", nil)` + `RecordFetchError(url, "network")`를 호출하도록 규정한다. 기존 fail-open requirement("업로드 실패 시 URLScheduler 상 fetch 성공으로 취급되어 재시도 큐에 들어가지 않는다")는 신규 정책과 직접 모순되므로 제거한다.

**Migration**: archived change `pioneer-snapshot-storage`가 남긴 fail-open 회귀 테스트는 본 change 구현 단계에서 fail-close 규약에 맞춰 교체되어야 한다(구체 테스트 교체 지침은 tasks.md 6.6 참조). object storage 업로드 실패는 계속 로그로 기록되어야 한다는 부분 역시 신규 `RecordFetchError` 호출 시 kind=`"network"`와 함께 관측 가능해진다.

---

### Requirement: Pioneer는 ParseLinks 후 FilterLinks를 거쳐 Enqueue한다
**Reason**: 이 requirement의 "Pioneer가 FilterChain 결과만 Enqueue한다" 행위 계약은 본 change가 도입하는 `pioneer` capability의 "FilterChain 호출은 Pioneer의 책임이다" requirement 및 "Pioneer 메인 루프는 Dequeue → fetch → snapshot → parse → filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus 반복이다" requirement가 이어받아 규범화한다. `bot` spec에 동일 주제를 중복 기술하면 SSOT가 깨지므로 제거한다.

**Migration**: 4개 scenario의 처리 내역은 다음과 같다.
- "Enqueue된 URL은 필터 체인 통과 결과만 포함한다": 새 `pioneer` capability의 "필터 통과 링크만 Enqueue" scenario가 이어받는다.
- "빈 결과 처리": `pioneer` capability의 "필터 통과 링크만 Enqueue"에 흡수된다(반환된 부분 집합이 빈 목록인 경우 Enqueue 인자가 빈 슬라이스가 되는 것이 자연스러운 행위).
- "Redirect chain의 최종 URL만 사용": 본 requirement 범위 밖으로 이관되지 않으며, `fetcher` 구현의 책임으로 남는다. Pioneer는 Fetcher가 반환한 최종 URL을 그대로 사용한다(Pioneer spec은 fetcher 동작을 재규범화하지 않는다).
- "파싱 불가능한 URL은 큐에 포함되지 않는다": `pioneer-link-filter-policy`의 filter 체인(특히 `DomainFilter`/`DedupFilter`의 URL parse 실패 시 탈락 동작)이 이어받는다.
