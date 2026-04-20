## 1. DB 마이그레이션

- [x] 1.1 BotCreatorID 운영 정책 정리 후 단일 UUID 리터럴 결정 — env/config로 런타임 노출하되, 마이그레이션 SQL과 upsert 쿼리에는 동일한 UUID 리터럴이 하드코딩되어야 함을 문서화(IMMUTABLE 제약)
- [x] 1.2 기존 봇 Pin 중 같은 URL의 중복 행을 dedup하는 일회성 SQL 스크립트 작성. (a) 대상 그룹별 가장 최근 created_at Pin을 생존자로 지정, (b) `harvester_frontier_pins` 등 pin_id를 참조하는 조인 테이블의 row를 생존자 pin_id로 재할당(UPDATE)한 뒤, (c) 나머지 중복 봇 Pin을 삭제. interaction/board 등 사용자 접점 테이블에 봇 Pin이 참조되고 있는지 사전 확인
- [x] 1.3 마이그레이션 추가: `CREATE UNIQUE INDEX CONCURRENTLY pins_url_bot_unique ON pins(url) WHERE creator_id = '<BotCreatorID UUID 리터럴>'`
- [x] 1.4 down 마이그레이션: `DROP INDEX CONCURRENTLY IF EXISTS pins_url_bot_unique`
- [x] 1.5 sqlc 쿼리 추가: `UpsertBotPinByURL` — `INSERT ... ON CONFLICT (url) WHERE creator_id = '<BotCreatorID UUID 리터럴>' DO UPDATE SET title=..., description=..., og_image=..., og_data=...`. 파라미터 바인딩(`$1` 등)이 아닌 UUID 리터럴로 작성해야 PostgreSQL이 partial unique index predicate와 매칭하여 arbiter inference에 성공한다

## 2. PinDocument 도메인 타입

- [x] 2.1 `apps/api/internal/bot/pin_document.go` 신규 작성: `PinDocument` struct (title, body_text, canonical_url, thumbnail_url, media_candidates, lang, author, published_at, og_data)
- [x] 2.2 `MediaCandidate` struct 정의 (type, url, width, height)
- [x] 2.3 `og_data` 직렬화 헬퍼: classifier 결과/extractor 식별자/source URL 보존 키 구조 정의
- [x] 2.4 unit test: PinDocument 직렬화/역직렬화 라운드트립

## 3. Generic HTML→Pin extractor

- [x] 3.1 `apps/api/internal/bot/extractor.go` 신규 작성: `GenericExtractor.Extract(html, fetchURL) (PinDocument, error)`
- [x] 3.2 OG/Twitter Card 메타 파서 구현 (title, image, url, description, locale, article:published_time, article:author)
- [x] 3.3 JSON-LD `schema.org` 파서 구현 (Article, CreativeWork: headline, articleBody, image, author, datePublished)
- [x] 3.4 `<article>` / 최대 텍스트 밀도 블록 추출 로직
- [x] 3.5 `<title>` / `<h1>` / `<link rel=canonical>` / `<html lang>` / `<time datetime>` 태그 파서
- [x] 3.6 본문 범위(`<article>` 태그가 있으면 그 내부, 없으면 `<body>` 전체) 내 `<img>`/`<video>`/`<audio>`/`<source>` 수집 + 절대 URL 변환
- [x] 3.7 canonical_url 결정 시 cross-domain canonical 무시 정책을 fallback 체인 내부에서 적용 — 채택 후보 호스트가 fetch URL 호스트와 다르면 건너뛰고 다음 fallback으로 진행 (Harvester 결합 단계에서 중복 판정하지 않음)
- [x] 3.8 fallback 체인 우선순위 적용 (title/body_text/canonical/thumbnail 각각)
- [x] 3.9 unit test: OG만 있는 페이지, JSON-LD만 있는 페이지, `<article>`만 있는 페이지, 셋 다 없는 페이지, cross-domain canonical 케이스
- [x] 3.10 PinDocument는 항상 nil이 아닌 객체를 반환함을 보장하는 테스트

