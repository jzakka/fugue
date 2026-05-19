# fix-bot-harvest-pipeline-ssrf

## Problem

`apps/api/internal/bot/harvest_pipeline.go:146`은 `HarvestPipeline.client`을 `&http.Client{}`(bare client)로 초기화한다. 이 client는 다음 두 메서드에서 외부 사이트로부터 추출된 — 즉, 공격자가 제어 가능한 — URL을 fetch한다:

- `downloadAndUpload` (L337-399): `mediaURL` 인자. Pioneer가 페이지에서 추출한 미디어 URL.
- `cacheImage` (L450-513): `candidateURL` 인자. PickPrimaryImage가 페이지의 `og:image` / `twitter:image` 등에서 추출한 후보 URL.

두 메서드는 fetch 응답 바이트를 그대로 `p.storage.Upload`로 S3/MinIO 객체로 저장하고, 반환된 storage URL을 호출자(`createPin`/`PinDocument`)가 `pins.media_url` · `pins.og_image` 컬럼에 적재한다. 그 컬럼들은 공개 Pin 조회 API의 응답 본문으로 그대로 흘러간다.

bare `&http.Client{}`에는 (a) `Timeout`이 없고 (b) `CheckRedirect`가 default라 무제한 redirect 허용 (c) `Transport.DialContext`가 default라 private/reserved IP로의 outbound 연결을 막지 않는다. 결과적으로 다음 SSRF → public-disclosure 체인이 성립한다:

1. 외부 사이트 A가 자신의 HTML에 `<meta property="og:image" content="http://169.254.169.254/latest/meta-data/iam/security-credentials/<role>">` 같은 메타 태그를 노출.
2. Pioneer가 A를 크롤 → `PickPrimaryImage`가 위 og:image URL을 후보로 선정 → snapshot_key와 함께 `EnqueueHarvester`.
3. Harvester가 dequeue → `HarvestPipeline.processOne` → `cacheImage`가 `p.client.Do(req)`로 169.254.169.254에 직접 GET → AWS IMDS 응답 JSON을 S3에 `PutObject`.
4. `cacheImage` 반환값(storage URL)이 Pin 행의 `og_image`에 저장됨.
5. 공개 Pin 조회 API가 그 storage URL을 응답에 포함 → AWS IAM 임시 자격증명(액세스 키 + 시크릿 + 세션 토큰)이 인터넷에 공개됨.

`downloadAndUpload` 경로도 동일 — `pins.media_url`이 외부 노출 컬럼이므로 임의 internal endpoint 응답(예: K8s API 서버 응답, 사내 어드민 페이지)이 같은 방식으로 노출 가능하다.

추가 부수 위험:
- Timeout 부재로 slow-loris 응답을 보내는 사이트가 워커 1개를 영구 점유 → harvester 처리량 저하 (worker budget이 100 dequeue로 종료될 때까지 stall).
- imageCacheMaxBytes (20 MiB 기본)이 `cacheImage` 본문에 io.LimitReader로 적용되지만 redirect 시 매 hop의 상한이 아니며, `downloadAndUpload`의 비-image 분기(video/audio)는 `resp.Body`를 직접 storage로 stream하면서 `io.LimitReader`도 없다(`apps/api/internal/storage/storage.go`의 maxBytes 검증은 declared `size` 인자로만 트리거되므로 stream에서는 미동작).

같은 저장소 내부 `apps/api/internal/og/service.go:50-340`은 동일 신뢰 경계(외부 입력 URL fetch)에 대해 이미 SSRF-safe dialer를 구현 중이다:

- 모든 IP에 대해 `LookupIPAddr` 후 private/loopback/link-local/reserved 범위 거부
- `CheckRedirect` 매 hop에서 재해소 + private IP 거부 + non-http(s) scheme 거부
- 명시적 `connectTimeout` (3s) + `totalTimeout` (5s)
- maxRedirects = 5

이는 프로젝트 내부에 합의된 "외부 HTTP fetcher의 SSRF 방어 패턴"이다. Bot의 `harvest_pipeline.go`는 같은 경계에서 그 패턴을 따르지 않는다.

## Solution

신규 패키지 `apps/api/internal/httpclient`에 SSRF-safe `*http.Client` 팩토리를 추출한다. `og/service.go`의 dialer/CheckRedirect/isPrivateIP 로직을 그대로 가져와 일반화한다.

```go
// apps/api/internal/httpclient/ssrf.go
package httpclient

func NewSSRFSafeClient(opts Options) *http.Client { ... }
func IsPrivateIP(ip net.IP) bool { ... }

type Options struct {
    ConnectTimeout time.Duration
    TotalTimeout   time.Duration
    MaxRedirects   int
}
```

`apps/api/internal/bot/harvest_pipeline.go`의 `NewHarvestPipeline`이 `&http.Client{}` 대신 `httpclient.NewSSRFSafeClient(Options{ConnectTimeout: 5s, TotalTimeout: 60s, MaxRedirects: 5})`를 주입한다. 미디어 다운로드 특성상 og의 5s totalTimeout보다 길게 잡되, 무제한은 아니다.

`downloadAndUpload` 비-image 분기에 `io.LimitReader(resp.Body, maxMediaStreamBytes)`로 stream 상한을 명시(예: 200 MiB — `storage.maxBytes[MediaVideo]` = 100 MiB의 2배 여유). storage 측 size 검증이 declared size에만 의존하므로 stream 측 1차 가드가 필요하다.

