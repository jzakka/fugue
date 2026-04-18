## MODIFIED Requirements

### Requirement: Harvester는 Pin 생성 시 primary 이미지를 object storage에 캐시한다
시스템은 Harvester가 새 Pin을 생성할 때, 해당 페이지에서 추출한 primary 이미지 후보가 있으면 그 이미지를 우리 object storage에 저장하고, 저장 결과에 해당하는 참조 값을 Pin의 **대표 이미지 참조 속성**에 기록해야 한다(SHALL). 이미지 캐싱의 성공/실패는 Pin 생성 자체의 성공/실패에 영향을 주지 않아야 한다(SHALL NOT block Pin creation).

본 requirement는 기존 `bot` capability의 "Harvester는 미디어 파일을 스토리지에 다운로드하여 저장한다" requirement와는 **별개의 데이터 흐름**이며 서로의 실패 정책에 영향을 주지 않는다. 본문 미디어(item의 media 본체) 다운로드 실패는 기존 정책에 따라 item skip을 야기할 수 있으나, primary 이미지 캐시 실패는 본 requirement의 fallback 경로로만 처리되고 Pin 생성을 막지 않는다. Pin의 "대표 이미지 참조 속성"이 저장 스키마의 어느 컬럼에 매핑되는지는 구현 관심사이며 design 문서에서 확정한다(본 change 기준으로는 단일 속성을 사용한다).

본 capability에서 이미지 캐시 동작의 **외부 관찰 가능 행위**는 다음과 같다:
- **성공**: Pin의 대표 이미지 참조 속성에 object storage 상의 참조가 기록된다.
- **실패**: 다운로드 실패, 업로드 실패, 크기 초과 중 어느 것이든 **구분 없이** 동일하게 처리되어, 채택된 원본 후보 URL이 대표 이미지 참조 속성의 값으로 기록된다.
- **후보 없음**: 대표 이미지 참조 속성은 비어 있는(기록되지 않은) 상태로 남는다.

#### Scenario: 이미지 캐시 성공 시 storage 참조를 Pin에 기록
- **WHEN** Harvester가 페이지에서 primary 이미지 후보 URL을 찾고, 다운로드와 object storage 업로드가 모두 성공할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성에 object storage 참조를 기록한다

#### Scenario: 이미지 후보가 존재하지 않을 때
- **WHEN** Harvester가 추출 우선순위에 따른 모든 후보를 시도했지만 유효한 이미지 URL을 찾지 못할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성을 비워 두고 Pin은 정상 생성한다

#### Scenario: 이미지 캐시 실패해도 Pin 생성은 계속된다
- **WHEN** 이미지 후보는 찾았지만 다운로드·업로드·크기 초과 중 어느 하나로 캐시가 실패할 때
- **THEN** 시스템은 Pin 생성을 실패시키지 않고, Pin의 대표 이미지 참조 속성에 원본 후보 URL을 기록하며, 실패 사유는 로그로 관찰 가능하다

---

### Requirement: 이미지 추출은 og:image → twitter:image → article 내 의미 있는 img → JSON-LD image 우선순위를 따른다
시스템은 Pin의 primary 이미지 후보를 추출할 때 다음 4단계를 위에서 아래로 시도하고, 첫 번째로 유효한 후보를 채택해야 한다(SHALL): (1) `<meta property="og:image">`, (2) `<meta name="twitter:image">` 또는 `<meta property="twitter:image">`, (3) `<article>` 또는 `<main>` 내부의 의미 있는 `<img>`, (4) `<script type="application/ld+json">` 안 schema.org 객체의 `image` 필드. "유효"는 (a) URL이 절대 URL로 resolve 가능, (b) http 또는 https 스킴, (c) `data:` URI가 아님, (d) 명백한 추적 픽셀(1×1 이미지 등)로 의심되지 않음을 모두 만족해야 한다(SHALL).

동일 우선순위 단계에서 여러 후보가 발견될 때(예: 여러 `<script type="application/ld+json">` 블록, 또는 JSON-LD `image` 필드가 배열/객체 형태), 시스템은 **문서 내 등장 순서(DOM 순서)** 기준 첫 번째 유효 후보를 채택해야 한다(SHALL).

#### Scenario: og:image가 존재하면 1순위로 채택
- **WHEN** 페이지에 `<meta property="og:image" content="https://example.com/cover.jpg">` 가 있고, twitter:image와 article img도 함께 존재할 때
- **THEN** 시스템은 og:image의 URL을 채택한다

#### Scenario: og:image가 없으면 twitter:image 채택
- **WHEN** 페이지에 og:image는 없고 `<meta name="twitter:image" content="https://example.com/tw.jpg">` 가 있을 때
- **THEN** 시스템은 twitter:image의 URL을 채택한다