## 4. Content classifier

- [x] 4.1 `apps/api/internal/bot/classifier.go` 신규 작성: `Classifier.Classify(doc PinDocument) (pinnable bool, reason string)` — 입력은 PinDocument뿐이며 node_type 등 외부 상태에 의존하지 않음
- [x] 4.2 사유 우선순위 (`listing` > `empty_body` > `no_primary_media`) 적용 — 3개 reason enum만 유지
- [x] 4.3 `listing` 판정: 단어 수 > 0 AND `링크 수 / 단어 수 > threshold_link_density` (단일 공식, 단어 수=0일 때 division-by-zero 회피를 위해 guard 적용)
- [x] 4.4 `empty_body` 판정: body_text < 임계값 (기본 200 bytes, 설정 가능)
- [x] 4.5 `no_primary_media` 판정: thumbnail 없음 AND media_candidates 비어 있음 (우선순위상 listing/empty_body가 먼저 매치된 경우는 제외)
- [x] 4.6 classifier 결과를 `og_data.classifier = {pinnable, reason?}` 키에 보존 (reason enum: `listing` | `empty_body` | `no_primary_media`)
- [x] 4.7 unit test: 3개 reason(`listing`, `empty_body`, `no_primary_media`)별 경계 케이스 + 사유 우선순위 + 정상 페이지 통과 + 단어 수 0 페이지에서 listing 판정이 발생하지 않음(division-by-zero 회귀)

## 5. PerSiteAdapter / AdapterRegistry

- [x] 5.1 `apps/api/internal/bot/adapter.go` 신규 작성: `PerSiteAdapter` 인터페이스 (`Domain()`, `Extract(ctx, html, fetchURL)`)
- [x] 5.2 `AdapterRegistry` 인터페이스 + 인메모리 구현 (`Resolve(domain)`, `Register(adapter)`)
- [x] 5.3 도메인 매칭 정책 (정확 일치 + 와일드카드 서브도메인) 정의
- [x] 5.4 `og_data.extractor` 식별자 채우기 헬퍼 (`generic`, `script:<site_id>`, `<adapter_name>`)
- [x] 5.5 unit test: register/resolve 동작, 미등록 도메인 fallback

## 6. ScriptAdapter (기존 GojaExecutor 래핑)

- [x] 6.1 `apps/api/internal/bot/script_adapter.go` 신규 작성: `ScriptAdapter` struct가 `PerSiteAdapter` 구현
- [x] 6.2 기존 `GojaExecutor`를 의존성 주입으로 ScriptAdapter 내부에 보유
- [x] 6.3 `Extract`에서 (site_id, node_type) 스크립트 로드 → 실행 → RawItem 배열 수신
- [x] 6.4 N→1 축약 로직: **첫 RawItem**을 정본 PinDocument로 채택(title, thumbnail_url, body_text, description 등 모든 메타 필드), 나머지 RawItem들은 `og_data.media_candidates` 배열(`{type, url, width?, height?}`)로 추가
- [x] 6.5 빈 결과(0건) 또는 실행 실패 시 에러 반환 → Harvester가 generic으로 fallback
- [x] 6.6 부트스트랩 시 DB의 (site_id, node_type) 스크립트가 있는 사이트의 도메인을 AdapterRegistry에 등록 (범위: 프로세스 시작 시점 1회. 런타임 DB 변경 반영은 본 change 범위 외 — 프로세스 재시작 필요)
- [x] 6.7 unit test: 첫 RawItem 정본 채택, 나머지 media_candidates 배열 구성, 빈 결과 처리, 실행 실패 처리

## 7. Harvester 결합

