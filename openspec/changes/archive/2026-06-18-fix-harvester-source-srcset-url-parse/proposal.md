## Why

`openspec/specs/harvester/spec.md`의 "media_candidates 수집" Scenario는 SHALL 계약으로, 본문 범위 내 `<source>` 태그의 URL을 **절대 경로로 변환하여** `media_candidates`에 수집하도록 규정한다. 그러나 `apps/api/internal/bot/extractor.go`의 `handleSource`(L323-344)는 `<source>`에 `src` 속성이 없을 때 `srcset` 속성 **원문**을 파싱 없이 그대로 `MediaCandidate.URL`로 사용한다.

`srcset`은 `"a.webp 1x, b.webp 2x"`처럼 URL과 디스크립터(`1x`, `640w` 등)가 공백으로 구분되고 후보들이 콤마로 나열된 문자열이라, 원문 전체는 단일 URL이 아니다. `absolutize`(L494-510)의 `url.Parse`는 공백/콤마를 거부하지 않고 퍼센트 인코딩하므로 `ok=true`로 통과하여, `"https://example.com/article/a.webp%201x,%20b.webp%202x"` 같은 **페치 불가능한 깨진 URL**이 `media_candidates`에 수집된다.

`handleSource`는 picture/video/audio 내 모든 `<source>`에 대해 호출되며(L234), `src` 없이 `srcset`만 가진 `<picture><source srcset=... type="image/...">` 마크업은 반응형 이미지의 표준 형태라 실제 크롤 대상에서 흔하다. 그 결과 SHALL("절대 경로로 변환")이 production에서 위반된다. 본 change는 `srcset` 분기에서 첫 후보의 URL 토큰만 추출하는 파싱 누락 한 곳만 닫는다.

## What Changes

- `handleSource`가 `<source>`의 `srcset` 속성을 사용할 때, 콤마로 구분된 후보 목록 중 **첫 후보의 URL 토큰**(공백 앞부분)만 추출하여 미디어 후보 URL로 삼는다. 디스크립터(`1x`/`640w` 등)는 버린다.
- 추출 결과가 비면 해당 `<source>`는 후보로 올리지 않는다(기존 빈 src 가드와 동일).
- `src` 속성이 있는 `<source>`, video/audio `<source>`, 디스크립터 없는 단일 URL srcset의 기존 동작은 변경하지 않는다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities
- `harvester`: "media_candidates 수집" Requirement에 `<source srcset>` 다중 후보 파싱 규칙을 명시하는 Scenario를 추가한다. 기존 SHALL("절대 경로로 변환") 본문은 변경하지 않고, srcset이 디스크립터 포함 후보 목록일 때 첫 후보 URL만 절대화해 수집한다는 행위를 명문화한다.

## Impact

- 영향 코드: `apps/api/internal/bot/extractor.go`의 `handleSource`(L323-344) 단일 함수. 시그니처·호출부 불변.
- 외부 계약: `media_candidates` 배열의 스키마/타입 불변. 값의 정합성(깨진 URL → 유효 절대 URL)만 개선.
- 운영 지표: `srcset`-only `<source>`에서 수집되던 깨진 URL이 유효 URL로 교정되어, 후속 MediaValidator 페치 단계에서 placeholder/무효로 폐기되던 후보 일부가 정상 후보로 살아남을 수 있음.
- 마이그레이션/롤백: DB 스키마 변경 없음. 코드 단일 함수 변경이라 롤백은 커밋 revert로 즉시 가능.
