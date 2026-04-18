## Context

Pioneer는 `URLScheduler` consumer로서 큐에서 URL을 꺼내 fetch한 뒤, raw HTML에서 링크를 추출해 다시 스케줄러에 push하는 역할을 한다. 현재 raw 응답은 메모리에서 링크 파싱 후 폐기된다. 후속 단계인 Harvester는 동일 URL을 네트워크로 재요청하여 콘텐츠를 파싱하므로, 동일 원본을 두 번 fetch하는 구조다.

`apps/api/fuguebot_pseudo.go`는 향후 구조를 스케치해 두었다:
- Pioneer의 `SaveRawContent(content []byte) error` — fetch 성공 직후 호출되는 훅
- `CompositeFetcher{o ObjectStorageFetcher, h HTTPFetcher}` — object storage를 먼저 조회하고 실패 시 HTTP로 fallback

본 change는 이 중 **Pioneer 측 저장 경로**만 구현 범위로 삼는다. Harvester 측 재사용(CompositeFetcher에서 ObjectStorageFetcher 조회)은 `harvester-snapshot-first-fetch`에서 다룬다.

제약:
- 기존 Pioneer 루프 구조(`URLScheduler` consumer, BFS, dedup 정책)는 건드리지 않는다.
- 스냅샷 실패가 크롤 자체를 막으면 안 된다(사이트 공급자 장애로 크롤 전체 멈추는 블로킹 방지).
- 스토리지 비용과 콜드 저장소 수명 사이의 균형 필요.

## Goals / Non-Goals

**Goals:**
- Pioneer가 fetch 성공 시 raw 응답을 object storage에 gzip 스냅샷으로 저장한다.
- Harvester 등 후속 소비자가 normalized URL만으로 결정적으로 스냅샷 키를 계산할 수 있는 규칙을 제공한다.
- 스토리지 쓰기 실패가 Pioneer를 블로킹하지 않는다(fail-open).
- TTL 365일 lifecycle로 장기 비용을 제어한다.

**Non-Goals:**
- Harvester의 스냅샷 조회/fallback 로직(별도 change).
- 스냅샷 버전 관리/차분 저장(과거 스냅샷 diff 비교 등).
- Pioneer BFS 큐/스케줄링 정책 변경.
- 이미지/비디오 등 바이너리 미디어 저장(HTML 텍스트 응답에 한함).
- 암호화 키 관리 수준의 설계(버킷 기본 암호화 사용 전제).

## Decisions

### Decision 1: 키 규칙 — `snapshots/<sha256_hex>/<yyyymmdd>.html.gz`

- `<sha256_hex>`: normalized URL의 **sha256** digest를 hex로 인코딩한 64자 소문자 문자열. Pioneer/Harvester가 동일 해시 함수(`crypto/sha256`)와 동일 normalization 규칙을 공유한다(`bot` capability의 기존 URL normalization 재사용).
- `<yyyymmdd>`: UTC 기준 fetch 날짜. 같은 날 같은 URL을 다시 fetch하면 동일 키로 **덮어쓴다**.
- 확장자 `.html.gz`: 콘텐츠 타입과 압축 방식을 파일명으로 명시.
- 상수화: Go 코드에서 `SnapshotKeyPattern = "snapshots/%s/%s.html.gz"`로 정의하고, `SnapshotKey(normalizedURL string, t time.Time) string` 공개 함수를 제공해 Pioneer와 Harvester(`harvester-snapshot-first-fetch`)가 동일 구현을 공유한다.

**대안:**
- `snapshots/<hash>.html.gz` (날짜 없음): 동일 키 덮어쓰기만 남아 "최근 스냅샷" 개념이 희미해진다. 같은 날 여러 번 찍히는 재크롤과 날짜 기반 비교 니즈에 약함.
- `snapshots/<yyyy>/<mm>/<dd>/<hash>.html.gz` 파티셔닝: S3 listing 효율은 좋아지나, Harvester가 조회 시 날짜를 몰라 불편. 키 조회는 정확 매치가 주 유스케이스라 불필요.

채택 이유: 정확 매치 조회와 날짜별 구분이 모두 가능하고, 동일 날짜 재fetch는 idempotent 덮어쓰기로 단순해진다.

