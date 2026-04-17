## Why

현재 Harvester(`apps/api/internal/bot/harvester.go`, `harvest_pipeline.go`, `goja_executor.go`)는 "사이트별 JavaScript 파싱 스크립트가 DB에 존재해야만" 콘텐츠를 추출할 수 있다(2026-04-15 `perfect-harvester` 변경 결과). 노드 타입에 매칭되는 스크립트가 없으면 노드는 그대로 스킵되고, RawItem(`title`, `mediaURL`, `mediaType`...)이 추출되지 못하므로 Pin도 만들어지지 않는다. 이는 두 가지 구조 문제를 만든다.

1. **Pin이 무엇인가에 대한 SSOT 부재.** Pin은 본래 검색·추천이 의존하는 문서(URL, 제목, 본문, 썸네일, og 메타, canonical 등)인데, 현재 코드에서는 "스크립트가 추출한 미디어 항목"으로만 정의되어 있다. 검색 인덱싱 관점에서 Pin이 "원본 페이지에 대한 단일 정본 문서"라는 계약이 어디에도 명시되지 않는다.
2. **HTML→Pin이 site-specific 스크립트에 종속.** 사실상 모든 일반 웹페이지는 `og:title`, `og:image`, `<title>`, `<article>`, JSON-LD 만으로도 Pin 문서로 변환 가능하다. 그런데 현재 경로는 사이트마다 사람/AI가 작성한 JS가 있어야만 동작하므로, 미등록 사이트는 그래프에 노드로만 남고 Pin은 0개다. 사용자 강조 사항: **JS 스크립트 실행은 default 경로가 아니라 per-site override여야 한다.**

`apps/api/fuguebot_pseudo.go`의 `Harvester.Run` 의도(`fetch → ParseDocument → Index`)와 현재 구현(스크립트 → RawItem → 미디어 다운로드 → Pin)의 격차를 좁히기 위해, Harvester의 primary contract를 **HTML을 Pin 문서로 변환**으로 재정의한다. JS 스크립트 경로는 새 `PerSiteAdapter` 추상 아래 한 종류의 어댑터로 강등한다.

## What Changes

- 새 capability spec `harvester` 도입. 다음 정본 경로를 정의한다:
  1. **Generic HTML→Pin extractor**: 어떤 페이지든 OpenGraph → Twitter Card → `<article>`/주요 본문 → JSON-LD(`schema.org/Article`, `schema.org/CreativeWork`) → HTML title 순으로 fallback 체인을 적용해 Pin 후보 문서를 만든다. 추출 필드: `title`, `body_text`(본문 텍스트), `canonical_url`, `thumbnail_url`, `media_candidates[]`(image/video/audio URL 후보), `lang`, `author`, `published_at`, `og_data`(원본 메타 전체 JSON).
  2. **Content classifier**: 추출 결과가 Pin이 될 자격이 있는지 판정하고, 부적합 시 사유를 `listing` / `empty_body` / `low_text_link_ratio` / `no_primary_media` 중 하나로 분류한다. 부적합 페이지는 Pin을 만들지 않고 frontier row의 `harvested_at`만 마킹한다(반복 fetch 방지).
  3. **Canonical-URL upsert**: Bot이 만드는 Pin은 canonical URL 기준으로 멱등하게 upsert된다. ERD의 "user pin URL 중복 허용" 정책은 유지하기 위해, 봇 계정이 만든 Pin에만 적용되는 partial unique index `pins(url) WHERE creator_id = BotCreatorID`를 추가한다.
  4. **PerSiteAdapter / AdapterRegistry**: `domain → Adapter` 레지스트리. 도메인에 매치되는 어댑터가 있으면 generic extractor 대신 어댑터 결과를 채택한다(또는 generic 결과를 보강한다). 기존 `GojaExecutor`는 `ScriptAdapter`라는 한 가지 구현으로 래핑되어 레지스트리에 등록된다(즉, JS 스크립트는 default가 아닌 per-site override다).
  5. **Frontier 역참조**: Pin이 어떤 frontier URL에서 유래했는지를 추적할 수 있도록 `og_data.source` 키에 원본 fetch URL을 보존한다(canonical_url과 다를 수 있다).

