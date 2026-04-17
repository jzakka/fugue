## ADDED Requirements

### Requirement: Harvester는 Pin 생성 시 primary 이미지를 object storage에 캐시한다
시스템은 Harvester가 새 Pin을 생성할 때, 해당 페이지에서 추출한 primary 이미지(썸네일/대표 이미지) 후보를 다운로드하여 우리 object storage에 저장하고, 저장된 storage URL을 Pin의 thumbnail/og_image 컬럼에 기록해야 한다(SHALL). 이미지 캐싱의 성공/실패는 Pin 생성 자체의 성공/실패에 영향을 주지 않아야 한다(SHALL NOT block).

#### Scenario: 이미지 캐시 성공 시 storage URL을 Pin에 기록
- **WHEN** Harvester가 페이지에서 primary 이미지 후보 URL을 찾고, 다운로드와 object storage 업로드가 모두 성공할 때
- **THEN** 시스템은 Pin의 thumbnail_url 및 og_image 컬럼에 우리 object storage URL을 기록한다

#### Scenario: 이미지 후보가 존재하지 않을 때
- **WHEN** Harvester가 추출 우선순위에 따른 모든 후보를 시도했지만 유효한 이미지 URL을 찾지 못할 때
- **THEN** 시스템은 thumbnail_url과 og_image를 NULL로 두고 Pin은 정상 생성한다

#### Scenario: 이미지 캐시 실패해도 Pin 생성은 계속된다
- **WHEN** 이미지 후보는 찾았지만 다운로드 또는 업로드가 실패할 때
- **THEN** 시스템은 Pin 생성을 실패시키지 않고, 실패는 로그/메트릭으로만 기록한다

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
시스템은 캐시할 이미지를 object storage에 저장할 때, 키를 `images/<hash>/<timestamp>.<ext>` 형식으로 구성해야 한다(SHALL). `<hash>`는 채택된 후보 URL을 절대화하고 fragment를 제거한 정규화된 URL의 SHA-256 해시 hex 문자열(소문자, 64자) 이어야 하며, `<timestamp>`는 캐시 시점의 unix epoch 초, `<ext>`는 응답의 Content-Type에서 도출(예: `image/jpeg` → `.jpg`)하고, 도출 실패 시 원본 URL의 확장자, 그래도 없으면 `.bin`을 사용해야 한다(SHALL).

#### Scenario: 정규화된 URL의 SHA-256으로 디렉터리를 만든다
- **WHEN** 채택된 후보가 `https://Example.com/a.jpg#hero` 일 때
- **THEN** 시스템은 host를 lower-case로, fragment를 제거한 `https://example.com/a.jpg` 를 SHA-256 해싱한 hex 문자열을 디렉터리명으로 사용한다

#### Scenario: 응답 Content-Type으로 확장자를 결정한다
- **WHEN** 다운로드 응답의 Content-Type이 `image/webp` 일 때
- **THEN** 저장 키의 확장자는 `.webp` 가 된다

#### Scenario: Content-Type이 없을 때 원본 URL의 확장자를 사용한다
- **WHEN** 응답에 Content-Type이 없고 원본 URL이 `https://example.com/cover.png?v=2` 일 때
- **THEN** 저장 키의 확장자는 `.png` 가 된다

#### Scenario: 동일 URL을 다시 캐시해도 충돌하지 않는다
- **WHEN** 동일한 후보 URL을 두 번 캐시할 때
- **THEN** 시스템은 같은 hash 디렉터리 안에 서로 다른 timestamp 파일명으로 저장한다(덮어쓰기 충돌 없음)

#### Scenario: prefix가 본문 미디어와 분리된다
- **WHEN** 시스템이 이미지 캐시 객체와 본문 미디어 객체를 모두 저장할 때
- **THEN** 이미지 캐시는 `images/` prefix, 본문 미디어는 `bot/` prefix를 사용한다(prefix 충돌 없음)

---

### Requirement: 이미지 캐시 객체는 만료 정책을 두지 않는다
시스템은 `images/` prefix 아래에 저장된 객체에 대해 storage 측 lifecycle 만료 정책을 적용하지 않아야 한다(SHALL NOT). 객체의 정리는 별도의 후속 GC 작업이 책임지며, 본 capability 범위에서는 객체가 무기한 보관되는 것으로 간주한다.

#### Scenario: 캐시된 이미지는 만료되지 않는다
- **WHEN** 이미지가 캐시된 후 임의의 시간이 지났을 때
- **THEN** 시스템은 해당 객체를 삭제하지 않으며, Pin의 thumbnail_url로 계속 접근 가능하다

#### Scenario: 만료 정책 변경은 본 capability 외부의 GC change에서 처리된다
- **WHEN** 운영자가 `images/` prefix에 lifecycle 정책을 추가하려 할 때
- **THEN** 본 capability는 그것을 금지하지 않지만, 정책 정의/추가는 별도 GC change의 책임이다(본 spec은 만료를 정의하지 않는다)

---

### Requirement: 이미지 캐시 실패 시 원본 URL을 fallback으로 보존한다
시스템은 후보 URL은 찾았으나 다운로드 또는 object storage 업로드가 실패할 때, Pin row의 thumbnail_url 및 og_image 컬럼에 **채택된 원본 후보 URL을 그대로** 저장해야 한다(SHALL). 이 경우 Pin 생성은 성공으로 처리되어야 하며, 실패는 로그/메트릭으로 관찰 가능해야 한다(SHALL).

#### Scenario: 다운로드 실패 시 원본 URL fallback
- **WHEN** 후보 URL이 `https://example.com/cover.jpg` 인데 다운로드가 HTTP 403/404/타임아웃 등으로 실패할 때
- **THEN** 시스템은 Pin의 thumbnail_url 및 og_image에 `https://example.com/cover.jpg` 를 그대로 저장하고 Pin 생성을 성공시킨다

#### Scenario: 업로드 실패 시 원본 URL fallback
- **WHEN** 다운로드는 성공했으나 object storage 업로드가 실패할 때
- **THEN** 시스템은 Pin의 thumbnail_url 및 og_image에 원본 후보 URL을 저장하고 Pin 생성을 성공시킨다

#### Scenario: 다운로드 크기가 임계치를 초과하면 fallback
- **WHEN** 응답 Content-Length 또는 실제 다운로드 바이트가 시스템 임계치(예: 20MB)를 초과할 때
- **THEN** 시스템은 다운로드를 중단하고 원본 URL을 fallback으로 저장한다

#### Scenario: 실패는 관찰 가능하다
- **WHEN** 이미지 캐시가 fallback 경로로 처리될 때
- **THEN** 시스템은 실패 사유(다운로드/업로드/크기초과)를 식별 가능한 로그 또는 메트릭으로 기록한다
