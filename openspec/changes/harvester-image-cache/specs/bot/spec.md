## MODIFIED Requirements

### Requirement: Harvester는 Pin 생성 시 primary 이미지를 object storage에 캐시한다
시스템은 Harvester가 새 Pin을 생성할 때, 해당 페이지에서 추출한 primary 이미지 후보가 있으면 그 이미지를 다운로드하여 우리 object storage에 저장하고, 저장된 storage URL을 Pin의 `thumbnail_url` 및 `og_image` 컬럼에 기록해야 한다(SHALL). 이미지 캐싱의 성공/실패는 Pin 생성 자체의 성공/실패에 영향을 주지 않아야 한다(SHALL NOT block Pin creation).

본 capability에서 이미지 캐시 동작의 **외부 관찰 가능 행위**는 다음과 같다:
- **성공**: Pin의 `thumbnail_url`과 `og_image`에 우리 object storage URL이 기록된다.
- **실패**: 다운로드 실패, 업로드 실패, 크기 초과(Content-Length 혹은 read 누적) 중 어느 것이든 **구분 없이** 동일하게 처리되어, Pin의 `thumbnail_url`과 `og_image`에 **채택된 원본 후보 URL**이 그대로 기록된다.
- **후보 없음**: `thumbnail_url`과 `og_image`는 NULL로 기록된다.
- `thumbnail_url`과 `og_image`는 **항상 동일한 값**을 가진다(두 컬럼 모두 위 규칙에 따라 같은 문자열을 받거나 동시에 NULL이 된다).

#### Scenario: 이미지 캐시 성공 시 storage URL을 Pin에 기록
- **WHEN** Harvester가 페이지에서 primary 이미지 후보 URL을 찾고, 다운로드와 object storage 업로드가 모두 성공할 때
- **THEN** 시스템은 Pin의 `thumbnail_url`과 `og_image`에 **동일한** object storage URL을 기록한다

#### Scenario: 이미지 후보가 존재하지 않을 때
- **WHEN** Harvester가 추출 우선순위에 따른 모든 후보를 시도했지만 유효한 이미지 URL을 찾지 못할 때
- **THEN** 시스템은 `thumbnail_url`과 `og_image`를 모두 NULL로 두고 Pin은 정상 생성한다

#### Scenario: 이미지 캐시 실패해도 Pin 생성은 계속된다
- **WHEN** 이미지 후보는 찾았지만 다운로드·업로드·크기 초과 중 어느 하나로 캐시가 실패할 때
- **THEN** 시스템은 Pin 생성을 실패시키지 않고, Pin의 `thumbnail_url`과 `og_image`에 **동일한 원본 후보 URL**을 기록하며, 실패 사유는 로그로 관찰 가능하다

---

### Requirement: 이미지 추출은 og:image → twitter:image → article 내 주요 img → JSON-LD image 우선순위를 따른다
시스템은 Pin의 primary 이미지 후보를 추출할 때 다음 4단계를 위에서 아래로 시도하고, 첫 번째로 유효한 후보를 채택해야 한다(SHALL): (1) `<meta property="og:image">`, (2) `<meta name="twitter:image">` 또는 `<meta property="twitter:image">`, (3) `<article>` 또는 `<main>` 내부의 의미 있는 `<img>`, (4) `<script type="application/ld+json">` 안 schema.org 객체의 `image` 필드. "유효"는 (a) URL이 절대 URL로 resolve 가능, (b) http 또는 https 스킴, (c) `data:` URI가 아님, (d) 1×1 픽셀 추적용 의심 패턴(`pixel`, `1x1`, `spacer`)이 URL/파일명에 포함되지 않음을 모두 만족해야 한다(SHALL).

#### Scenario: og:image가 존재하면 1순위로 채택
- **WHEN** 페이지에 `<meta property="og:image" content="https://example.com/cover.jpg">` 가 있고, twitter:image와 article img도 함께 존재할 때
- **THEN** 시스템은 og:image의 URL을 채택한다

