## Context

Archive된 `2026-04-18-scheduler-retry-backoff`의 `tasks.md` 6.6은 당시 의존하던 `scheduler-claim-api` 구현이 선행된 뒤 수행하도록 설계된 "경계 검증" 항목이다. claim-api가 머지된 지금(commit `0a50180`) 해당 검증을 수행할 수 있게 되었으나, 현재 통합 테스트(`postgres_scheduler_test.go:TestIntegration_SetStatus_FetchedExcludesFromQueue`)는 `fetch_error_count`의 0 리셋과 `next_fetch_at`의 1년 뒤 예약은 단언하지만 **`last_fetched_at` 비-NULL 단언을 포함하지 않는다**. `scheduler` capability의 기본 scenario "fetched status 처리"는 `last_fetched_at`을 THEN 절에 포함하지만, 분기 scenario "fetched status — error_count 리셋"은 count 단언만 나열하여 해당 분기만 독립적으로 구현된 테스트가 타임스탬프 관찰을 빠뜨리기 쉬운 구조다. Harvester 측("harvested status — 빈 pinIDs")도 같은 성격의 구조적 갭이 있다.

본 change는 요구되는 행위를 바꾸지 않는다. 두 분기 시나리오의 THEN 절을 자족적으로 만들어, 해당 시나리오를 테스트로 번역했을 때 타임스탬프 비-NULL 관찰이 자연스럽게 포함되도록 문구를 보강하고, 그 결과를 기존 통합 테스트에 반영한 뒤, archive 태스크 6.6을 닫는다.

## Goals / Non-Goals

**Goals:**

- 6.6 태스크의 행위 요구사항(성공 경로가 `fetch_error_count`를 리셋하고 `last_fetched_at` 타임스탬프를 세팅함)을 통합 테스트로 완전히 관찰 가능한 상태로 만든다.
- `scheduler` spec의 두 분기 시나리오 THEN 절을 자족적으로 정비한다(문구 보강, 행위 변경 아님).
- Harvester 대칭성: `harvested_at` 비-NULL 관찰도 동일 기준으로 점검/보강한다.
- Archive된 task 6.6을 `[x]`로 전환하고 커버 테스트 포인터를 명시한다.

**Non-Goals:**

- `scheduler` capability의 실제 행위(backoff 공식, dead 임계값, enum 값, lease duration, politeness rate) 변경.
- `TestIntegration_Dequeue_LeaseExpiryReclaims` 전체 스위트 실행 시 test isolation flakiness 해결.
- 신규 `ErrorKind` 값 추가, 4xx 하위 분류(`410 Gone`, `429`, `401/403`) 도입.
- archive된 proposal.md/design.md/specs의 문구 수정 — 본 change는 archive의 `tasks.md` 체크박스 및 포인터 주석만 손댄다.

## Decisions

### D1. 시나리오 문구 보강 vs. 전용 새 시나리오 추가

**Choice**: 기존 두 시나리오의 THEN 절을 **MODIFIED** 로 보강한다.

**Rationale**: 새 시나리오를 추가하면 "fetched status 처리"(일반) + "fetched status — error_count 리셋"(count) + 신규(타임스탬프) 로 동일한 성공 경로를 세 개로 쪼개게 되어 spec 가독성과 테스트 매핑이 악화된다. 분기 시나리오는 "분기의 고유 관찰 포인트"만이 아니라 "분기를 독립 케이스로 설명할 때 필요한 최소 충분 관찰 포인트"를 가지는 게 spec-as-test 계약에 더 가깝다. MODIFIED는 `openspec-propose` 규약에 따라 전체 requirement 블록을 복사해 붙이므로 archive 시점의 spec 병합이 올바르게 이루어진다.

**부속 어휘 조정**: "harvested status — 빈 pinIDs" 시나리오의 기존 THEN 절에 있던 `harvested_at**만** 갱신되고` 의 "만"(only) 한정사는 본 MODIFIED에서 드롭한다. 이는 행위 변경이 아니라 어휘 교정이다 — 상위 Requirement 문단이 이미 `harvest_error_count = 0 으로 리셋`을 SHALL 로 명시하고 있어, 기존 "만" 은 해당 SHALL 과 국소적으로 약하게 모순했다. 새 THEN 절은 `harvested_at` 의 비-NULL 관찰만을 자족적으로 기술하고, 그 외 필드 변경(예: `harvest_error_count` 리셋, `last_updated_at` 갱신)은 Requirement 본문에 따라 일반 계약이 적용됨을 묵시한다.

