## MODIFIED Requirements

### Requirement: JavaScript 파싱 스크립트를 실행하여 콘텐츠 항목을 추출한다
스크립트 실행기는 DB에 저장된 JavaScript 스크립트를 실행하여 HTML에서 콘텐츠 항목 배열을 반환해야 한다(SHALL). 스크립트 실행 중 에러가 발생하면 빈 배열과 에러를 반환해야 한다(SHALL). **본 경로는 default HTML→Pin 변환 경로가 아니라 per-site override 경로다(SHALL).** Default 변환은 `harvester` capability의 generic HTML→Pin extractor가 담당하며, 스크립트 실행기는 `harvester` capability가 정의한 `PerSiteAdapter` 인터페이스의 한 가지 구현(`ScriptAdapter`)으로만 호출되어야 한다(SHALL). 어떤 사이트에 대해 ScriptAdapter가 등록되어 있지 않거나 실행이 실패하면 시스템은 generic extractor로 fallback해야 한다(SHALL).

#### Scenario: 정상적인 스크립트 실행
- **WHEN** 유효한 JavaScript 스크립트와 HTML이 주어질 때
- **THEN** 스크립트가 실행되어 추출된 콘텐츠 항목 배열이 반환된다

#### Scenario: 스크립트 구문 에러
- **WHEN** 문법 오류가 있는 JavaScript 스크립트가 주어질 때
- **THEN** 빈 배열과 구문 에러를 포함한 error가 반환된다

#### Scenario: 스크립트 런타임 에러
- **WHEN** 실행 중 예외가 발생하는 스크립트가 주어질 때
- **THEN** 빈 배열과 런타임 에러를 포함한 error가 반환된다

#### Scenario: 빈 HTML 입력
- **WHEN** 빈 문자열의 HTML이 주어질 때
- **THEN** 스크립트 실행이 정상 시도된다. 스크립트가 빈 배열을 반환하면 정상 처리되고, 예외를 throw하면 런타임 에러로 처리된다

#### Scenario: 스크립트 실행 타임아웃
- **WHEN** 스크립트가 지정된 타임아웃을 초과하여 실행될 때
- **THEN** 실행이 중단되고 빈 배열과 타임아웃 에러를 포함한 error가 반환된다

#### Scenario: 스크립트 경로는 default가 아니라 per-site override
- **WHEN** 어떤 사이트의 HTML이 default 변환 경로(generic HTML→Pin extractor)로도 처리 가능할 때
- **THEN** 시스템은 해당 사이트에 대해 `PerSiteAdapter`(예: ScriptAdapter)가 명시적으로 등록되어 있을 때만 스크립트 실행 경로를 사용하고, 그렇지 않으면 generic extractor를 사용한다

#### Scenario: ScriptAdapter 실패 시 generic extractor로 fallback
- **WHEN** ScriptAdapter로 래핑된 스크립트 실행기가 타임아웃, 구문 에러, 런타임 에러 등 어떤 사유로든 실패하여 빈 배열과 에러를 반환할 때
- **THEN** Harvester는 같은 HTML에 대해 generic HTML→Pin extractor로 fallback하여 PinDocument 생성을 시도한다

---

### Requirement: DOM 헬퍼 함수를 스크립트 런타임에 주입한다
실행 런타임은 스크립트가 HTML을 탐색할 수 있도록 DOM 유사 API를 제공해야 한다(SHALL). 최소한 querySelectorAll, querySelector, textContent, getAttribute 접근을 지원해야 한다(SHALL). **본 헬퍼는 `PerSiteAdapter`의 ScriptAdapter 구현 내부에서만 사용되며, default 변환 경로(generic HTML→Pin extractor)는 표준 Go HTML 파서(`golang.org/x/net/html` 등)를 사용해야 한다(SHALL).**

