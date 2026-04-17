## ADDED Requirements

### Requirement: Pin은 원본 페이지에 대한 단일 정본 문서다
Bot이 만드는 Pin은 검색 SSOT 역할을 하며, 한 원본 페이지(canonical URL 기준)에 대해 봇 계정 소유 Pin이 정확히 한 개만 존재해야 한다(SHALL). 같은 canonical URL을 다시 harvest하면 새 Pin을 만들지 않고 기존 봇 Pin을 update해야 한다(SHALL). 일반 사용자 Pin은 본 제약의 영향을 받지 않으며, 일반 사용자는 동일 URL로 복수 Pin을 보유할 수 있다(SHALL).

#### Scenario: 새 canonical URL에 대한 첫 harvest
- **WHEN** Harvester가 어떤 canonical URL에 해당하는 봇 Pin이 DB에 없는 상태에서 그 페이지를 harvest할 때
- **THEN** 시스템은 해당 canonical URL로 봇 계정 소유 Pin을 한 개 새로 생성한다

#### Scenario: 같은 canonical URL 재harvest
- **WHEN** Harvester가 이미 봇 Pin이 존재하는 canonical URL을 다시 harvest할 때
- **THEN** 시스템은 새 Pin을 만들지 않고 기존 봇 Pin의 메타(title, description, og_image, og_data 등)를 update한다

#### Scenario: 동일 URL의 일반 사용자 Pin은 영향 없음
- **WHEN** 어떤 일반 사용자가 이미 동일 URL로 Pin을 보유한 상태에서 봇이 그 URL을 harvest할 때
- **THEN** 봇 Pin이 추가로 생성되며 기존 사용자 Pin은 변경되지 않는다

#### Scenario: 일반 사용자의 동일 URL 복수 Pin은 허용
- **WHEN** 두 명의 서로 다른 일반 사용자가 같은 URL로 Pin을 생성할 때
- **THEN** 두 Pin이 모두 생성된다 (URL 중복 제약은 봇 Pin에만 적용)

#### Scenario: 봇 Pin 멱등성을 강제하는 partial unique index
- **WHEN** `pins` 테이블 스키마를 확인할 때
- **THEN** `WHERE creator_id = <BotCreatorID>` 조건의 partial unique index가 `pins(url)`에 존재한다

---

### Requirement: Generic HTML→Pin extractor가 default 변환 경로다
시스템은 어떤 HTML 페이지에 대해서도 site-specific 스크립트 없이 Pin 문서를 생성할 수 있어야 한다(SHALL). Default 변환은 OpenGraph → Twitter Card → JSON-LD(`schema.org`) → semantic HTML(`<article>`, `<title>`) 순의 fallback 체인으로 동작해야 한다(SHALL).

#### Scenario: title 추출 fallback 체인
- **WHEN** generic extractor가 HTML에서 title을 추출할 때
- **THEN** 시스템은 `og:title`, `twitter:title`, `<article>` 내 `<h1>`, `<title>` 태그 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다

#### Scenario: body_text 추출 fallback 체인
- **WHEN** generic extractor가 HTML에서 본문 텍스트를 추출할 때
- **THEN** 시스템은 `<article>` textContent, JSON-LD `schema.org/Article.articleBody`, 본문 내 최대 텍스트 밀도 블록, `og:description`, `meta[name=description]` 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다

#### Scenario: canonical_url 결정
- **WHEN** generic extractor가 canonical URL을 결정할 때
- **THEN** 시스템은 `<link rel="canonical">`, `og:url`, fetch에 사용한 URL 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다

#### Scenario: thumbnail_url 추출
- **WHEN** generic extractor가 썸네일 URL을 결정할 때
- **THEN** 시스템은 `og:image`, `twitter:image`, JSON-LD `schema.org` image 필드, 본문 내 첫 `<img>` 순으로 검사하여 처음 발견된 비어있지 않은 절대 URL을 사용한다

#### Scenario: media_candidates 수집
- **WHEN** generic extractor가 본문에서 미디어 후보를 수집할 때
- **THEN** 시스템은 본문 내 `<img>`, `<video>`, `<audio>`, `<source>` 태그의 URL을 절대 경로로 변환하여 type(image/video/audio)과 함께 `media_candidates` 배열에 수집한다

