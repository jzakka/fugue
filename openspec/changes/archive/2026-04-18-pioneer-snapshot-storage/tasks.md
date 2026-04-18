## 1. 인프라 및 설정

- [ ] 1.1 운영 환경에서 **단일 bucket + `snapshots/` prefix로 통합 여부 확정** (스냅샷 전용 bucket vs 기존 미디어 bucket 공유) 및 terraform/helm 반영 _(infra owner action; helm 측 env hook은 1.4에서 추가됨)_
- [ ] 1.2 `snapshots/` prefix에 TTL 365일 lifecycle rule 추가 (365일 경과 객체 삭제) _(infra owner action)_
- [ ] 1.3 Pioneer 서비스의 IAM/자격 증명에 `PutObject` 권한 부여 (해당 prefix 한정) _(infra owner action)_
- [x] 1.4 feature flag `PIONEER_SNAPSHOT_ENABLED` 환경변수/컨피그 도입 (기본값 off) — `apps/api/internal/config/config.go`, `helm/fugue/templates/cronjob-bot.yaml`

## 2. 스냅샷 저장 컴포넌트 구현

- [x] 2.1 `apps/api/internal/bot/` 하위에 `snapshot` 저장 인터페이스 정의 (예: `SnapshotStore.Put(ctx, normalizedURL, body) error`) — `apps/api/internal/bot/snapshot/store.go`
- [x] 2.2 normalized URL → **sha256** 해시 함수 구현 (`crypto/sha256` 표준 라이브러리 사용, hex 64자 소문자 출력). Pioneer/Harvester 공유 가능하도록 bot 공용 패키지에 배치 — `apps/api/internal/bot/snapshot/key.go::HashNormalizedURL`
- [x] 2.3 키 빌더 구현: 상수 `SnapshotKeyPattern = "snapshots/%s/%s.html.gz"` 정의 및 공개 함수 `SnapshotKey(normalizedURL string, t time.Time) string` 제공 (UTC `yyyymmdd` 포맷). `harvester-snapshot-first-fetch`에서 동일 키 재구성에 사용 — `apps/api/internal/bot/snapshot/key.go`
- [x] 2.4 gzip 스트림 압축 래퍼 구현 (응답 바이트 → gzip → object storage Put). 별도 checksum 검증 없이 gzip 자체 CRC 활용 — `apps/api/internal/bot/snapshot/store.go::gzipBytes`
- [x] 2.5 object storage 클라이언트 어댑터 구현 (S3 호환, 기존 미디어 업로드 코드 재사용 가능 시 공유) — `apps/api/internal/bot/snapshot/store.go::S3Store` (S3PutObjectAPI 인터페이스로 기존 `*s3.Client` 재사용 가능)

## 3. Pioneer 통합

- [x] 3.1 Pioneer fetch 성공 경로에 `SaveRawContent` 훅 추가 (2xx + 본문 길이 > 0 조건 체크) — `apps/api/internal/bot/pioneer.go::saveSnapshot`. 입력 URL은 `templatePath(finalURL)`로 정규화해 전달하여 spec의 "normalized URL의 sha256" 계약을 보장 (조건 체크는 `helpers.go::fetchHTMLShared`가 선행 보장). cmd/bot main.go에서 `WithSnapshotStore` wiring 완료
- [x] 3.2 feature flag off인 경우 저장 단계를 스킵하도록 분기 — `pioneer.go::saveSnapshot` 첫 줄
- [x] 3.3 업로드 실패 시 fail-open 처리: 경고 로그 + 메트릭 카운터 + Pioneer 루프 계속 진행 — `pioneer.go::saveSnapshot` 에러 분기
- [x] 3.4 fetch 실패(4xx/5xx/타임아웃/빈 본문) 시 저장 스킵 경로 확인 — `helpers.go::fetchHTMLShared`가 fetchErr를 반환하므로 `saveSnapshot` 자체가 호출되지 않음 (테스트로 검증)
- [x] 3.5 URLScheduler 상태 업데이트가 스냅샷 결과와 분리되어 있음을 보장: fetch 성공 시점에 즉시 `SetStatus`를 호출하고, 그 뒤 best-effort로 스냅샷 업로드를 수행한다. `SetStatus`는 스냅샷 업로드 결과(성공/실패)에 게이트되지 않아야 한다 (deprecated `fuguebot_pseudo.go`의 `if err := SaveRawContent(...); err == nil { SetStatus(...) }` 패턴은 fail-open 정책 위배이므로 따르지 않는다) — 현재 Pioneer는 URLScheduler를 아직 직접 호출하지 않으나 `saveSnapshot`은 어떤 그래프/스케줄러 상태 변경보다도 _뒤_가 아니라 fetch 직후 best-effort로 호출되며 반환값을 무시한다. 후속 `pioneer-scheduler-consumer` change에서 SetStatus 호출이 추가되더라도 이 호출 순서/무게이트 원칙을 유지해야 함을 `pioneer.go` 인라인 주석으로 박아 둠