#### Scenario: querySelectorAll로 요소 목록 조회
- **WHEN** 스크립트가 CSS selector를 인자로 querySelectorAll을 호출할 때
- **THEN** 매칭되는 모든 요소의 배열이 반환된다

#### Scenario: querySelector로 단일 요소 조회
- **WHEN** 스크립트가 CSS selector를 인자로 querySelector를 호출할 때
- **THEN** 첫 번째 매칭 요소가 반환되거나, 없으면 null이 반환된다

#### Scenario: 요소의 textContent 접근
- **WHEN** 스크립트가 조회된 요소의 textContent를 읽을 때
- **THEN** 해당 요소의 텍스트 내용이 반환된다

#### Scenario: 요소의 getAttribute 접근
- **WHEN** 스크립트가 조회된 요소의 getAttribute를 호출할 때
- **THEN** 해당 속성 값이 반환되거나, 없으면 null이 반환된다

#### Scenario: DOM 헬퍼는 default 경로에서 사용되지 않는다
- **WHEN** Harvester가 generic HTML→Pin extractor를 통해 페이지를 처리할 때
- **THEN** 시스템은 JavaScript DOM 헬퍼 런타임을 초기화하지 않고 표준 Go HTML 파서를 사용한다

---

### Requirement: 스크립트 실행 결과를 콘텐츠 항목 배열로 변환한다
스크립트의 반환값을 콘텐츠 항목 배열로 변환해야 한다(SHALL). 필수 필드(title, mediaURL, mediaType)가 누락된 항목은 건너뛰어야 한다(SHALL). 선택 필드(description, sourceURL)가 빈 문자열이거나 누락된 경우에는 항목을 건너뛰지 않고 정상 처리해야 한다(SHALL). **본 변환 결과는 그대로 Pin 다건으로 indexing되지 않는다(SHALL NOT). ScriptAdapter는 N개의 콘텐츠 항목을 `harvester` capability가 정의한 PinDocument 1건으로 축약하여, 첫 번째 항목을 정본 메타로 채택하고 나머지 항목들은 `og_data.media_candidates`에 추가해야 한다(SHALL).**

#### Scenario: 정상적인 결과 변환
- **WHEN** 스크립트가 title, mediaURL, mediaType 필드를 포함한 객체 배열을 반환할 때
- **THEN** 각 객체가 콘텐츠 항목으로 변환되어 반환된다

#### Scenario: 필수 필드 누락 항목 스킵
- **WHEN** 스크립트 반환값 중 title, mediaURL, 또는 mediaType이 빈 문자열이거나 누락(undefined/null)인 항목이 있을 때
- **THEN** 해당 항목은 결과에서 제외된다

#### Scenario: sourceURL 누락 시 기본값 사용
- **WHEN** 추출된 항목에 sourceURL 필드가 없거나 빈 문자열일 때
- **THEN** 스크립트 실행 시 제공된 URL이 sourceURL로 사용된다

#### Scenario: 비배열 반환값 처리
- **WHEN** 스크립트가 배열이 아닌 값을 반환할 때 (undefined, null, 문자열, 숫자, 단일 객체 등)
- **THEN** 빈 배열과 에러가 반환된다

#### Scenario: N개 항목을 PinDocument 1건으로 축약
- **WHEN** ScriptAdapter가 한 페이지에 대해 N개의 콘텐츠 항목을 받을 때
- **THEN** 첫 번째 항목의 title/mediaURL/mediaType이 PinDocument의 정본 메타로 채택되고, 나머지 항목들은 type/url과 함께 `og_data.media_candidates`에 추가되어 노드 1개당 정확히 1건의 PinDocument가 반환된다

#### Scenario: 빈 결과 시 generic으로 fallback
- **WHEN** 스크립트 실행이 성공했으나 유효한 콘텐츠 항목 0건을 반환할 때
- **THEN** ScriptAdapter는 에러로 간주하여 generic HTML→Pin extractor로의 fallback이 일어나도록 한다