#### Scenario: lang/author/published_at 추출
- **WHEN** generic extractor가 부가 메타를 추출할 때
- **THEN** 시스템은 `<html lang>` 또는 `og:locale`로부터 lang을, `schema.org` Author/`meta[name=author]`/`og:article:author`로부터 author를, `schema.org` datePublished/`article:published_time`/`<time datetime>`로부터 published_at을 추출한다 (없으면 빈 값/null)

#### Scenario: cross-domain canonical 무시
- **WHEN** HTML의 `<link rel="canonical">`이 fetch URL과 다른 도메인을 가리킬 때
- **THEN** 시스템은 해당 canonical을 무시하고 fetch URL을 canonical_url로 사용한다

#### Scenario: 사이트별 스크립트가 없어도 Pin이 생성된다
- **WHEN** 어떤 사이트에 대해 PerSiteAdapter가 등록되어 있지 않은 상태에서 일반 HTML 페이지를 harvest할 때
- **THEN** generic extractor가 동작하여 PinDocument가 생성되고, classifier가 통과시키면 Pin이 DB에 upsert된다

---

### Requirement: PinDocument는 검색 SSOT 최소 필드를 가진다
Generic extractor 또는 PerSiteAdapter가 반환하는 `PinDocument`는 다음 최소 필드를 가져야 한다(SHALL): `title`, `body_text`, `canonical_url`, `thumbnail_url`, `media_candidates`, `lang`, `author`, `published_at`, `og_data`. 추출 실패 또는 부재 필드는 빈 값/빈 배열/null로 표현해야 하며, 객체 자체는 항상 반환되어야 한다(SHALL).

#### Scenario: 모든 필드가 추출된 경우
- **WHEN** generic extractor가 OG, JSON-LD, `<article>`을 모두 가진 페이지를 처리할 때
- **THEN** PinDocument의 모든 최소 필드가 비어있지 않은 값으로 채워진다

#### Scenario: 일부 필드만 추출된 경우
- **WHEN** generic extractor가 `og:title`만 있고 본문이 비어 있는 페이지를 처리할 때
- **THEN** PinDocument의 title은 채워지고 body_text는 빈 문자열, media_candidates는 빈 배열, published_at은 null로 반환된다

#### Scenario: PinDocument는 항상 반환된다
- **WHEN** generic extractor가 어떠한 HTML에 대해서도 호출될 때
- **THEN** 시스템은 nil이 아닌 PinDocument 객체를 반환한다 (필드는 비어 있을 수 있다)

---

### Requirement: Content classifier가 Pin 생성 가능 여부를 판정한다
시스템은 PinDocument 생성 후 Pin으로 indexing할지 여부를 판정해야 한다(SHALL). 부적합한 경우 Pin을 만들지 않고 사유를 다음 중 하나로 분류해야 한다(SHALL): `listing`, `empty_body`, `low_text_link_ratio`, `no_primary_media`. 사유는 우선순위(`listing` > `empty_body` > `no_primary_media` > `low_text_link_ratio`) 순으로 평가되며, 첫 매치에서 평가가 종료되어야 한다(SHALL).

#### Scenario: listing 페이지 분류
- **WHEN** 노드 타입이 `list`이거나, 같은 도메인 내 outgoing 링크 수가 본문 단어 수에 비해 압도적으로 많을 때
- **THEN** classifier는 `pinnable=false, reason=listing`을 반환한다

#### Scenario: empty_body 분류
- **WHEN** PinDocument의 body_text가 임계 길이(기본 200자) 미만일 때
- **THEN** classifier는 `pinnable=false, reason=empty_body`를 반환한다 (단, listing이 먼저 매치되면 listing이 우선)

#### Scenario: no_primary_media 분류
- **WHEN** PinDocument의 thumbnail_url이 비어 있고 media_candidates가 빈 배열이며 body_text도 임계 길이 미만일 때
- **THEN** classifier는 `pinnable=false, reason=no_primary_media`를 반환한다 (listing/empty_body가 먼저 매치되면 그쪽이 우선)