#### Scenario: og:image가 없으면 twitter:image 채택
- **WHEN** 페이지에 og:image는 없고 `<meta name="twitter:image" content="https://example.com/tw.jpg">` 가 있을 때
- **THEN** 시스템은 twitter:image의 URL을 채택한다

#### Scenario: og/twitter가 모두 없으면 article 내 주요 img 채택
- **WHEN** og:image와 twitter:image가 모두 없고, `<article>` 또는 `<main>` 내부에 width/height가 둘 다 100px 이상이거나 비어있지 않은 alt를 가진 `<img>`가 있을 때
- **THEN** 시스템은 해당 article/main 내부의 첫 번째 그러한 `<img>` 의 src를 채택한다

#### Scenario: 위 셋이 모두 없으면 JSON-LD image 채택
- **WHEN** og:image, twitter:image, article 내 유효 img가 모두 없고, `<script type="application/ld+json">` 안에 `"image": "https://example.com/ld.jpg"` 또는 `"image": ["https://example.com/ld.jpg", ...]` 또는 `"image": {"url": "https://example.com/ld.jpg"}` 가 있을 때
- **THEN** 시스템은 JSON-LD image의 첫 번째 URL을 채택한다

#### Scenario: 상대 URL은 페이지 URL 기준으로 절대화한다
- **WHEN** 채택된 후보가 `/static/cover.jpg` 같은 상대 경로일 때
- **THEN** 시스템은 페이지 URL을 base로 하여 절대 URL로 변환한 뒤 다운로드한다

#### Scenario: data: URI는 후보에서 제외된다
- **WHEN** og:image의 값이 `data:image/png;base64,...` 일 때
- **THEN** 시스템은 해당 후보를 건너뛰고 다음 우선순위 단계로 진행한다

#### Scenario: 1×1 추적 픽셀로 의심되는 후보는 제외된다
- **WHEN** 후보 URL이 `https://tracker.example.com/pixel.gif` 또는 `https://example.com/1x1.png` 처럼 추적 픽셀 패턴을 가질 때
- **THEN** 시스템은 해당 후보를 건너뛰고 다음 후보 또는 다음 우선순위 단계로 진행한다

---

### Requirement: 이미지 캐시는 `images/<sha256(url)>/<unix_ts>.<ext>` 키 스킴을 사용한다
시스템은 캐시할 이미지를 object storage에 저장할 때, 키를 `images/<hash>/<timestamp>.<ext>` 형식으로 구성해야 한다(SHALL). `<hash>`는 채택된 후보 URL을 다음 규칙으로 정규화한 문자열의 SHA-256 해시 hex(소문자, 64자)여야 한다:
1. 페이지 URL 기준 절대 URL로 resolve,
2. fragment(`#...`) 제거,
3. host lower-case,
4. 쿼리 파라미터는 보존(정렬/제거하지 않음 — CDN 변환 결과는 별개 객체로 취급).

`<timestamp>`는 캐시 시점의 unix epoch 초이고, `<ext>`는 응답 Content-Type을 다음 매핑으로 도출한다(SHALL): `image/jpeg` → `.jpg`, `image/png` → `.png`, `image/webp` → `.webp`, `image/gif` → `.gif`, 그 외/누락 → 원본 URL path의 확장자로 fallback, 그래도 없으면 `.bin`.

#### Scenario: 정규화된 URL의 SHA-256으로 디렉터리를 만든다
- **WHEN** 채택된 후보가 `https://Example.com/a.jpg?w=800#hero` 일 때
- **THEN** 시스템은 host를 lower-case로, fragment를 제거하고, 쿼리(`?w=800`)는 **그대로 보존**한 `https://example.com/a.jpg?w=800` 를 SHA-256 해싱한 hex 문자열을 디렉터리명으로 사용한다