- `bot` capability spec MODIFIED: "JavaScript 파싱 스크립트를 실행하여 콘텐츠 항목을 추출한다" 및 그에 부속된 DOM 헬퍼·결과 변환 requirement를 "**per-site override 경로**"로 재정의한다. 기본(default) HTML→Pin 경로가 아님을 명시한다. 실행기 자체의 계약(타임아웃·구문/런타임 에러·필수 필드 검증)은 변경하지 않는다.

- **BREAKING (의미 수준)**: Bot이 생성한 Pin의 의미가 "스크립트가 추출한 미디어 한 건"에서 "원본 페이지에 대한 단일 정본 문서"로 바뀐다. 동일 페이지에 대해 봇은 하나의 Pin만 가진다(canonical 기준 upsert). 한 페이지에서 추출된 추가 미디어들은 Pin의 `og_data`/`media_candidates`로 보관되며, 이들을 별도 Pin으로 분리할지는 본 change 범위 외다.

## Capabilities

### New Capabilities
- `harvester`: HTML 콘텐츠를 Pin 문서(검색 SSOT)로 변환하는 정본 경로. Generic extractor + content classifier + canonical-URL upsert + PerSiteAdapter 레지스트리로 구성된다.

### Modified Capabilities
- `bot`: JavaScript 스크립트 실행 경로를 "default HTML→Pin 추출"이 아닌 "per-site override 어댑터"로 재정의한다. 기존 ScriptExecutor / DOM 헬퍼 / 결과 변환 requirement는 ScriptAdapter 경로의 계약으로 유지하되, "이 경로가 default가 아니다"를 명시한다.

## Impact

- **코드**:
  - `apps/api/internal/bot/` — 신규 `extractor.go`(generic HTML→Pin), `classifier.go`(listing/empty/... 사유 분류), `adapter.go`(PerSiteAdapter 인터페이스 + AdapterRegistry), `script_adapter.go`(기존 GojaExecutor 래핑).
  - `apps/api/internal/bot/harvester.go` — `executeNode`가 "스크립트 호출"이 아닌 "AdapterRegistry.Resolve(domain) || GenericExtractor"를 사용하도록 변경. 스크립트가 없는 도메인도 Pin이 생성된다.
  - `apps/api/internal/bot/harvest_pipeline.go` — RawItem 흐름은 ScriptAdapter 경로에만 남기고, generic 경로는 Pin upsert로 바로 진입.
- **DB**:
  - `pins` 테이블에 partial unique index `pins(url) WHERE creator_id = <BotCreatorID>` 추가. 일반 사용자 Pin의 URL 중복 허용 정책은 유지된다.
  - 신규 컬럼은 추가하지 않는다. 추출된 부가 메타(`canonical_url`, `lang`, `author`, `published_at`, `media_candidates`)는 기존 `pins.og_data` JSONB에 보관한다.
- **인터페이스**: 본 change는 `URLScheduler` consumer 루프(`harvester-scheduler-consumer`)가 호출하는 `ParseDocument`/`Index`의 내부 구현을 정의한다. scheduler 인터페이스 자체는 건드리지 않는다.
- **범위 외 (별도 change)**: 이미지/HTML 캐시(`harvester-image-cache`), snapshot first fetch(`harvester-snapshot-first-fetch`), 워커 예산/종료 조건(`harvester-worker-budget`), scheduler consumer 루프 자체(`harvester-scheduler-consumer`).
- **참조**:
  - `apps/api/fuguebot_pseudo.go`의 `Harvester.ParseDocument` / `Harvester.Index` 의도.
  - 직전 정본: `openspec/changes/archive/2026-04-15-perfect-harvester/` (스크립트 기반 경로의 마지막 스냅샷).
  - ERD: `docs/erd.md`의 `pins` 테이블 정의("URL 유니크 제약 없음" 정책 포함).