#### Scenario: low_text_link_ratio 분류
- **WHEN** body_text 길이를 outgoing 링크 수로 나눈 비율이 임계값 미만일 때
- **THEN** classifier는 `pinnable=false, reason=low_text_link_ratio`를 반환한다 (다른 사유가 매치되지 않은 경우에 한해)

#### Scenario: 정상 콘텐츠 페이지 통과
- **WHEN** PinDocument가 충분한 body_text를 가지고 thumbnail_url 또는 media_candidates가 존재하며 listing 패턴에 해당하지 않을 때
- **THEN** classifier는 `pinnable=true`를 반환한다

#### Scenario: 사유는 og_data에 보존된다
- **WHEN** classifier가 어떤 사유로든 판정을 내릴 때
- **THEN** 판정 결과(`{pinnable, reason?}`)는 PinDocument의 `og_data.classifier`에 저장되어 후속 디버깅/메트릭에 사용된다

---

### Requirement: Pinnable이 아닌 페이지는 Pin을 만들지 않고 harvested_at만 마킹한다
시스템은 classifier가 `pinnable=false`로 판정한 페이지에 대해 Pin을 생성하거나 update해서는 안 된다(SHALL NOT). 대신 frontier row의 `harvested_at`만 마킹해 동일 URL의 즉시 재harvest를 방지해야 한다(SHALL).

#### Scenario: listing 페이지 처리
- **WHEN** classifier가 어떤 페이지를 `listing`으로 분류할 때
- **THEN** Pin이 생성되지 않고 frontier row의 harvested_at만 현재 시각으로 갱신된다

#### Scenario: 빈 페이지 처리
- **WHEN** classifier가 어떤 페이지를 `empty_body`로 분류할 때
- **THEN** Pin이 생성되지 않고 frontier row의 harvested_at만 현재 시각으로 갱신된다

#### Scenario: 통계 분류
- **WHEN** Harvester가 노드 단위 통계를 집계할 때
- **THEN** pinnable=false로 분류된 노드는 PinsCreated/Deduped/Failed 어디에도 카운트되지 않고 별도의 Skipped(또는 Classified) 카운트로 분류된다

---

### Requirement: PerSiteAdapter / AdapterRegistry로 site-specific 변환을 등록한다
시스템은 도메인별로 generic extractor를 대체할 수 있는 어댑터를 등록할 수 있어야 한다(SHALL). 어댑터가 도메인에 매치되면 generic extractor 대신 어댑터의 결과를 채택해야 한다(SHALL). 어댑터 실행이 에러를 반환하면 generic extractor로 fallback해야 한다(SHALL).

#### Scenario: 어댑터 등록과 조회
- **WHEN** 어떤 도메인에 대한 PerSiteAdapter가 AdapterRegistry에 등록되어 있을 때
- **THEN** 같은 도메인 URL에 대해 `Resolve(domain)`이 해당 어댑터와 ok=true를 반환한다

#### Scenario: 어댑터 우선 적용
- **WHEN** Harvester가 어떤 URL을 처리할 때 그 도메인의 PerSiteAdapter가 등록되어 있을 때
- **THEN** Harvester는 generic extractor가 아닌 어댑터의 `Extract`를 호출하고 그 결과를 PinDocument로 사용한다

#### Scenario: 어댑터 실패 시 generic으로 fallback
- **WHEN** PerSiteAdapter의 `Extract`가 에러를 반환할 때
- **THEN** Harvester는 generic extractor로 fallback하여 PinDocument를 생성한다

#### Scenario: 어댑터 미등록 도메인은 generic 사용
- **WHEN** 어떤 URL의 도메인에 대해 PerSiteAdapter가 등록되어 있지 않을 때
- **THEN** Harvester는 generic extractor를 사용한다

#### Scenario: 어댑터 식별자가 og_data에 보존된다
- **WHEN** 어떤 PerSiteAdapter 또는 generic extractor가 PinDocument를 생성할 때
- **THEN** `og_data.extractor`에 사용된 추출기 식별자(`generic`, `script:<site_id>`, 또는 어댑터 이름)가 기록된다