"article 내 의미 있는 `<img>`"는 다음 기준 중 **어느 하나라도** 만족하는 `<article>`/`<main>` 내부 `<img>` 요소를 의미한다(SHALL): (i) `width` 속성과 `height` 속성이 **둘 다** 100 이상, 또는 (ii) `alt` 속성이 비어 있지 않음. 위 기준을 만족하지 않는 `<img>`(예: width/height 미지정이거나 하나만 지정된 작은 img, alt가 공백)는 본 우선순위 단계에서 후보가 아니다(SHALL NOT).

#### Scenario: og/twitter가 모두 없으면 article 내 의미 있는 img 채택
- **WHEN** og:image와 twitter:image가 모두 없고, `<article>` 또는 `<main>` 내부에 `width="600" height="400"` 처럼 둘 다 100 이상을 만족하거나 `alt="상품 사진"` 처럼 비어있지 않은 alt를 갖는 `<img>`가 있을 때
- **THEN** 시스템은 해당 article/main 내부의 DOM 순서상 첫 번째 그러한 `<img>` 의 src를 채택한다

#### Scenario: 크기 기준·alt 기준 어느 쪽도 충족 못하는 img는 article 후보가 아니다
- **WHEN** `<article>` 내부에 `<img src="icon.png">` 처럼 width/height 속성이 없고 alt도 비어 있는 `<img>`만 있을 때
- **THEN** 시스템은 해당 img를 후보에서 제외하고 다음 우선순위 단계(JSON-LD)로 진행한다

#### Scenario: 위 셋이 모두 없으면 JSON-LD image 채택
- **WHEN** og:image, twitter:image, article 내 유효 img가 모두 없고, `<script type="application/ld+json">` 안에 `"image": "https://example.com/ld.jpg"` 또는 `"image": ["https://example.com/ld.jpg", ...]` 또는 `"image": {"url": "https://example.com/ld.jpg"}` 가 있을 때
- **THEN** 시스템은 DOM 순서상 첫 번째 JSON-LD 블록의 첫 번째 유효 image URL을 채택한다(배열이면 첫 요소, 객체면 `url` 필드)

#### Scenario: 상대 URL 후보는 절대 URL로 해석되어 채택된다
- **WHEN** 채택된 후보 속성 값이 `/static/cover.jpg` 같은 상대 경로일 때
- **THEN** 시스템은 페이지 URL을 base로 하여 절대 URL로 해석한 값을 후보로 사용한다

#### Scenario: data: URI는 후보에서 제외된다
- **WHEN** og:image의 값이 `data:image/png;base64,...` 일 때
- **THEN** 시스템은 해당 후보를 건너뛰고 다음 우선순위 단계로 진행한다

#### Scenario: 추적 픽셀로 의심되는 후보는 제외된다
- **WHEN** 후보 URL이 1×1 추적 픽셀로 의심되는 특성(예: 파일명이 추적 픽셀 관례적 패턴을 포함)을 가질 때
- **THEN** 시스템은 해당 후보를 건너뛰고 다음 후보 또는 다음 우선순위 단계로 진행한다

---

### Requirement: 이미지 캐시 객체는 후보 URL에서 파생된 안정적이고 충돌 회피된 키로 저장된다
시스템은 캐시할 이미지를 object storage에 저장할 때, 후보 URL과 저장 시점에서 결정적으로 파생되는 키로 저장해야 한다(SHALL). 키 구성은 다음 외부 관찰 가능 조건을 만족해야 한다:
- 서로 다른 후보 URL은 서로 다른 키로 저장된다(SHALL).
- 같은 후보 URL을 서로 다른 시점에 캐시하면 서로 다른 키로 저장되어, 이전 객체가 덮어써지지 않는다(SHALL NOT overwrite).
- 이미지 캐시 저장 네임스페이스는 본문 미디어(item의 media 본체) 저장 네임스페이스와 **분리**되어, 두 경로의 모니터링/lifecycle 정책을 독립적으로 운용할 수 있다(SHALL).
- 저장 키는 응답 Content-Type에서 파생된 확장자를 포함해야 하며(SHALL), Content-Type이 없거나 매핑되지 않을 때의 fallback 또한 결정적이어야 한다(SHALL).

구체 해시 알고리즘, 타임스탬프 해상도, Content-Type ↔ 확장자 매핑 테이블, 네임스페이스 이름 같은 **키 구성의 내부 알고리즘**은 design 문서에서 확정한다.

단, 스킴과 호스트의 **대소문자 차이만**(RFC 3986상 case-insensitive한 컴포넌트)은 서로 다른 후보로 취급하지 않는다 — 예: `HTTP://Example.com/x` 와 `http://example.com/x` 는 동일 후보로 간주되어 동일 키 공간에 저장된다.

#### Scenario: 서로 다른 후보 URL은 서로 다른 키로 저장된다
- **WHEN** 두 후보 URL이 정규형(스킴·호스트는 대소문자 동일시, path·query는 문자 그대로) 기준으로 스킴/호스트/경로/쿼리 중 어느 하나라도 다를 때
- **THEN** 두 객체의 저장 키는 서로 다르다

