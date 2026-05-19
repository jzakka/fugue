## ADDED Requirements

### Requirement: title은 pins.title 컬럼 cap에 맞춰 rune-safe 사전 절단된다

`PinDocument.Title`을 `pins.title` 컬럼에 저장하기 전에 시스템은 200 rune 이내로 잘라야 한다(SHALL). 절단은 UTF-8 rune 경계에서만 수행되며 멀티바이트 문자를 바이트 경계에서 절단해서는 안 된다(SHALL NOT). 200 rune 이하 입력은 무손실로 보존된다(SHALL).

이는 `pins.title VARCHAR(200) NOT NULL` 컬럼 cap에서 발생하는 `value too long for type character varying(200)` 거부를 사전 차단해 Pin upsert SHALL이 결정적으로 충족되도록 한다. `pins.description`의 500 rune cap 사전 절단과 동일한 패턴을 title에 대해서도 enforce한다.

#### Scenario: 201 rune title 입력 시 200 rune으로 잘려 저장된다
- **WHEN** Pioneer가 가져온 페이지의 `<title>` 또는 `<h1>` 또는 `og:title`이 201 rune 이상이고 Harvester가 그 PinDocument로 Pin을 upsert하려 할 때
- **THEN** 시스템은 `pins.title`에 정확히 200 rune까지만 저장하며 PostgreSQL의 `value too long for type character varying(200)` 에러를 발생시키지 않는다

#### Scenario: 멀티바이트 title은 rune 경계에서 잘린다
- **WHEN** title이 한국어/일본어/이모지 등 멀티바이트 문자열이고 201 rune 이상일 때
- **THEN** 시스템은 200번째 rune까지만 보존하며, 201번째 rune의 일부 바이트가 잘려 들어가 깨진 문자가 발생하지 않는다

#### Scenario: 200 rune 이하 title은 무손실 보존된다
- **WHEN** title이 빈 문자열 또는 200 rune 이하일 때
- **THEN** 시스템은 입력 그대로 `pins.title`에 저장하며 길이/내용을 변경하지 않는다

#### Scenario: classifier 입력은 절단되지 않은 원본 title을 받는다
- **WHEN** classifier가 PinDocument를 평가할 때
- **THEN** classifier는 잘리지 않은 원본 title로 판정을 수행하며, `pins.title`에 저장되는 값은 이와 무관하게 200 rune으로 잘린 형태다

이는 description(500 rune)에 대한 기존 Scenario "classifier는 원본 body_text를 받고 description은 잘린 형태"와 동일한 대칭 정책이다.