### Decision 1a: 해시 함수 — sha256 확정

- `crypto/sha256` (Go 표준 라이브러리) 사용. 외부 의존성 없이 즉시 사용 가능.
- 출력은 hex 64자 소문자. 키 경로에 그대로 노출되므로 외부 관찰 가능한 behavior contract다.
- 속도: Pioneer의 fetch 지연(네트워크 I/O) 대비 sha256 계산 비용은 무시할 수준. HTML 본문이 아닌 normalized URL 문자열(수백 바이트)만 해싱.

**Open Question 종결(sha256 vs xxh3):**
- xxh3는 더 빠르지만 `github.com/cespare/xxhash` 같은 외부 의존성이 필요하고, 본 use case의 해싱 대상이 짧은 URL 문자열이라 속도 이점이 사실상 없음.
- **sha256 채택**. 근거: (a) 표준 라이브러리만 사용 → 의존성 추가 없음, (b) URL 길이가 짧아 속도 충분, (c) 충돌 확률이 암호학적으로 무시 가능 수준이라 prefix 자르기 불필요(64자 전체 사용).

### Decision 2: 압축 — gzip (Pioneer 프로세스에서 인라인)

- 응답 바이트를 `gzip.NewWriter`로 감싸 업로드 스트림에 직접 연결.
- 별도 압축 서비스나 lambda 없이 Pioneer 프로세스에서 처리.

**대안:**
- zstd: 더 나은 비율/속도이나 object storage 클라이언트와 Harvester 측 재해석 코드가 늘어나고 Go 표준 라이브러리 외 의존성 필요. HTML 스냅샷 규모에서 gzip로 충분.
- 무압축: HTML은 텍스트라 저장 비용과 전송 비용이 2-5x로 부풀어 TTL 365일 전제에서 비용 비효율.

### Decision 3: 저장 조건 — fetch 성공(2xx + 본문 수신)만

- 4xx/5xx/네트워크 에러/타임아웃은 저장하지 않는다.
- 빈 본문(0 byte)도 저장 대상 아님(Harvester가 쓸 가치 없음).

**대안:** 실패 응답도 저장해 재시도 판단에 사용 — 별도 change에서 다룰 재시도/백오프 설계와 연계되므로 본 change에서는 out-of-scope.

### Decision 4: 쓰기 실패 시 fail-open

- object storage `Put` 실패는 경고 로그 + 메트릭 카운터만 남기고 Pioneer 루프는 링크 처리로 계속 진행한다.
- 트랜잭션/재시도 큐 없음. 다음 크롤 세션에서 자연스럽게 재작성된다.

**대안:** 실패 시 재시도 큐 — 구조 복잡도 증가. 본 change는 스냅샷을 best-effort 자료로만 정의한다. 부재·손상 시의 소비자 fallback 정책은 후속 change `harvester-snapshot-first-fetch`에서 확정한다.

### Decision 5: TTL 365일 — 버킷 lifecycle rule로 관리

- 애플리케이션 코드가 아닌 object storage bucket lifecycle policy로 만료. Pioneer는 만료 시점을 알 필요 없다.
- prefix `snapshots/` 하위에만 rule 적용.

**대안:** object별 metadata에 만료 시각 기록 후 배치 삭제 — 운영 부담만 증가, lifecycle rule이 표준 방식.

### Decision 6: 동시 쓰기 — last-write-wins (object storage 기본 동작)

- 여러 Pioneer 워커가 동일 URL을 같은 UTC 날짜에 각자 fetch하여 업로드하면, 같은 키(`snapshots/<sha256_hex>/<yyyymmdd>.html.gz`)에 대해 동시 PUT이 발생할 수 있다.
- 처리 정책: **object storage의 기본 atomic PUT 동작을 따른다**. 마지막에 commit된 PUT이 최종 객체로 남는다(last-write-wins).
- 애플리케이션 레벨에서 lock, If-Match/If-None-Match 헤더, versioning 사용 안 함.
- 근거: 같은 URL의 같은 날 스냅샷은 내용이 거의 같고, Harvester가 요구하는 것은 "그날의 HTML 한 벌"이지 특정 워커의 결과물이 아니다. 일관성보다 단순성이 우선.

