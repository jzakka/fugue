## 1. Pioneer 성공 경로 테스트 보강

- [x] 1.1 `apps/api/internal/scheduler/postgres_scheduler_test.go` 의 `TestIntegration_SetStatus_FetchedExcludesFromQueue`(`fetch_error_count=3 → SetStatus("fetched")`) 는 현재 `count == 0` 및 `next_fetch_at > now + 300d` 만 단언한다. 여기에 `last_fetched_at` 컬럼이 비-NULL 임을 확인하는 단언을 추가한다. SELECT 컬럼에 `last_fetched_at` 을 추가하고 `sql.NullTime` 등 적절한 타입으로 Scan하여 `Valid == true` 를 `t.Errorf` 로 확인한다. 정확한 시각 값은 검증하지 않는다(NULL 여부만 본다).
- [x] 1.2 단독 실행으로 해당 테스트가 PASS 하는지 확인한다. 실행 명령:
      ```
      TEST_DATABASE_URL="postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable" \
        go test ./apps/api/internal/scheduler/ \
        -run TestIntegration_SetStatus_FetchedExcludesFromQueue -count=1 -v
      ```

## 2. Harvester 성공 경로 대칭성 점검

- [x] 2.1 **확인 only**: `apps/api/internal/scheduler/postgres_scheduler_test.go` 의 `TestIntegration_SetStatus_HarvestedEmptyPinsSkipsInsert` 가 이미 `harvested_at.Valid == true` 단언을 포함함을 확인한다(현 상태 기준 라인 ~586–594). 포함되어 있으면 코드 변경 없이 본 하위 task 를 완료로 기록한다.
    - Verified: `postgres_scheduler_test.go:598-600` 에서 `if !harvestedAt.Valid { t.Errorf("harvested_at not set") }` 단언 존재. 코드 변경 0.
- [x] 2.2 **확인 only**: `TestIntegration_SetStatus_HarvestedAtomicityOnPinFKFail` 이 롤백 경로에서 `harvested_at` 이 NULL 로 남는 것을 단언함을 확인한다(현 상태 기준 라인 ~560–568). 포함되어 있으면 코드 변경 없이 본 하위 task 를 완료로 기록한다.
    - Verified: `postgres_scheduler_test.go:572-574` 에서 `if harvestedAt.Valid { t.Errorf("harvested_at set despite pins INSERT failure (atomicity broken)") }` 단언 존재. 코드 변경 0.
- [x] 2.3 **조건부 보강**: 2.1/2.2 확인 중 어느 쪽이라도 단언이 실제 누락되어 있으면 해당 테스트에 `harvested_at` NULL 여부 단언을 1.1 과 같은 방식으로 추가한다. 누락이 없으면 본 하위 task 는 "skipped — already covered" 메모와 함께 완료 처리한다.
    - skipped — already covered (2.1, 2.2 확인 결과 양쪽 테스트 모두 단언 포함).
- [x] 2.4 단독 실행으로 위 두 테스트가 PASS 하는지 확인한다. 실행 명령:
      ```
      TEST_DATABASE_URL="postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable" \
        go test ./apps/api/internal/scheduler/ \
        -run "TestIntegration_SetStatus_Harvested" -count=1 -v
      ```

## 3. Spec delta 의 문서 측 관찰 포인트 명시 (메타 확인)

- [x] 3.1 본 change 의 `specs/scheduler/spec.md` 가 기존 `openspec/specs/scheduler/spec.md` 의 "SetStatus는 status enum 4종을 처리하고 harvested 시 pin 매핑을 저장한다" Requirement 블록 전체를 MODIFIED 로 복제하고, 오직 두 시나리오("fetched status — error_count 리셋", "harvested status — 빈 pinIDs") 의 THEN 절에서만 비-NULL 관찰 포인트를 추가(및 빈 pinIDs 시나리오의 "만" 어휘 드롭)하는 범위로 한정되어 있음을 diff 로 재확인한다. 그 외 시나리오/용어는 base 와 **자구 일치**해야 한다.
    - Verified: `diff` 결과 정확히 2 라인 변경 — (1) "fetched status — error_count 리셋" THEN 에 `last_fetched_at 은 비-NULL 상태이며` 추가, (2) "harvested status — 빈 pinIDs" THEN 에서 `harvested_at만 갱신` → `harvested_at 은 호출 시각으로 갱신되어 비-NULL 상태가 되고` 치환. 그 외 모든 라인 자구 일치.

## 4. Archive task 6.6 체크박스 닫기

- [x] 4.1 `openspec/changes/archive/2026-04-18-scheduler-retry-backoff/tasks.md` 의 6.6 라인을 `- [ ]` 에서 `- [x]` 로만 변경한다. 원문 문구(줄 전체 텍스트)는 유지한다.
- [x] 4.2 6.6 바로 아래에 들여쓴 단일 불릿(ex: `    - Covered by ...`) 으로 "커버 테스트: `apps/api/internal/scheduler/postgres_scheduler_test.go:TestIntegration_SetStatus_FetchedExcludesFromQueue`(본 change `scheduler-retry-backoff-closeout` 에서 `last_fetched_at` 단언 보강)" 를 추가한다. archive된 tasks.md 내 다른 라인은 건드리지 않는다.
- [x] 4.3 archive 의 `proposal.md`, `design.md`, `specs/` 는 수정하지 않음을 확인한다(`git diff -- 'openspec/changes/archive/2026-04-18-scheduler-retry-backoff/**'` 로 변경 범위가 `tasks.md` 단일 파일임을 검증).
    - Verified: `git diff --stat` 결과 `openspec/changes/archive/2026-04-18-scheduler-retry-backoff/tasks.md | 3 ++-` 단일 파일, 2+/1-.

## 5. 전체 정합성 확인

- [x] 5.1 `openspec validate scheduler-retry-backoff-closeout --strict` 가 통과하는지 확인한다.
- [x] 5.2 SetStatus 통합 테스트 묶음이 단독 스코프에서 전부 PASS 하는지 확인한다. 실행 명령:
      ```
      TEST_DATABASE_URL="postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable" \
        go test ./apps/api/internal/scheduler/ \
        -run "TestIntegration_SetStatus_" -count=1 -v
      ```
      `TestIntegration_Dequeue_LeaseExpiryReclaims` 는 본 change 범위 밖이며 실행하지 않거나 실패해도 무시한다(proposal "범위 외" 참조).
