## MODIFIED Requirements

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
- **THEN** 시스템은 `<link rel="canonical">`, `og:url`, fetch에 사용한 URL 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다. 단, 채택 후보의 호스트가 fetch URL의 호스트와 다르면 해당 후보는 건너뛰고 다음 fallback으로 넘어간다 (cross-domain canonical 무시)

#### Scenario: thumbnail_url 추출
- **WHEN** generic extractor가 썸네일 URL을 결정할 때
- **THEN** 시스템은 `og:image`, `twitter:image`, JSON-LD `schema.org` image 필드, 본문 내 첫 `<img>` 순으로 검사하여 처음 발견된 비어있지 않은 절대 URL을 사용한다

#### Scenario: media_candidates 수집
- **WHEN** generic extractor가 본문에서 미디어 후보를 수집할 때
- **THEN** 시스템은 본문 범위 내 `<img>`, `<video>`, `<audio>`, `<source>` 태그의 URL을 절대 경로로 변환하여 type(image/video/audio)과 함께 `media_candidates` 배열에 수집한다. 본문 범위는 `<article>` 태그가 있으면 그 내부, 없으면 `<body>` 전체로 정의된다

#### Scenario: source의 srcset 다중 후보에서 첫 URL만 수집
- **WHEN** `<source>` 태그가 `src` 속성 없이 `srcset` 속성만 가지며, 그 값이 `"a.webp 1x, b.webp 2x"`처럼 디스크립터를 포함한 콤마 구분 후보 목록일 때
- **THEN** 시스템은 첫 후보의 URL 토큰(콤마 분리 후 첫 후보의 공백 앞부분)만 추출하여 절대 경로로 변환해 `media_candidates`에 수집하고, 디스크립터(`1x`/`640w` 등)와 나머지 후보는 버린다. 추출된 URL 토큰이 비어 있으면 해당 `<source>`는 후보로 수집하지 않는다

#### Scenario: lang/author/published_at 추출
- **WHEN** generic extractor가 부가 메타를 추출할 때
- **THEN** 시스템은 `<html lang>` 또는 `og:locale`로부터 lang을, `schema.org` Author/`meta[name=author]`/`og:article:author`로부터 author를, `schema.org` datePublished/`article:published_time`/`<time datetime>`로부터 published_at을 추출한다 (없으면 빈 값/null)

#### Scenario: cross-domain canonical 무시
- **WHEN** HTML의 `<link rel="canonical">`이 fetch URL과 다른 도메인을 가리킬 때
- **THEN** 시스템은 해당 canonical을 무시하고 fetch URL을 canonical_url로 사용한다

#### Scenario: 사이트별 스크립트가 없어도 Pin이 생성된다
- **WHEN** 어떤 사이트에 대해 PerSiteAdapter가 등록되어 있지 않은 상태에서 일반 HTML 페이지를 harvest할 때
- **THEN** generic extractor가 동작하여 PinDocument가 생성되고, classifier가 통과시키면 Pin이 DB에 upsert된다