---

### Requirement: 기존 JS 스크립트 경로는 ScriptAdapter로 PerSiteAdapter에 등록된다
시스템은 기존 `GojaExecutor` 기반 JS 스크립트 실행 경로를 `ScriptAdapter`로 래핑하여 AdapterRegistry에 등록해야 한다(SHALL). ScriptAdapter는 DB의 (site_id, node_type) 스크립트를 로드해 실행하고, 결과를 PinDocument 1건으로 축약해야 한다(SHALL).

#### Scenario: ScriptAdapter 등록
- **WHEN** Harvester 부트스트랩 시 DB에 (site_id, node_type) 스크립트가 등록된 사이트가 있을 때
- **THEN** 그 사이트의 도메인에 대한 ScriptAdapter가 AdapterRegistry에 등록된다

#### Scenario: ScriptAdapter는 RawItem N건을 PinDocument 1건으로 축약
- **WHEN** ScriptAdapter가 스크립트를 실행하여 N개의 RawItem을 받을 때
- **THEN** 첫 번째 RawItem이 PinDocument의 정본 메타(title, thumbnail_url, body_text 등)로 채택되고, 나머지 RawItem들은 type/url/width/height와 함께 `og_data.media_candidates`에 추가된다

#### Scenario: ScriptAdapter 결과의 extractor 식별자
- **WHEN** ScriptAdapter가 PinDocument를 반환할 때
- **THEN** `og_data.extractor`에 `script:<site_id>` 형태의 식별자가 기록된다

#### Scenario: ScriptAdapter 실행 실패 시 generic fallback
- **WHEN** ScriptAdapter 내부의 스크립트 실행이 타임아웃, 구문 에러, 런타임 에러 중 하나로 실패할 때
- **THEN** Harvester는 generic extractor로 fallback하여 PinDocument를 생성한다

---

### Requirement: Canonical-URL 기반 멱등 upsert
시스템은 PinDocument를 DB에 indexing할 때 canonical_url을 키로 봇 Pin을 멱등하게 upsert해야 한다(SHALL). upsert는 partial unique index `pins(url) WHERE creator_id = <BotCreatorID>`를 사용한 PostgreSQL `ON CONFLICT DO UPDATE`로 구현되어야 한다(SHALL).

#### Scenario: 신규 Pin 생성 (insert)
- **WHEN** PinDocument의 canonical_url에 해당하는 봇 Pin이 없을 때
- **THEN** 시스템은 새 Pin을 insert한다

#### Scenario: 기존 봇 Pin update
- **WHEN** PinDocument의 canonical_url에 해당하는 봇 Pin이 이미 있을 때
- **THEN** 시스템은 INSERT ... ON CONFLICT DO UPDATE를 통해 기존 Pin의 title, description, og_image, og_data를 새 값으로 갱신한다

#### Scenario: 동시 upsert race
- **WHEN** 두 워커가 같은 canonical_url에 대해 동시에 upsert를 시도할 때
- **THEN** PostgreSQL의 partial unique index가 race를 직렬화하여 정확히 한 개의 Pin만 존재한다 (한 쪽은 insert, 다른 쪽은 update로 귀결)

#### Scenario: 일반 사용자 Pin과의 충돌 없음
- **WHEN** 같은 canonical_url로 일반 사용자 Pin이 이미 존재할 때 봇이 그 URL을 upsert할 때
- **THEN** partial unique index는 `creator_id = <BotCreatorID>`만 적용되므로 충돌 없이 봇 Pin이 별도로 insert된다

---

### Requirement: 원본 fetch URL은 og_data.source에 보존된다
시스템은 PinDocument를 Pin으로 변환할 때, fetch에 사용한 원본 URL을 `og_data.source`에 보존해야 한다(SHALL). 이는 canonical_url과 다를 수 있으며, frontier row 역참조에 사용된다(SHALL).

#### Scenario: canonical_url과 fetch URL이 같을 때
- **WHEN** PinDocument의 canonical_url과 fetch URL이 동일할 때
- **THEN** `og_data.source`에 fetch URL이 저장된다 (canonical_url과 동일한 값)

