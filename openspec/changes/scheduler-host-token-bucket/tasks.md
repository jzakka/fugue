## 1. 의존성 및 설정

- [ ] 1.1 `go.mod`에 `golang.org/x/time/rate` 추가 및 `go mod tidy`
- [ ] 1.2 설정 키 정의: `scheduler.host_default_rate_per_sec`(기본 1.0), `scheduler.host_default_burst`(기본 5), `scheduler.host_blocked_sleep_ms`(기본 100, 상한 1000), `scheduler.host_token_bucket_enabled`(기본 true)
- [ ] 1.3 설정 surface 위치 결정(env 변수 또는 config 파일)과 로딩 코드 추가

## 2. HostRateLimiter 컴포넌트

- [ ] 2.1 `apps/api/internal/scheduler/host_rate_limiter.go` 신설: `map[string]*rate.Limiter` + `sync.RWMutex` 구조체 정의
- [ ] 2.2 `Allow(host string) bool` 메서드: 호스트 bucket이 없으면 기본 rate/burst로 lazy 생성하고 `Allow()` 호출 결과 반환
- [ ] 2.3 `SetHostRate(host string, ratePerSec float64, burst int)` 메서드: 신규/기존 호스트 모두 새 설정으로 즉시 동작하도록 Limiter 재생성
- [ ] 2.4 신규 호스트 등록 시 write lock, 기존 호스트 조회 시 read lock으로 분리
- [ ] 2.5 `host_token_bucket_enabled = false`일 때 `Allow()`가 항상 true를 반환하는 우회 경로 추가

## 3. URLScheduler dequeue 통합 훅

- [ ] 3.1 `scheduler-claim-api`의 dequeue 흐름과 통합 지점 확인(후보 N개 fetch → host 필터링 → 첫 통과 후보 claim)
- [ ] 3.2 dequeue 루프에서 후보별 `HostRateLimiter.Allow(host)` 호출 추가
- [ ] 3.3 모든 후보가 blocked일 때 `host_blocked_sleep_ms` 만큼 sleep 후 frontier 재조회
- [ ] 3.4 sleep 값이 1000ms를 초과하면 1000ms로 캡

## 4. 단위 테스트

- [ ] 4.1 신규 호스트의 첫 `Allow` 호출이 기본 rate/burst로 bucket을 생성함을 검증
- [ ] 4.2 burst 한도까지는 연속 `Allow`가 모두 true, 그 다음은 false임을 검증
- [ ] 4.3 시간 경과(가짜 시계 또는 `time.Sleep`)에 따라 token이 충전되어 `Allow`가 다시 true가 됨을 검증
- [ ] 4.4 `SetHostRate` 호출 후 즉시 새 rate/burst가 반영됨을 검증
- [ ] 4.5 `host_token_bucket_enabled = false`일 때 `Allow`가 항상 true를 반환함을 검증
- [ ] 4.6 동시 다수 goroutine에서 `Allow`/`SetHostRate` 호출 시 race detector(`go test -race`) 통과

## 5. 통합 테스트(dequeue 흐름)

- [ ] 5.1 후보 집합에 동일 호스트 K개가 포함될 때 burst만큼만 claim되고 나머지는 다음 사이클로 미루어짐을 검증
- [ ] 5.2 최상위 score 후보가 blocked일 때 차상위 후보가 먼저 claim됨을 검증
- [ ] 5.3 모든 후보가 blocked인 시나리오에서 sleep 후 재조회가 발생함을 검증

## 6. 호스트 override 통합 지점 노출

- [ ] 6.1 Pioneer가 호출할 수 있도록 scheduler 패키지에서 `HostRateLimiter`(또는 동등 인터페이스)를 export
- [ ] 6.2 Pioneer 측 통합(`pioneer-link-filter-policy`)에서 robots.txt Crawl-delay → `SetHostRate(host, 1/delay, 1)` 호출하는 hook 위치를 PR 설명/문서로 명시(본 change에서 호출은 추가하지 않음)

## 7. 검증 및 문서

- [ ] 7.1 `openspec validate scheduler-host-token-bucket --strict` 통과 확인
- [ ] 7.2 `docs/architecture.md`의 scheduler 섹션에 host token bucket 동작과 기본 rate/burst 설명 추가
- [ ] 7.3 운영 가이드에 "프로세스 수 × 호스트 rate = 외부 사이트 실효 rate" 주의사항과 튜닝 가이드 1단락 추가

## 8. 후속 change 메모(본 change 범위 외)

- [ ] 8.1 Limiter GC 정책(미사용 호스트 정리)이 필요해질 시점에 별도 change로 도입함을 메모
- [ ] 8.2 분산 rate limit이 필요해지면 `scheduler-distributed-rate-limit` 신규 change로 분리함을 메모
- [ ] 8.3 `Reserve()` 기반 정확한 wait 모델로 전환 검토 시점을 운영 데이터로 판단하도록 메모