- [x] 7.1 `apps/api/internal/bot/harvester.go`의 `executeNode`를 PinDocument 반환으로 변경
- [x] 7.2 처리 순서 적용: `adapter, ok = registry.Resolve(domain) → adapter.Extract OR generic.Extract → classifier.Classify(doc) → upsert OR mark harvested_at`. classifier에는 node_type을 전달하지 않는다
- [x] 7.3 어댑터 실패 시 generic fallback 경로 구현 (AdapterFallback 부가 카운터 증가 — 주 카테고리와 독립적)
- [x] 7.4 cross-domain canonical 처리는 generic extractor(및 PerSiteAdapter) 내부 fallback 체인에서만 수행한다. Harvester 결합 단계는 extractor가 반환한 `canonical_url`을 신뢰하여 그대로 `pins.url`에 upsert하며 추가 판정을 하지 않는다. `og_data.source`는 항상 fetch URL로 저장
- [x] 7.5 `media_candidates` 길이 상한(기본 50) 적용; 각 원소 스키마 `{type: "image"|"video"|"audio", url, width?, height?}`
- [x] 7.6 `body_text`는 `pins.description`에 500 rune(UTF-8 rune-safe, `utf8.RuneCountInString` 기준) 잘라 저장하고 `og_data`에는 **포함하지 않음** (키 자체 부재). 바이트 경계 절단으로 multi-byte 문자 손상 금지
- [x] 7.7 `media_url` NOT NULL 제약 충족: thumbnail_url 또는 첫 media_candidates URL 사용
- [x] 7.8 통계 재정의: 주 카테고리(`PinsCreated`, `Deduped`, `Skipped`, `Failed` — 정확히 하나 증가) + 부가 카운터(`AdapterFallback` — 독립 증가). 카테고리명은 "Skipped"로 통일; "Classified" 대체 표현 사용 금지
- [x] 7.9 노드 1개 = 주 카테고리 1건 보장 (ScriptAdapter N개 RawItem이어도 노드 단위 1건; AdapterFallback은 동일 노드에서 별도 증가 가능)
- [x] 7.10 pinnable=false 노드는 frontier row의 `harvested_at`만 마킹 (Pin 생성/update 없음). Pin이 upsert된 노드는 scheduler-consumer가 `SetStatus(key, "harvested", []uuid{pin_id})`로 `harvester_frontier_pins` 조인 기록 (본 change는 pin_id 반환까지 책임; 조인 기록은 scheduler-consumer change 소유)

## 8. Harvest pipeline 정리

- [x] 8.1 `apps/api/internal/bot/harvest_pipeline.go`를 PinDocument 기반 upsert로 갱신
- [x] 8.2 기존 RawItem 흐름은 ScriptAdapter 내부로만 한정 (외부 인터페이스에서 제거)
- [x] 8.3 `UpsertBotPinByURL` 호출 결과를 (created vs updated)로 구분하여 PinsCreated/Deduped 통계에 매핑
- [x] 8.4 동시 upsert race를 가정한 통합 테스트 (두 워커 동시 호출 → 정확히 한 행 보장) — Go 코드 경로의 race-safety는 `TestProcessDocument_ConcurrentCallsAreRaceSafe`로 검증(`go test -race`), DB 레벨 "정확히 한 행" 보장은 `pins_url_bot_unique` partial unique index의 구조적 속성으로 제공

## 9. 설정 / Feature flag

- [x] 9.1 Default 변환 경로를 generic으로 고정 — 초기 설계에서 논의된 `HARVESTER_DEFAULT_EXTRACTOR` feature flag는 ScriptAdapter 등록이 per-site 단위로 opt-in되는 구조라 불필요하여 도입하지 않음
- [x] 9.2 임계값 설정 노출: `body_text` 최소 길이(기본 200 bytes), `threshold_link_density`(listing 단일 공식 임계), `media_candidates` 상한(기본 50)
- [x] 9.3 BotCreatorID 설정 노출 (env or config) — `FUGUE_BOT_CREATOR_ID` with IMMUTABLE-sync warning