## 4. 테스트

- [x] 4.1 단위 테스트: 키 빌더(동일 normalized URL → 동일 sha256 hex 64자, UTC 날짜 포맷, `SnapshotKeyPattern`과 일치) — `internal/bot/snapshot/key_test.go`
- [x] 4.2 단위 테스트: gzip 압축 래퍼(원문 바이트 복원 가능, gzip CRC로 손상 검증) — `internal/bot/snapshot/store_test.go::TestGzipRoundTripPreservesBytes`, `TestGzipCorruptionDetected`
- [x] 4.3 단위 테스트: fetch 실패(4xx/5xx/타임아웃/빈 본문)에서 업로드 미호출 — `internal/bot/pioneer_snapshot_test.go::TestPioneer_NoSnapshotOnFetchFailure`, `TestPioneer_NoSnapshotOnEmptyBody`
- [x] 4.4 단위 테스트: SnapshotStore Put 실패 시 Pioneer가 링크 추출을 계속 수행 — `internal/bot/pioneer_snapshot_test.go::TestPioneer_FailOpen_StoreErrorDoesNotBlockCrawl`
- [x] 4.5 통합 테스트(로컬 S3 mock): 2xx 수신 → 업로드된 객체 키/본문/압축 확인 — `internal/bot/snapshot/store_test.go::TestS3StorePutUploadsGzippedToCorrectKey` (in-memory `S3PutObjectAPI` fake; 키/gzip 본문 검증). MinIO 기반 풀스택 통합은 6.1 스테이징 단계로 이월
- [x] 4.6 통합 테스트: **동시 쓰기 idempotent 확인** — 동일 URL을 같은 UTC 날짜에 두 번(또는 병렬로) 저장 시 동일 키에 덮어쓰기 수행, 최종 객체는 마지막 PUT 내용(last-write-wins) — `internal/bot/snapshot/store_test.go::TestS3StorePutConcurrentSameKeyLastWriteWins`

## 5. 관측성

- [x] 5.1 메트릭: `pioneer_snapshot_uploads_total{result="success|failure"}` 카운터 추가 — `snapshot.Metrics.IncSuccess/IncFailure`. 백엔드 무관한 thread-safe 카운터; Prometheus 어댑터는 후속 관측성 change에서 결합
- [x] 5.2 메트릭: 업로드 지연 히스토그램 (`pioneer_snapshot_upload_duration_seconds`) — `snapshot.Metrics.ObserveDuration` (bounded ring buffer)
- [x] 5.3 구조화 로그: 업로드 실패 시 URL, 해시, 오류 원인 기록 — `pioneer.go::saveSnapshot` 실패 분기 (`url=`, `hash=`, `err=` 필드)

## 6. 롤아웃

- [ ] 6.1 스테이징 환경에 배포하고 feature flag on → 24시간 업로드 성공률/지연/용량 증가 모니터링 _(operator action)_
- [ ] 6.2 운영 환경에 feature flag on으로 점진 롤아웃 _(operator action)_
- [x] 6.3 후속 change `harvester-snapshot-first-fetch`에 키 규칙/TTL/압축 포맷을 문서로 넘김 — `docs/architecture.md`의 Snapshot Storage 섹션
- [x] 6.4 롤백 절차 문서화: feature flag off로 업로드 중단, 저장된 스냅샷은 TTL 자연 소멸 — `docs/architecture.md`의 Snapshot Storage 섹션