`og/service.go`를 동일 helper로 마이그레이션하는 작업은 **본 change 범위 밖** — 기존 og 동작/테스트 회귀 위험 회피. 일시적으로 SSRF 로직이 두 곳에 존재하나, 후속 change에서 og가 helper를 import하도록 정리한다(별도 backlog 후보로 등록).

## Why

같은 신뢰 경계(외부 URL fetch)에서 패키지마다 보호 수준이 다른 것은 결함이다. og가 SSRF-safe dialer를 가진다는 사실은 프로젝트가 이 클래스 위험을 인식하고 있음을 뜻하므로, bot가 그 보호를 갖지 않는 것은 spec 누락이 아닌 wiring 누락이다.

handler / consumer 레이어에서 URL 입력을 검증(예: og:image가 internal IP인지 사전 거부)하는 대신 HTTP client 단일 지점에서 dialer가 거부하도록 두면 (a) 모든 redirect hop에서 자동 적용 (b) DNS rebinding 회피(연결 시점 IP 검사) (c) `downloadAndUpload`/`cacheImage` 외 향후 추가될 외부 fetch 경로도 자동 보호 — 세 측면에서 단일 SSoT.

## Scope

- 신규 파일 `apps/api/internal/httpclient/ssrf.go` — SSRF-safe client 팩토리 + `IsPrivateIP` 헬퍼.
- 신규 파일 `apps/api/internal/httpclient/ssrf_test.go` — dialer가 IPv4 private / IPv6 ULA / loopback / link-local / metadata IP(169.254.169.254) 모두 거부하는지, CheckRedirect가 redirect → private IP를 거부하는지, public IP는 통과하는지, totalTimeout이 발동하는지 검증.
- `apps/api/internal/bot/harvest_pipeline.go` — `NewHarvestPipeline` 한 줄 변경(`&http.Client{}` → `httpclient.NewSSRFSafeClient(...)`).
- `apps/api/internal/bot/harvest_pipeline.go` — `downloadAndUpload`의 비-image 분기 stream에 `io.LimitReader`로 200 MiB 상한 추가.
- `openspec/specs/harvester/spec.md` — 신규 Requirement: `외부 미디어 fetch는 SSRF-safe HTTP client를 경유한다` (5 Scenarios).

## Out of scope

- `og/service.go` 마이그레이션(별도 follow-up backlog).
- Pioneer의 `fetchHTMLShared` (`apps/api/internal/bot/helpers.go`) SSRF 가드 — HTML 본문이 외부 응답으로 echo되지 않아 disclosure 체인이 약함. 별도 backlog 후보로 분리.
- HEAD/Range 기반 사전 size probing — `cacheImage`는 Content-Length 사전 검사를 이미 수행. video/audio 분기의 추가 probing은 효과 대비 복잡.
- AWS IMDSv2(토큰 강제) 적용 — 인프라 레이어 결정으로 별개.
- `pins.og_image`/`pins.media_url` 컬럼에 이미 적재된 과거 데이터의 forensic scan — 사고 대응 사이클 별개.

## Rollback

`apps/api/internal/bot/harvest_pipeline.go`의 한 줄을 `&http.Client{}`로 되돌리고 `httpclient` 패키지 import를 제거하면 동작이 즉시 이전 상태로 복원된다. `httpclient` 패키지 자체는 import가 사라지면 dead code로 남으므로 별도 삭제 가능. 응답 contract / 정상 fetch 경로(공개 IP의 외부 사이트) 동작은 동일 — 정상 미디어 캐시 회귀 없음.

## QA plan

처리 모드 §7 실 환경 QA:

1. **SSRF 차단 검증 (cacheImage 경로)**
   - docker-compose로 api+postgres+minio+redis 기동.
   - 호스트 머신에 가짜 metadata 응답을 serve하는 `python3 -m http.server 8081`을 띄우고, 다른 포트에 `<html><head><meta property="og:image" content="http://127.0.0.1:8081/latest/meta-data/test-secret"></head></html>` 페이지 serve.
   - `harvester_frontier`에 위 페이지 URL을 enqueue → harvester를 짧은 duration으로 1회 실행.
   - 검증: `python3` 서버 access log에 169.254/127.0.0.1 GET 기록 **없음**, minio에 `images/<hash>/<ts>.html.gz` 같은 객체 생성 **없음**, harvester 로그에 "blocked private/reserved IP" 메시지 1건.
2. **SSRF 차단 검증 (downloadAndUpload 경로)**
   - 동일 셋업, 페이지에 `<meta property="og:image" content="<public valid image>"> <a href="http://10.0.0.1/private.mp4">` 형태로 정상 og:image + 사설 IP 미디어 링크 혼합.
   - 검증: og:image 정상 캐시, mp4 fetch는 거부, Pin row의 media_url이 빈 값 또는 fallback 처리.
3. **정상 외부 fetch 회귀**
   - public IP 외부 사이트(예: `https://github.githubassets.com/favicon.ico`)를 og:image로 가진 페이지 1건을 enqueue.
   - 검증: `psql`로 `SELECT og_image FROM pins WHERE source_url = '<test url>'` → minio storage URL 적재. minio에 객체 1건 존재.
4. **Timeout 발동**
   - 호스트에 1Hz로 1바이트씩 응답하는 slow server를 띄우고 og:image로 지정.
   - 검증: 60초 내 cacheImage가 timeout 에러로 종료, harvester 워커가 다음 작업으로 진행.
5. **회귀: 일반 핀 생성 (Pioneer→Harvester full pipeline 1회)**
   - `make crawl` 또는 짧은 duration의 pioneer/harvester 직접 실행 1회.
   - 검증: 정상 페이지 1건이 새 Pin으로 적재되고 og_image 컬럼에 storage URL 채워짐.
