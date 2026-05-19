## ADDED Requirements

### Requirement: 외부 미디어 fetch는 SSRF-safe HTTP client를 경유한다

Harvester가 외부 사이트로부터 추출된 — 즉, 호출자가 제어할 수 없는 — URL에 대해 미디어 바이트를 직접 가져와 객체 저장소에 적재할 때, 그 HTTP fetch는 SSRF-safe HTTP client를 경유해야 한다(SHALL). SSRF-safe HTTP client는 다음을 모두 enforce한다:

- 매 outbound 연결의 dial 단계에서 대상 호스트의 모든 해소된 IP가 private/reserved 범위(loopback, link-local, IPv4 RFC1918, IPv6 ULA, unspecified, carrier-grade NAT, benchmarking, documentation 등)에 속하면 연결을 거부한다(SHALL).
- HTTP redirect의 매 hop마다 대상 host를 재해소하고 같은 검사를 반복한다(SHALL).
- 명시적 connect timeout과 total timeout을 가진다(SHALL). 응답이 그 안에 완료되지 않으면 client는 에러로 종료한다(SHALL).
- 명시적 최대 redirect 횟수를 초과하면 에러로 종료한다(SHALL).
- non-`http`/`https` scheme으로의 redirect를 거부한다(SHALL).

Harvester가 외부 미디어 응답 본문을 stream으로 객체 저장소에 전달할 때는 명시적인 stream 크기 상한 가드를 갖춰야 한다(SHALL). 외부 서버가 `Content-Length`를 거짓으로 작게 응답하더라도 실제 stream된 바이트 수가 상한을 넘으면 fetch가 끊겨야 한다(SHALL).

본 Requirement는 기존 Requirement `Generic HTML→Pin extractor가 default 변환 경로다`가 추출하는 `og:image`/`twitter:image`/JSON-LD `schema.org` image 후보 URL과, 그 외 페이지에서 추출되는 임의 미디어 URL에 대한 fetch 경로를 모두 포괄한다. 기존 Scenario의 추출 / 우선순위 / 정규화 정의는 변경하지 않는다.

#### Scenario: 후보 이미지 URL이 private/reserved IP를 가리키면 거부

- **WHEN** 외부 페이지의 `og:image`가 `http://169.254.169.254/latest/meta-data/...` 같은 link-local IP URL이고 Harvester가 그 URL을 cacheImage 경로로 fetch 하려고 시도하면
- **THEN** SSRF-safe client의 dialer가 dial 직전에 IP 검사로 연결을 거부하고, 외부 저장소(S3/MinIO)에 해당 응답 바이트가 객체로 적재되지 않는다. cacheImage는 fallback 경로(원본 candidate URL 그대로 반환)로 진입한다.

#### Scenario: 미디어 URL이 사설 IP를 가리키면 거부

- **WHEN** 외부 페이지에서 추출된 `mediaURL`이 `http://10.0.0.1/...` 같은 IPv4 private 범위 IP URL이고 Harvester가 downloadAndUpload 경로로 fetch 하려고 시도하면
- **THEN** SSRF-safe client의 dialer가 연결을 거부하고, 외부 저장소에 해당 응답 바이트가 적재되지 않으며, 호출자는 fetch 에러로 인식한다.

#### Scenario: redirect 응답에서 사설 IP로의 hop을 거부

- **WHEN** 외부 서버가 200 OK 대신 302 Location: `http://192.168.1.1/...` 같은 응답을 돌려보내고 Harvester가 그 redirect를 follow 하려고 시도하면
- **THEN** SSRF-safe client의 CheckRedirect 콜백이 hop의 host를 재해소하여 사설 IP 매핑을 감지하고 redirect를 거부한다. 외부 저장소에 사설 호스트 응답 바이트가 적재되지 않는다.

#### Scenario: 공개 IP를 가리키는 정상 미디어는 통과

- **WHEN** 외부 페이지의 `og:image`가 공개 IP를 가진 정상 CDN URL(예: `https://github.githubassets.com/favicon.ico`)이고 Harvester가 cacheImage 경로로 fetch 하면
- **THEN** SSRF-safe client는 정상 응답을 받아 객체 저장소에 적재하고 storage URL을 반환한다. 이전 동작과 동일.

#### Scenario: total timeout 초과 시 fetch가 종료된다

- **WHEN** 외부 서버가 응답 본문을 매우 느린 속도(예: 1바이트/초)로 전송하여 SSRF-safe client에 설정된 total timeout 안에 응답이 끝나지 않는 경우
- **THEN** client는 timeout 에러로 종료하고 호출자는 fetch 에러로 인식하며, Harvester 워커는 다음 작업으로 진행한다. 외부 저장소에는 미완료 부분 응답이 적재되지 않는다.
