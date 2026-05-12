## ADDED Requirements

### Requirement: 미디어 후보 유효성 검증

시스템은 추출 단계에서 수집된 미디어 후보(이미지/비디오/오디오)를 PinDocument의 `media_candidates` 또는 `thumbnail_url`로 채택하기 전에 외부 관찰 가능한 유효성 기준으로 검증해야 한다(SHALL). 검증을 통과하지 못한 후보는 PinDocument에 채택되지 않아야 한다(SHALL NOT). 유효성 기준은 (a) 선언된 타입의 미디어로 디코딩 가능할 것, (b) 미디어가 의미 있는 콘텐츠 크기를 가질 것이다. 구체 임계값과 측정 축은 운영 학습으로 조정 가능한 구현 파라미터이며 본 스펙의 행위 계약 일부가 아니다.

#### Scenario: 의미 있는 콘텐츠 크기 임계값을 만족하지 못하는 이미지는 후보에서 제외된다
- **WHEN** 추출된 이미지 후보가 디코딩 결과 의미 있는 콘텐츠 크기 임계값을 만족하지 못할 때 (QA-reported regression: 1x1 픽셀 placeholder GIF)
- **THEN** 해당 후보는 PinDocument의 `media_candidates`와 `thumbnail_url` 어디에도 포함되지 않는다

#### Scenario: 디코딩 불가능한 미디어는 후보에서 제외된다
- **WHEN** 미디어 후보 바이트열이 선언된 타입(image/video/audio)의 디코더 검증에 실패할 때
- **THEN** 해당 후보는 PinDocument의 `media_candidates`와 `thumbnail_url` 어디에도 포함되지 않는다

#### Scenario: 정상 미디어는 검증을 통과해 후보로 채택된다
- **WHEN** 미디어 후보가 디코딩 가능하고 의미 있는 콘텐츠 크기 임계값을 만족할 때
- **THEN** 해당 후보는 기존 추출 행위에 따라 PinDocument의 `media_candidates` 또는 `thumbnail_url`에 채택된다

#### Scenario: 모든 후보가 무효일 때는 빈 배열로 PinDocument가 구성된다
- **WHEN** 추출된 모든 미디어 후보가 검증에 탈락할 때
- **THEN** PinDocument의 `media_candidates`는 빈 배열, `thumbnail_url`은 빈 문자열로 구성된다

---

### Requirement: 정본 키 영속 제한

시스템은 미디어 후보 검증에서 탈락한 파일을 Pin이 참조 가능한 정본 키(Pin의 `media_url`이 가리키는 ObjectStorage 객체 키)로 ObjectStorage에 업로드하지 않아야 한다(SHALL NOT). Pin의 `media_url`이 가리키는 ObjectStorage 자원은 항상 유효성 검증을 통과한 미디어여야 한다(SHALL).

#### Scenario: 검증 탈락 후보는 정본 키에 업로드되지 않는다
- **WHEN** 미디어 후보가 유효성 검증에 실패할 때
- **THEN** 해당 미디어 바이트열은 Pin이 참조하는 정본 키로 ObjectStorage에 영속되지 않는다

#### Scenario: 정본 미디어 키는 항상 유효한 미디어를 가리킨다
- **WHEN** Pin의 `media_url`이 가리키는 ObjectStorage 자원을 조회할 때
- **THEN** 해당 자원은 본 스펙의 미디어 후보 유효성 기준을 만족하는 미디어 파일이다

---

### Requirement: 검증 실패 사유의 og_data 기록

시스템은 미디어 후보가 검증에 탈락한 사실과 사유를 PinDocument의 `og_data`에 관찰 가능한 형태로 기록해야 한다(SHALL). 이 기록은 디버깅 및 운영 메트릭 집계에 사용된다. 기록은 최소한 (a) 탈락한 후보 수, (b) 사유 분류(예: 디코딩 실패, 최소 크기 미달)별 카운트를 외부에서 조회 가능해야 한다. 구체 필드명/포맷은 구현 결정이며 본 스펙의 행위 계약 일부가 아니다.

#### Scenario: 모든 후보가 탈락한 경우 og_data에 사유가 보존된다
- **WHEN** 추출된 미디어 후보가 모두 유효성 검증에 탈락할 때
- **THEN** PinDocument의 `og_data`에서 탈락 후보 수와 사유 분류별 카운트가 관찰 가능하다

#### Scenario: 일부 후보가 탈락한 경우에도 사유가 보존된다
- **WHEN** 추출된 미디어 후보 중 일부만 검증에 탈락하고 나머지가 채택될 때
- **THEN** PinDocument의 `og_data`에서 탈락한 후보 수와 사유가 관찰 가능하며, 채택된 후보들은 정상 경로로 `media_candidates`/`thumbnail_url`에 들어간다

---

### Requirement: Pin primary media invariant

시스템은 본 변경 배포 이후 새로 생성되는 Pin이 가리키는 primary media(`media_url`이 가리키는 자원)가 본 스펙의 미디어 후보 유효성 기준을 만족함을 보장해야 한다(SHALL). 유효한 primary media를 확보할 수 없는 페이지에 대해서는 Pin을 생성하지 않아야 한다(SHALL NOT). 이는 기존 classifier의 `no_primary_media` 경로와 정합한다. 본 변경 배포 이전에 누적된 Pin은 본 invariant의 예외이며, 운영 backfill로 점진 정상화된다.

#### Scenario: 모든 미디어 후보가 무효한 페이지는 Pin을 만들지 않는다
- **WHEN** 페이지에서 추출된 모든 미디어 후보가 유효성 검증에 탈락할 때
- **THEN** Pin은 생성되지 않고 페이지는 `harvested_at`만 마킹되며 classifier reason은 `no_primary_media`로 기록된다

#### Scenario: 유효 미디어가 하나 이상 있으면 Pin이 생성된다
- **WHEN** 페이지에서 추출된 미디어 후보 중 하나 이상이 유효성 검증을 통과하고 다른 pinnability 조건도 만족될 때
- **THEN** Pin이 생성되며 `media_url`은 검증을 통과한 미디어 자원을 가리킨다