#### Scenario: 스킴/호스트 대소문자 차이만 있는 두 후보는 동일 후보로 취급된다
- **WHEN** 두 후보 URL이 `HTTPS://Example.com/a.jpg` 와 `https://example.com/a.jpg` 처럼 스킴과 호스트의 대소문자만 다르고 path·query가 동일할 때
- **THEN** 두 후보는 동일 후보로 간주되어 저장 키 파생의 관점에서 동일한 키 공간에 매핑된다

#### Scenario: 같은 후보 URL을 다른 시점에 재캐시하면 별도 객체로 저장된다
- **WHEN** 동일 후보 URL을 서로 다른 시점에 두 번 캐시할 때
- **THEN** 두 번째 업로드는 첫 번째 객체를 덮어쓰지 않고 별도 키로 저장된다

#### Scenario: 응답 Content-Type에서 확장자가 파생된다
- **WHEN** 다운로드 응답의 Content-Type이 이미지 타입을 명시할 때
- **THEN** 저장 키에는 해당 Content-Type에 대응되는 확장자가 포함된다

#### Scenario: Content-Type 확장자 파생이 실패하면 결정적 fallback 확장자가 사용된다
- **WHEN** 다운로드 응답의 Content-Type이 알려진 이미지 타입 매핑에 없을 때
- **THEN** 저장 키는 원본 URL의 확장자 또는 사전에 정의된 기본 확장자를 결정적 규칙으로 사용한다

#### Scenario: 이미지 캐시 네임스페이스가 본문 미디어 네임스페이스와 분리된다
- **WHEN** 시스템이 이미지 캐시 객체와 본문 미디어 객체를 모두 저장할 때
- **THEN** 두 저장 위치는 분리된 네임스페이스를 가져 서로 prefix 충돌이나 lifecycle 교차가 없다

---

### Requirement: 이미지 캐시 객체의 TTL/만료는 본 capability 외부다
본 capability는 이미지 캐시 저장 네임스페이스에 저장된 객체에 대해 만료 정책을 **정의하지 않는다**(SHALL NOT define lifecycle). 객체의 TTL, lifecycle rule, 고아 객체 정리(GC)는 **storage 운영자 및 후속 change의 책임**이다. 본 spec은 만료 여부에 의존하는 동작을 갖지 않는다.

#### Scenario: 본 spec은 만료 동작을 정의하지 않는다
- **WHEN** 운영자가 이미지 캐시 네임스페이스에 lifecycle 정책을 설정하거나 설정하지 않을 때
- **THEN** 본 capability의 관찰 가능 동작(캐시 성공/실패 fallback/후보 없음 공란)은 변하지 않는다

---

### Requirement: 이미지 캐시 실패는 단일 fallback 경로로 처리된다
시스템은 후보 URL은 찾았으나 (a) 다운로드 실패, (b) 업로드 실패, (c) 응답 Content-Length 혹은 read 누적이 구현 임계치를 초과 중 **어느 것이든** 발생하면, **구분 없이 동일하게** Pin의 대표 이미지 참조 속성에 채택된 원본 후보 URL을 그대로 기록해야 한다(SHALL). Pin 생성은 성공으로 처리되어야 하며, 부분적으로 다운로드된 바이트는 object storage에 업로드되지 않아야 한다(SHALL NOT upload partial bytes). 업로드 도중 실패로 인해 부분 객체(예: 중단된 멀티파트 업로드 또는 partial commit)가 남을 수 있는 경우, **해당 객체의 정리 책임은 본 capability 외부**(storage lifecycle 또는 후속 GC change)에 위임된다. 실패 사유는 로그로 관찰 가능해야 한다(SHALL).

#### Scenario: 다운로드 실패 시 원본 URL fallback
- **WHEN** 후보 URL이 다운로드 단계에서 HTTP 403/404/타임아웃 등으로 실패할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성에 채택된 원본 후보 URL을 기록하고 Pin 생성을 성공시킨다

#### Scenario: 업로드 실패 시 원본 URL fallback
- **WHEN** 다운로드는 성공했으나 object storage 업로드가 실패할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성에 원본 후보 URL을 기록하고 Pin 생성을 성공시킨다

#### Scenario: 다운로드 크기가 임계치를 초과하면 fallback 및 부분 데이터 버림
- **WHEN** 응답 Content-Length 또는 실제 다운로드 바이트가 시스템 임계치를 초과할 때
- **THEN** 시스템은 다운로드를 즉시 중단하고, 부분 데이터를 storage에 업로드하지 않으며, 원본 URL을 대표 이미지 참조 속성에 기록한다

#### Scenario: 실패는 관찰 가능하다
- **WHEN** 이미지 캐시가 fallback 경로로 처리될 때
- **THEN** 시스템은 실패 사유(다운로드/업로드/크기초과)를 식별 가능한 로그로 기록한다