#### Scenario: 응답 Content-Type으로 확장자를 결정한다
- **WHEN** 다운로드 응답의 Content-Type이 `image/webp` 일 때
- **THEN** 저장 키의 확장자는 `.webp` 가 된다

#### Scenario: 알 수 없는 Content-Type은 `.bin`으로 처리된다
- **WHEN** 다운로드 응답의 Content-Type이 매핑에 없고 원본 URL에도 확장자가 없을 때
- **THEN** 저장 키의 확장자는 `.bin` 이 된다

#### Scenario: prefix가 본문 미디어와 분리된다
- **WHEN** 시스템이 이미지 캐시 객체와 본문 미디어 객체를 모두 저장할 때
- **THEN** 이미지 캐시는 `images/` prefix, 본문 미디어는 `bot/` prefix를 사용한다(prefix 충돌 없음)

---

### Requirement: 이미지 캐시 객체의 TTL/만료는 본 capability 외부다
본 capability는 `images/` prefix 아래에 저장된 객체에 대해 만료 정책을 **정의하지 않는다**(SHALL NOT define lifecycle). 객체의 TTL, lifecycle rule, 고아 객체 정리(GC)는 **storage 운영자 및 후속 change의 책임**이다. 본 spec은 만료 여부에 의존하는 동작을 갖지 않는다.

#### Scenario: 본 spec은 만료 동작을 정의하지 않는다
- **WHEN** 운영자가 `images/` prefix에 lifecycle 정책을 설정하거나 설정하지 않을 때
- **THEN** 본 capability의 관찰 가능 동작(캐시 성공/실패 fallback/후보 없음 NULL)은 변하지 않는다

---

### Requirement: 이미지 캐시 실패는 단일 fallback 경로로 처리된다
시스템은 후보 URL은 찾았으나 (a) 다운로드 실패, (b) 업로드 실패, (c) 응답 Content-Length 혹은 read 누적이 구현 임계치를 초과 중 **어느 것이든** 발생하면, **구분 없이 동일하게** Pin의 `thumbnail_url`과 `og_image`에 채택된 원본 후보 URL을 그대로 기록해야 한다(SHALL). Pin 생성은 성공으로 처리되어야 하며, 부분적으로 다운로드된 바이트는 storage에 업로드되지 않아야 한다(SHALL NOT upload partial bytes). 실패 사유는 로그로 관찰 가능해야 한다(SHALL).

#### Scenario: 다운로드 실패 시 원본 URL fallback
- **WHEN** 후보 URL이 `https://example.com/cover.jpg` 인데 다운로드가 HTTP 403/404/타임아웃 등으로 실패할 때
- **THEN** 시스템은 Pin의 `thumbnail_url`과 `og_image`에 `https://example.com/cover.jpg` 를 **동일 값으로** 기록하고 Pin 생성을 성공시킨다

#### Scenario: 업로드 실패 시 원본 URL fallback
- **WHEN** 다운로드는 성공했으나 object storage 업로드가 실패할 때
- **THEN** 시스템은 Pin의 `thumbnail_url`과 `og_image`에 원본 후보 URL을 **동일 값으로** 기록하고 Pin 생성을 성공시킨다

#### Scenario: 다운로드 크기가 임계치를 초과하면 fallback 및 부분 데이터 버림
- **WHEN** 응답 Content-Length 또는 실제 다운로드 바이트가 시스템 임계치를 초과할 때
- **THEN** 시스템은 다운로드를 즉시 중단하고, 부분 데이터를 storage에 업로드하지 않으며, 원본 URL을 두 컬럼에 동일 값으로 기록한다

#### Scenario: 실패는 관찰 가능하다
- **WHEN** 이미지 캐시가 fallback 경로로 처리될 때
- **THEN** 시스템은 실패 사유(다운로드/업로드/크기초과)를 식별 가능한 로그로 기록한다
