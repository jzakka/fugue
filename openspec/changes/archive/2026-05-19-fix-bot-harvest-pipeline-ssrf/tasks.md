# Tasks: fix-bot-harvest-pipeline-ssrf

## 1. SSRF-safe HTTP client 헬퍼 패키지 신설

- [ ] 1.1 `apps/api/internal/httpclient/ssrf.go` 신규 — `NewSSRFSafeClient(Options) *http.Client`, `IsPrivateIP(net.IP) bool`, `Options{ConnectTimeout, TotalTimeout, MaxRedirects}` 정의.
  - dialer는 `LookupIPAddr`로 모든 IP 해소 → 한 개라도 private/reserved면 에러.
  - `CheckRedirect`은 매 hop에서 host 재해소 + private/reserved 거부 + non-http(s) scheme 거부 + maxRedirects 초과 거부.
  - private 범위: IPv4 loopback / link-local(169.254/16) / unspecified / 10.0.0.0/8 / 172.16.0.0/12 / 192.168.0.0/16 / 100.64.0.0/10 / 198.18.0.0/15 / 192.0.0.0/24 / 192.0.2.0/24 / 198.51.100.0/24 / 203.0.113.0/24 / IPv6 ::1 / fe80::/10 / fc00::/7.
- [ ] 1.2 `apps/api/internal/httpclient/ssrf_test.go` 신규 — 표 기반 테스트로 다음 검증:
  - 169.254.169.254 → 거부 (link-local)
  - 127.0.0.1 → 거부 (loopback)
  - 10.0.0.1 → 거부 (private IPv4)
  - fc00::1 → 거부 (IPv6 ULA)
  - 0.0.0.0 → 거부 (unspecified)
  - 8.8.8.8 → IsPrivateIP false (public)
  - 1.1.1.1 → IsPrivateIP false (public)
- [ ] 1.3 `httptest.NewServer`로 localhost 응답을 받는 client 호출이 dialer 차단 에러로 실패하는지 확인 (loopback 거부 통합 검증).
- [ ] 1.4 totalTimeout 발동 검증 — `httptest.NewServer`가 `time.Sleep(2s)` 후 응답하는 핸들러 + `TotalTimeout=500ms` → 에러 + `context.DeadlineExceeded` 또는 timeout 메시지.

## 2. HarvestPipeline에 SSRF-safe client 주입

- [ ] 2.1 `apps/api/internal/bot/harvest_pipeline.go` import에 `httpclient` 패키지 추가.
- [ ] 2.2 `NewHarvestPipeline` 본문의 `client: &http.Client{}` 한 줄을 `client: httpclient.NewSSRFSafeClient(httpclient.Options{ConnectTimeout: 5*time.Second, TotalTimeout: 60*time.Second, MaxRedirects: 5})`로 교체.

## 3. downloadAndUpload stream 상한 가드

- [ ] 3.1 `apps/api/internal/bot/harvest_pipeline.go` 상수 `maxMediaStreamBytes int64 = 200 << 20` 추가 (file scope).
- [ ] 3.2 `downloadAndUpload` 비-image 분기(L386-398)의 `resp.Body`를 `io.LimitReader(resp.Body, maxMediaStreamBytes)`로 감싸 storage.Upload에 전달.

## 4. spec ADDED

- [ ] 4.1 `openspec/specs/harvester/spec.md`에 `### Requirement: 외부 미디어 fetch는 SSRF-safe HTTP client를 경유한다` 신규 Requirement 추가. 5개 Scenario:
  - private/reserved IP fetch 거부 (cacheImage 경로)
  - private/reserved IP fetch 거부 (downloadAndUpload 경로)
  - public IP 정상 fetch 통과
  - redirect 응답에서 private IP로의 hop 거부
  - totalTimeout 초과 시 종료
- [ ] 4.2 `openspec validate` 통과 확인.

## 5. 사전 검증

- [ ] 5.1 `cd apps/api && go vet ./... && go build ./... && go test ./...` 모두 통과 — 특히 기존 bot/og 테스트 회귀 없음.
- [ ] 5.2 `cd apps/api && go test ./internal/httpclient/...` 신규 패키지 테스트 통과.

## 6. 실 환경 QA (proposal.md QA plan 5 단계 모두 수행)

- [ ] 6.1 docker-compose up -d로 postgres/redis/minio 기동.
- [ ] 6.2 `cd apps/api && go run cmd/server/main.go` API 기동.
- [ ] 6.3 QA 1: SSRF 차단 (cacheImage 경로) — gist/locally served HTML로 og:image=link-local 페이지 enqueue → 거부 로그/객체 미생성 확인.
- [ ] 6.4 QA 2: SSRF 차단 (downloadAndUpload 경로) — 사설 IP 미디어 링크 거부 확인.
- [ ] 6.5 QA 3: 정상 외부 fetch 회귀 — 공개 og:image 1건 정상 캐시 확인.
- [ ] 6.6 QA 4: totalTimeout 발동 — slow server에 대한 60s timeout 동작 확인 (이 항목은 시간 소모 큼; 가능하면 timeout을 1s로 임시 단축 후 검증하거나 단위 테스트 5.2로 대체).
- [ ] 6.7 QA 5: full pipeline 회귀 — 짧은 pioneer/harvester 1 cycle 실행.

## 7. 커밋 & PR & 머지 & 아카이브

- [ ] 7.1 backlog `status: in_progress` → 단일 커밋 안에 그대로 두지 않고, 본 fix 커밋과는 별도로 step 10에서 `done` 처리.
- [ ] 7.2 커밋 메시지 `fix(bot harvest_pipeline): wire SSRF-safe HTTP client + stream size cap`.
- [ ] 7.3 `git push -u origin loop-system/fix-bot-harvest-pipeline-ssrf`.
- [ ] 7.4 `gh pr create` — proposal/QA 결과 포함.
- [ ] 7.5 `merge-on-green <PR#>`.
- [ ] 7.6 `openspec archive` → `.fugue/backlog-system.yaml` 항목 `done` + `.fugue/decision-log.md` 1~3줄 추가.
