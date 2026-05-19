# Design: HarvestPipeline title rune-safe truncate

## D1. 절단 위치 선택: consumer 단일 지점

후보:
- (A) `harvester_consumer.go:processOne`에서 `doc.BodyText` truncate 직후 1줄 추가
- (B) `harvest_pipeline.go:ProcessDocument` 진입 직후 truncate
- (C) GenericExtractor와 ScriptAdapter 각각에서 추출 시점에 cap

**선택: (A)**.

근거:
- `doc.BodyText`가 이미 (A) 위치에서 절단되고 있어 "pipeline write 직전 캡 강제" 패턴이 이미 합의된 상태. Title을 같은 지점에 두면 책임 위치가 한 함수에 집중되어 추적 가능.
- (B)는 동일 절단을 두 진입점(`createPins` → `ProcessDocument`)에 분산시키므로 합의된 패턴을 깨뜨림.
- (C)는 GenericExtractor 1곳 + ScriptAdapter N곳에 절단 책임을 분산. 새 adapter가 추가될 때마다 절단을 잊을 위험 + 절단 길이가 컬럼 cap의 동기화 변경에 취약. 단일 지점 원칙 위반.

## D2. 절단 함수: 기존 `truncateRunes` 재사용

후보:
- (A) 기존 `truncateRunes(s, n)` 그대로 사용
- (B) 새 함수 `truncateTitle(s)` (n=200 하드코딩)

**선택: (A)**.

근거:
- `truncateRunes`는 이미 rune-safe하고 multi-byte 안전. 동일 알고리즘이 description에 검증되어 있음.
- 새 함수 도입은 같은 동작을 다른 이름으로 두는 중복. 호출부에 `200` 리터럴을 명시하면 호출 지점만 봐도 어느 컬럼의 cap인지 즉시 식별 가능(description은 500, title은 200).

## D3. 컬럼 cap 상수화 여부

후보:
- (A) 호출 지점에 `200`, `500` 리터럴 그대로
- (B) `const titleMaxRunes = 200` / `const descriptionMaxRunes = 500`를 패키지 상수로

**선택: (A)**.

근거:
- 현재 코드도 `truncateRunes(doc.BodyText, 500)`로 리터럴을 그대로 쓰고 있어 일관성 유지.
- 상수 도입은 향후 컬럼 cap이 바뀔 때 한 곳에서 갱신할 수 있는 장점이 있으나, DB 마이그레이션과 동기화되어야 하므로 상수만 바꿔서는 안 됨. 마이그레이션 없이 상수만 바뀌는 회귀를 차단하려면 상수 도입의 가치가 제한적.
- spec 변경 시점에 상수 도입을 고민하는 것이 더 자연스러우므로 본 change에서는 미루고 리터럴 유지.

## D4. ScriptAdapter 경로 커버리지

`ScriptAdapter.Extract(...)` → `harvester_consumer.processOne` → `truncate(BodyText/Title)` → `ProcessDocument` 순서.

- consumer가 두 진입점(GenericExtractor와 ScriptAdapter) 모두의 결과를 받아서 가공하므로, consumer 한 지점의 truncate가 두 adapter를 모두 커버한다.
- ScriptAdapter 안에서 별도 truncate를 추가할 필요가 없다.

## D5. Spec ADDED Scenario 위치

`openspec/specs/harvester/spec.md`의 "PinDocument 부가 필드 og_data 저장 정책" 섹션은 L223-256. 그 안에 description rune-cap Scenario(L238-240)와 인접해 title rune-cap Scenario를 위치시킨다 → 두 컬럼의 cap 정책이 같은 섹션에 모이게 함.

## D6. truncateRunes 동작 검증

기존 함수는 multi-byte rune을 바이트 경계가 아닌 rune 경계로 자른다(L497-507 `range s`가 rune iteration). 한국어/일본어/이모지 모두 안전. 본 change는 함수를 손대지 않고 호출만 추가하므로 기존 동작이 description에서 이미 검증된 사실에 의존한다.

## D7. 실패 모드와 회귀 위험

- **회귀 위험 1**: 200 rune 이하의 정상 title은 `truncateRunes`가 그대로 반환(L497-499 early return). 변경 없음.
- **회귀 위험 2**: title이 빈 문자열일 때. `truncateRunes("", 200)` → utf8.RuneCountInString("")=0 ≤ 200 → 그대로 "" 반환. 변경 없음.
- **회귀 위험 3**: 정확히 200 rune title. utf8.RuneCountInString = 200 ≤ 200 → 그대로 반환. 변경 없음.
- **회귀 위험 4**: 201 rune 이상에서 truncate 결과가 정확히 200 rune인지. for-range가 0..200 → count==200일 때 i를 기록하고 `s[:i]` 반환. 200 rune 정확.

이상 4 케이스를 모두 단위 테스트로 회귀 방지한다.

## D8. 백워드 호환성

이미 DB에 들어 있는 title row는 모두 ≤200 자(VARCHAR(200) NOT NULL이 INSERT를 거부했을 것이므로). 본 변경은 신규 INSERT에만 영향. 마이그레이션 불필요. 기존 row 변환 불필요.