**Alternatives considered**:

- _새 시나리오 ADDED_: 시나리오 수가 늘어 spec-test 매핑이 1:N으로 흐려진다. 기각.
- _전면 재작성 (ADDED로 새 requirement)_: 기존 requirement와 중복되거나 상충하게 된다. 기각.

### D2. 테스트 수정 최소주의

**Choice**: 기존 두 테스트 함수에 **단언만 1~2줄 추가**하고 새 테스트 파일/함수는 만들지 않는다.

**Rationale**: 6.6은 "경계 검증"이므로 새 테스트가 아니라 이미 존재하는 경계 테스트의 검증 영역을 완결하는 것이 본질이다. Go test 파일에 구조적 리팩터/헬퍼 추가는 본 change 범위 밖이다.

### D3. Archive 파일 수정 범위

**Choice**: archive된 `tasks.md` 만 수정한다. 체크박스(`[ ]` → `[x]`)와 "커버 테스트: `postgres_scheduler_test.go:TestIntegration_SetStatus_FetchedExcludesFromQueue` (본 change `scheduler-retry-backoff-closeout` 에서 보강)" 수준의 포인터 주석만 허용한다.

**Rationale**: Fugue 저장소에 "archive immutable" 컨벤션이 명문화되어 있지 않지만, archive의 본문(요구사항/설계/스코프 서술)을 사후 수정하면 당시 의사결정 기록의 진위성을 훼손한다. 체크박스 상태 전환은 "이 task가 향후 어디에서 완료되었는지"를 연결하는 bookkeeping에 가깝고, 포인터 주석은 부가 정보로 격리된다. 이 원칙을 design.md에 명시해 후속 change 저자가 오해하지 않도록 한다.

**Alternatives considered**:

- _archive 전체를 잠그고 본 change의 새 tasks.md에만 "6.6 carry-over" 기록_: 추적성은 확보되나 archive 파일을 보는 이가 6.6의 최종 운명을 스스로 파악하기 어렵다. archive 파일 자체의 체크박스 상태 전환이 가장 자연스러운 "완결 표시"다. D3 채택.

### D4. 통합 테스트의 `TEST_DATABASE_URL` 의존성은 그대로

**Choice**: 기존 `openTestDB`의 `TEST_DATABASE_URL` gate 방식을 유지한다. `docker-compose up -d`로 기동된 `fugue-postgres-1` DSN(`postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable`)을 로컬 실행 전제로 한다.

**Rationale**: 본 change는 테스트 인프라를 바꿀 범위가 아니다. 6.6은 이 gate 하에서 통과함을 이미 확인했다.

## Risks / Trade-offs

- **Risk**: archive 파일 수정을 허용한 전례가 생긴다 → **Mitigation**: D3에 수정 범위(체크박스 + 포인터 주석)를 명시하고, 본 change의 proposal.md에도 "문구/요구사항/스코프 변경은 금지" 문구를 유지. 후속 change가 이를 확대 해석하지 않도록 본 change의 design.md를 참조 규범으로 둔다.
- **Risk**: spec 시나리오 MODIFIED 문구 보강이 "행위 변경"으로 잘못 읽힐 수 있다 → **Mitigation**: proposal.md와 design.md 양쪽에서 "문구 보강 / 관찰 포인트 자족성 / 행위 변경 아님"을 명시. 보강 전/후 모두 해당 행위는 `SetStatus` 구현체가 이미 수행하고 있으며, 일반 시나리오가 이미 문서화한 내용과 동일하다.
- **Risk**: 테스트 단언이 과도하게 구체화되어(예: 특정 시각 range) 유지보수 부담을 만든다 → **Mitigation**: 단언은 NULL 여부 한 줄로만 한정한다. `last_fetched_at`의 정확한 시각은 검증하지 않는다 — 이미 `next_fetch_at > now + 300d` 단언이 타임스탬프 근사 관찰을 담당한다.
- **Trade-off**: 본 change의 규모 대비 OpenSpec 워크플로(proposal + design + specs + tasks)가 무겁다. 그러나 archive 파일 수정이라는 민감한 결정(D3)이 포함되어 있으므로 전체 아티팩트를 거치는 비용이 정당화된다.

## Migration Plan

단일 PR/커밋으로 적용 가능. 롤백은 git revert로 충분하며 DB/외부 의존성 영향 없음.