#### Scenario: canonical_url과 fetch URL이 다를 때
- **WHEN** PinDocument의 canonical_url이 같은 도메인의 다른 URL을 가리킬 때 (예: query string 정규화)
- **THEN** `og_data.source`에는 원본 fetch URL이, `og_data.canonical_url`에는 canonical URL이 각각 저장된다

#### Scenario: frontier 역참조
- **WHEN** 운영자가 어떤 Pin이 어떤 frontier URL에서 유래했는지 추적할 때
- **THEN** `og_data.source` 필드를 통해 원본 frontier URL을 알 수 있다

---

### Requirement: 추출 부가 메타는 pins.og_data JSONB에 보관한다
시스템은 PinDocument의 부가 필드(canonical_url, lang, author, published_at, media_candidates, source, extractor, classifier)를 `pins` 테이블의 신규 컬럼이 아닌 기존 `og_data` JSONB 컬럼에 보관해야 한다(SHALL). `media_candidates`의 길이는 상한(기본 50)을 넘지 않도록 잘려야 한다(SHALL).

#### Scenario: og_data 키 구조
- **WHEN** Pin이 upsert될 때
- **THEN** `og_data`에는 최소 다음 키가 포함된다: `canonical_url`, `lang`, `author`, `published_at`, `media_candidates`, `source`, `extractor`, `classifier`

#### Scenario: media_candidates 상한 적용
- **WHEN** 추출된 media_candidates가 상한(기본 50)을 초과할 때
- **THEN** 앞에서부터 상한 개수까지만 보관되고 나머지는 잘린다

#### Scenario: 신규 컬럼 미추가
- **WHEN** 본 변경의 마이그레이션을 적용한 후 `pins` 테이블 스키마를 확인할 때
- **THEN** 부가 필드를 위한 새 컬럼이 추가되지 않았으며, partial unique index만 추가되어 있다

---

### Requirement: Harvester 노드 단위 통계 정의
시스템은 Harvester가 처리한 한 노드(URL)에 대해 다음 5개 통계 카테고리 중 정확히 하나로 집계해야 한다(SHALL): `PinsCreated`(신규 봇 Pin insert), `Deduped`(기존 봇 Pin update), `Skipped`(classifier가 pinnable=false 판정), `Failed`(extractor/upsert 에러), `AdapterFallback`(어댑터 실패로 generic으로 fallback). ScriptAdapter가 RawItem을 N개 반환하더라도 노드 1개당 통계 1개만 집계되어야 한다(SHALL).

#### Scenario: 신규 페이지 harvest
- **WHEN** Harvester가 새 canonical URL의 페이지를 처리하고 Pin을 새로 insert할 때
- **THEN** PinsCreated가 1 증가한다

#### Scenario: 기존 페이지 재harvest
- **WHEN** Harvester가 이미 봇 Pin이 있는 canonical URL을 다시 처리하고 Pin을 update할 때
- **THEN** Deduped가 1 증가한다

#### Scenario: pinnable=false 페이지 처리
- **WHEN** Harvester가 어떤 페이지를 처리하고 classifier가 pinnable=false로 판정할 때
- **THEN** Skipped가 1 증가한다 (PinsCreated/Deduped/Failed는 증가하지 않는다)

#### Scenario: 추출/upsert 에러
- **WHEN** Harvester가 어떤 페이지에 대해 extractor 또는 upsert에서 에러를 만날 때
- **THEN** Failed가 1 증가한다

#### Scenario: 어댑터 실패 후 generic 성공
- **WHEN** ScriptAdapter가 실패해 generic extractor로 fallback되어 Pin이 생성될 때
- **THEN** PinsCreated 또는 Deduped가 1 증가하고 별도로 AdapterFallback이 1 증가한다

#### Scenario: ScriptAdapter의 N개 RawItem
- **WHEN** ScriptAdapter가 한 노드에 대해 N개의 RawItem을 반환하여 PinDocument 1건으로 축약될 때
- **THEN** 노드 1개당 PinsCreated 또는 Deduped가 정확히 1만 증가한다 (N이 아니다)
