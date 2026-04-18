## 1. 의존성 및 설정

- [x] 1.1 `go.mod`에 `golang.org/x/time/rate` 추가 및 `go mod tidy`
- [x] 1.2 설정 키 정의: `scheduler.host_default_rate_per_sec`(기본 1.0), `scheduler.host_default_burst`(기본 5), `scheduler.host_token_bucket_enabled`(기본 true). 모든 후보 blocked 시 sleep 관련 설정은 본 change 범위 외이며 `scheduler-claim-api`에서 정의됨.
- [x] 1.3 설정 surface 위치 결정(env 변수 또는 config 파일)과 로딩 코드 추가

## 2. HostRateLimiter 컴포넌트

- [x] 2.1 `apps/api/internal/scheduler/host_rate_limiter.go` 신설: `HostRateLimiter` 구조체 정의 (`map[string]*rate.Limiter` + `sync.RWMutex` + 기본 rate/burst 필드)
- [x] 2.2 `func (l *HostRateLimiter) Allow(host string) bool` 구현: 호스트 bucket이 없으면 기본 rate/burst로 lazy 생성하고 `limiter.Allow()` 결과 반환
- [x] 2.3 `func (l *HostRateLimiter) SetHostRate(host string, rate float64, burst int)` 구현: 신규/기존 호스트 모두 새 설정으로 즉시 동작하도록 Limiter 재생성
- [x] 2.4 `SetHostRate`에서 `rate <= 0` 또는 `burst <= 0`이면 현재 운영자 설정 기본값(`scheduler.host_default_rate_per_sec`, `scheduler.host_default_burst`; 미설정 시 공장 기본값 1 req/sec, burst 5)으로 대체하고 `WARN` 레벨 경고 로그(대체된 실제 값 명시)를 남기는 유효성 처리 추가 (에러 반환/패닉 없음)
- [x] 2.5 신규 호스트 등록 시 write lock, 기존 호스트 조회 시 read lock으로 분리 (double-checked locking 패턴)
- [x] 2.6 호스트 키는 호출부에서 전달된 문자열을 그대로 사용 (scheduler 내부에 정규화 로직 없음; 정규화 책임은 Pioneer)
- [x] 2.7 `host_token_bucket_enabled = false`일 때 `Allow()`가 항상 true를 반환하는 우회 경로 추가

## 3. claim-api와의 통합 경계

- [x] 3.1 본 change는 `Allow`/`SetHostRate` 두 메서드만 제공하고, claim 시점의 호출 타이밍은 `scheduler-claim-api` change의 Claim 프로토콜에서 정의됨을 PR 설명/design.md에 명시
- [x] 3.2 Pioneer 등 외부 소비자가 호출할 수 있도록 scheduler 패키지에서 `HostRateLimiter`를 export

## 4. 단위 테스트

- [x] 4.1 신규 호스트의 첫 `Allow` 호출이 기본 rate/burst(1 rps, burst 5)로 bucket을 생성함을 검증
- [x] 4.2 burst 한도까지는 연속 `Allow`가 모두 true, 그 다음은 false임을 검증
- [x] 4.3 시간 경과(가짜 시계 또는 `time.Sleep`)에 따라 token이 충전되어 `Allow`가 다시 true가 됨을 검증
- [x] 4.4 `SetHostRate` 호출 후 즉시 새 rate/burst가 반영됨을 검증(기존 호스트/신규 호스트 모두)
- [x] 4.5 `SetHostRate(host, 0, 5)`, `SetHostRate(host, 1, 0)`, `SetHostRate(host, -1, -1)` 입력 시 기본값으로 대체되고 WARN 로그가 발생함을 검증
- [x] 4.6 `host_token_bucket_enabled = false`일 때 `Allow`가 항상 true를 반환함을 검증
- [x] 4.7 동시 다수 goroutine에서 `Allow`/`SetHostRate` 혼합 호출 시 race detector(`go test -race`) 통과

## 5. 검증 및 문서

- [x] 5.1 `openspec validate scheduler-host-token-bucket --strict` 통과 확인
- [x] 5.2 `docs/architecture.md`의 scheduler 섹션에 host token bucket 동작(메서드 시그니처, 기본 rate/burst, 유효성 정책) 설명 추가
- [x] 5.3 운영 가이드에 "프로세스 수 × 호스트 rate = 외부 사이트 실효 rate" 주의사항과 튜닝 가이드 1단락 추가

## 6. 후속 change 메모(본 change 범위 외)

- [x] 6.1 Limiter GC 정책(미사용 호스트 정리)이 필요해질 시점에 별도 change로 도입함을 메모
- [x] 6.2 분산 rate limit이 필요해지면 `scheduler-distributed-rate-limit` 신규 change로 분리함을 메모
- [x] 6.3 `Reserve()` 기반 정확한 wait 모델로 전환 검토 시점을 운영 데이터로 판단하도록 메모
