# Design: fix-bot-harvest-pipeline-ssrf

## D1. SSRF 보호의 위치 — dialer 단일 지점

대안:
- (A) HTTP client `Transport.DialContext`에서 IP 검사 — 모든 redirect / connection / DNS 결과에 자동 적용.
- (B) caller가 URL 입력 시점에 host의 IP를 LookupIPAddr로 검사.
- (C) caller가 `httptrace`로 connect 시점 hook으로 검사.

채택: A.

근거:
- B는 redirect를 follow하는 매 hop에 대해 caller가 직접 재검증 코드를 작성해야 하고, DNS rebinding(검증 후 connect 사이에 DNS 결과가 변경)을 막지 못한다.
- C는 `httptrace.GetConn`만으로 거부할 수 없고, `ConnectStart` 이후 실제 dial은 이미 진행됨.
- A는 og/service.go에 이미 검증된 구현이 있다. `LookupIPAddr` → 모든 IP에 대해 private 검사 → `ips[0]`로 직접 dial. CheckRedirect도 같은 검사를 매 hop에 수행.

## D2. 코드 중복 — og를 즉시 마이그레이션할 것인가

대안:
- (A) `httpclient/` 신설하고 bot만 사용. og는 자체 dialer 유지(동일 로직 두 곳).
- (B) `httpclient/` 신설하고 og도 곧바로 마이그레이션.

채택: A.

근거:
- B는 og의 기존 unit test와 production 동작에 의존하는 호출자(`og.Service`)가 정상 회귀하는지 추가 QA가 필요. 본 cycle의 핵심 목적(bot HarvestPipeline SSRF 차단)과 직교한 리스크가 발생.
- A는 두 사이클로 분리해 변경 폭을 작게 유지. 일시적인 중복은 다음 cycle에서 og 마이그레이션 시 한 줄 교체로 해소된다.
- 후속 backlog 후보(별도 등록 필요): `system-yyyymmdd-og-service-use-shared-ssrf-client`. 본 change archive 시점에 발견 사이클에 자동 추가하지 않는다(루프 규칙: cycle 1건만).

## D3. 거부 IP 범위 — og의 범위를 그대로 차용

og/service.go L298-340이 거부하는 범위:
- IPv4 loopback (127.0.0.0/8)
- IPv6 loopback (::1)
- Link-local (169.254.0.0/16, fe80::/10) — **AWS IMDS 169.254.169.254 차단의 핵심**
- Unspecified (0.0.0.0, ::)
- IPv4 private (10/8, 172.16/12, 192.168/16)
- Carrier-grade NAT (100.64.0.0/10)
- Benchmarking (198.18.0.0/15)
- IETF Protocol Assignments (192.0.0.0/24)
- Documentation (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24)
- IPv6 ULA (fc00::/7)

채택: 위 범위 그대로 유지. 추가 / 제외 없음.

근거: og가 이 범위를 채택한 결정은 이미 production에서 통과하고 있다. 본 cycle은 wiring 차이를 좁히는 목적이라 정책 범위 변경은 별개 결정사항. multicast/anycast(예: 224.0.0.0/4)는 og가 명시적으로 거부하지 않으므로 본 cycle도 동일.

## D4. Timeout 값 — og(5s)는 미디어에 부적합

og.totalTimeout = 5s는 HTML 페이지(보통 < 100 KB) fetch에 맞춘 값. 미디어는 cacheImage가 최대 20 MiB, downloadAndUpload 비-image 분기는 무제한 stream이므로 5s에 대부분 timeout된다.

채택:
- `ConnectTimeout = 5s` (og 3s보다 약간 여유 — 미디어 사이트가 응답 시작이 느릴 수 있음)
- `TotalTimeout = 60s` (20 MiB / 60s = 약 333 KB/s 하한 — 글로벌 CDN 평균보다 낮아 정상 트래픽이 timeout되지 않음)
- `MaxRedirects = 5` (og와 동일)

근거: 60s는 정상 미디어 다운로드에 충분하면서 slow-loris류 공격이 워커를 무한 점유하는 것을 막는다. 추후 운영 데이터로 p95 다운로드 시간을 측정해 조정 가능 — 환경 변수로 노출하지 않는 이유는 default가 충분히 보수적이라 튜닝 필요성이 명확해질 때 wiring을 추가하기 위함(YAGNI).

## D5. Stream 상한 — downloadAndUpload 비-image 분기

현재 `downloadAndUpload` 비-image 분기(L386-398)는:
```go
size := resp.ContentLength
uploadedURL, err := p.storage.Upload(ctx, filename, contentType, size, resp.Body)
```
`resp.Body`를 그대로 storage.Upload로 stream. `storage.Upload`는 `size > limit` 검증만 하고 stream에는 LimitReader를 걸지 않음. 서버가 Content-Length를 거짓으로 작게 응답하면 실제 stream은 무제한.

채택: stream을 `io.LimitReader(resp.Body, maxMediaStreamBytes)`로 감싼다. `maxMediaStreamBytes = 200 << 20` (200 MiB — `storage.maxBytes[MediaVideo]`의 2배 여유). 200 MiB를 넘기는 응답은 stream 도중 EOF로 끊겨 storage.Upload가 짧은 객체 적재로 실패 → 호출자가 에러로 인식.

근거: 200 MiB가 정상 case 상한과 같으면 정상 응답도 정확히 cap에 닿을 때 false reject 가능. 2배 여유로 두면 storage layer의 declared-size 검증이 최종 가드로 작동. 향후 음원/영상 한도가 상향되면 이 상수도 함께 올린다.

## D6. spec.md ADDED Requirement 위치

`openspec/specs/harvester/spec.md`에 ADDED.

근거: `HarvestPipeline`은 harvester capability의 일부 — Pioneer가 enqueue한 작업을 Harvester가 처리하면서 호출하는 pipeline이다. og/service.go는 별개 capability(현재 spec 미존재)라 og 측 ADDED는 본 change 범위 밖. Pioneer의 `fetchHTMLShared`는 별개 backlog 후보.

## D7. 위협 모델 — 본 cycle이 차단하는 것 / 차단하지 못하는 것

차단:
- ✅ og:image / media_url에 metadata IP(169.254.169.254) 직접 지정 → S3 mirror → 공개 노출
- ✅ og:image에 사설 IP(10/8 등) 지정 → S3 mirror → 공개 노출
- ✅ 외부 사이트가 redirect 응답으로 사설 IP 노출 (CheckRedirect 매 hop 재해소)
- ✅ DNS rebinding으로 검증 시점 IP와 connect 시점 IP가 다른 공격 (dialer에서 LookupIPAddr 후 직접 dial)
- ✅ slow-loris로 워커 점유 (TotalTimeout 60s)
- ✅ Content-Length 거짓 응답으로 stream 무제한 (LimitReader 200 MiB)

차단하지 못함 (별도 cycle):
- ❌ Pioneer의 HTML 페이지 fetch(`fetchHTMLShared`)를 통한 blind SSRF (HTML 본문이 어디에도 echo되지 않아 disclosure 체인은 끊김 — 단 timing 채널은 남음)
- ❌ 공개 IP를 가진 외부 프록시로의 reflection(예: 공개 SSRF gateway 통한 사설망 우회)
- ❌ IMDSv2 토큰 강제(인프라 레이어)
- ❌ 이미 적재된 `pins.og_image`/`media_url`의 forensic scan(별도 사고 대응 사이클)