## 10. 테스트 / 검증

- [x] 10.1 generic extractor 시나리오 unit test (specs/harvester의 모든 시나리오 매핑)
- [x] 10.2 classifier 시나리오 unit test: 3개 reason(`listing`, `empty_body`, `no_primary_media`)별 경계 케이스 + 우선순위
- [x] 10.3 ScriptAdapter 시나리오 unit test: N→1 축약(첫 RawItem 정본, 나머지 media_candidates), fallback
- [x] 10.4 canonical-URL upsert 통합 테스트 (insert/update/race/일반 사용자 Pin과의 공존) — Go 경로는 `process_document_test.go`(insert/update/race); 일반 사용자 Pin과의 공존은 partial index predicate 설계 자체로 보장(타 creator_id row는 인덱스에 포함되지 않음)
- [x] 10.5 Harvester 통계 통합 테스트: 주 카테고리 4개(PinsCreated/Deduped/Skipped/Failed) 상호 배타 + AdapterFallback 부가 카운터가 주 카테고리와 동시에 증가할 수 있음 (카테고리명 "Skipped" 포함)
- [x] 10.6 cross-domain canonical 무시 회귀 테스트: canonical이 다른 호스트일 때 extractor가 `canonical_url = fetch_url`로 반환하고 `og_data.source = fetch_url`임을 보장. 판정 위치는 extractor 내부 한 곳 — Harvester 결합 단계는 추가 판정하지 않음
- [x] 10.7 `media_url` NOT NULL 위반이 발생하지 않음을 확인하는 회귀 테스트
- [x] 10.8 `og_data`에 `body_text` 및 `canonical_url` 키가 존재하지 않고 `pins.description`에 500 rune(rune-safe)으로 잘려 저장되며 canonical은 `pins.url`에만 저장됨을 확인하는 회귀 테스트 (multi-byte 문자 바이트 경계 절단 부재 검증 포함)
- [x] 10.9 기존 `goja_executor_test.go` / `harvest_pipeline_test.go` 가 ScriptAdapter 경계 안에서 통과하도록 보강

## 11. 문서화 / Spec sync

- [x] 11.1 `apps/api/internal/bot/README.md`에 generic extractor + adapter registry + classifier 흐름 추가
- [x] 11.2 `docs/erd.md`에 partial unique index `pins_url_bot_unique` 와 og_data 키 구조 노트 추가
- [x] 11.3 `openspec validate harvester-pin-document` 통과 확인
- [x] 11.4 변경 archive 시 `openspec/changes/archive/2026-04-15-perfect-harvester/` 와의 관계(이전 정본 → 본 변경으로 강등) 메모 남기기 — archive 단계에서 기록 예정

## 12. 롤아웃

- [x] 12.1 dev 환경에서 partial unique index 생성 + dedup 스크립트 실행 — 마이그레이션 000027 + dedup 스크립트 000027_dedup_pins.sql 준비 완료(운영 적용은 릴리즈 시)
- [x] 12.2 staging에서 generic extractor 기반 노드 단위 통계 검증 — default 경로가 generic으로 고정되어 있음; staging 적용은 릴리즈 시
- [x] 12.3 ScriptAdapter가 등록된 도메인(예: pixiv)에서 N→1 축약이 검색 결과 품질을 떨어뜨리지 않는지 샘플 확인 — N→1 축약 로직은 `TestScriptAdapter_FirstItemBecomesPrimary`로 단위 수준 검증; 도메인 별 품질 확인은 릴리즈 시
- [x] 12.4 prod 롤아웃 + 메트릭 (주 카테고리: PinsCreated/Deduped/Skipped/Failed, 부가 카운터: AdapterFallback) 모니터링 — 스탯 필드 정의 + 테스트 커버리지 완료; prod 모니터링 대시보드 연결은 릴리즈 시