### Decision 7: checksum 검증 — Pioneer 측은 별도 검증을 두지 않는다

- Pioneer는 object storage 업로드 전/후 별도 MD5/SHA checksum 비교를 수행하지 않는다.
- gzip 포맷 자체가 trailer에 CRC-32를 포함하므로, 저장 형식 자체에 손상 감지 수단이 이미 내재한다.
- 소비자(Harvester)가 손상을 감지했을 때의 fallback 정책(예: snapshot miss 취급 + HTTP fallback)은 **본 change의 책임 범위 밖**이며 후속 change `harvester-snapshot-first-fetch`에서 정의한다. 본 change는 해당 소비자 동작을 선결정하지 않는다.

**대안:** S3 `Content-MD5` 헤더 + 서버측 검증 — 구현 비용 대비 이득 제한적. 저장 형식(gzip trailer CRC)이 이미 손상 감지 수단을 제공하므로 Pioneer 측 추가 검증은 불필요.

## Risks / Trade-offs

- **[스토리지 비용 증가]** → TTL 365일 + gzip 압축 + HTML에 한정으로 예산을 제한한다. 필요 시 TTL을 당기는 정책을 후속 change에서 조정.
- **[같은 날 여러 번 fetch 시 최신 스냅샷만 남음]** → 날짜 단위 덮어쓰기는 의도된 동작. 시간 단위 이력이 필요해지면 키 규칙을 `<yyyymmddHHMM>`으로 확장하되, 현재 니즈는 "가장 최근 하루치"로 충분.
- **[스냅샷과 그래프 불일치]** → Pioneer가 저장에 실패하면 Harvester가 스냅샷을 못 찾고 HTTP로 폴백하므로 정합성은 Harvester가 담당. 본 change는 best-effort 경로만 책임.
- **[URL normalization 불일치로 해시 충돌/누락]** → Pioneer와 Harvester가 동일 normalization 함수를 공유해야 한다. 기존 `bot` capability의 URL 정규화 규칙을 그대로 재사용하며, 변경 시 두 소비자 모두 영향 받음을 문서화.
- **[민감한 HTML이 장기 보관됨]** → 버킷은 비공개 + 서버측 암호화 전제. 개인정보가 포함된 페이지는 애초 크롤 제외 대상(기존 `bot` 정책). TTL 365일이 보존 한도.
- **[동일 키에 대한 동시 PUT]** → 여러 Pioneer 워커가 동일 URL을 같은 UTC 날짜에 동시에 업로드할 수 있으나, object storage의 기본 atomic PUT 동작(last-write-wins)을 따른다. 별도 lock/version 관리는 두지 않는다. 같은 날 같은 URL의 스냅샷은 내용이 거의 같다는 전제 하에 수용 가능한 risk로 기록.

## Migration Plan

1. 버킷/prefix(`snapshots/`) 및 365일 TTL lifecycle rule 구성.
2. Feature flag(예: `PIONEER_SNAPSHOT_ENABLED`) 도입하여 단계적 롤아웃. 초기 off → 스테이징에서 on → 운영 on.
3. 배포 후 24시간 모니터링: 업로드 성공률, 평균 업로드 지연, 스토리지 증가량.
4. 롤백: feature flag off. 키 규칙은 문서화된 상태로 남기고, 저장된 스냅샷은 TTL로 자연 소멸.

## Open Questions

- (해결 완료) 해시 함수 선택(sha256 prefix vs xxh3)과 prefix 길이 → Decision 1a 참조.
- feature flag의 영구화 여부: 장기적으로 on-by-default 이후 삭제할지, 운영 toggle로 유지할지. **결정 시점**: `harvester-snapshot-first-fetch` 배포 완료 후 본 change의 롤아웃 마감 단계(tasks 6)에서 결정 기록 남긴다.
- 버킷 선택: 기존 미디어 버킷과 동일 물리 bucket + prefix 분리로 갈지, 전용 bucket을 만들지(IAM/lifecycle 분리 편의). **운영 시점 결정**(terraform/helm에서 확정). 본 change의 키 규칙(`snapshots/` prefix)은 두 선택지 모두와 호환.
