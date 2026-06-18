## Context

`handleSource`(`apps/api/internal/bot/extractor.go:323-344`)는 본문 범위 내 `<source>` 태그를 미디어 후보로 수집한다. 현재 구현:

```go
func (s *extractScan) handleSource(n *html.Node) {
	src := getAttr(n, "src")
	if src == "" {
		src = getAttr(n, "srcset")   // 원문 그대로
	}
	if src == "" {
		return
	}
	mediaType := mediaTypeFromMIME(getAttr(n, "type"))
	if mediaType == "" {
		return
	}
	cand := MediaCandidate{Type: mediaType, URL: src}   // 깨진 URL 가능
	...
}
```

수집된 URL은 이후 `buildMediaCandidates`(L512+)가 `absolutize`(L494-510)로 절대화한다. `absolutize`의 `url.Parse`는 `"a.webp 1x, b.webp 2x"` 같은 공백/콤마 포함 문자열을 거부하지 않고 퍼센트 인코딩하여 `ok=true`로 통과시킨다(실측: `"https://example.com/article/a.webp%201x,%20b.webp%202x"`). 그래서 깨진 URL이 후보 배열까지 도달한다.

`srcset` 문법(HTML 표준): `image-candidate-string`들이 콤마로 구분되고, 각 후보는 `URL [공백 디스크립터]`. 첫 후보의 URL을 추출하려면 콤마로 분리 → 첫 토큰 → 앞뒤 공백 트림 → 공백으로 분리 → 첫 토큰이 URL.

## Goals / Non-Goals

**Goals:**
- `<source srcset>`에서 첫 후보의 URL만 추출하여 `media_candidates`에 유효한 절대 URL이 들어가도록 한다(spec "media_candidates 수집" SHALL 준수).
- 변경을 `handleSource` 단일 함수에 한정한다.

**Non-Goals:**
- `srcset`의 모든 후보(2x, 3x 등)를 각각 별도 미디어 후보로 수집하지 않는다. spec은 단일 절대 URL 수집을 규정하므로 첫 후보(통상 1x 기준 해상도)만 채택한다(보수적 선택).
- `<img srcset>`/`sizes` 처리(프론트엔드/디자인 트랙, 별개 영역)는 건드리지 않는다.
- `absolutize` 자체의 공백/콤마 거부 로직 강화는 범위 밖(다른 호출부 회귀 위험). 입력단에서 정제한다.
- `MediaValidator` wiring(별개 change `fix-harvester-wire-media-validator`)과 무관.

## Decisions

1. **srcset 첫 후보 URL 추출 헬퍼를 도입한다.** `handleSource`의 `src == ""` 분기에서 `getAttr(n, "srcset")` 결과를 그대로 쓰지 않고, 첫 콤마 구분 후보의 URL 토큰만 뽑는다.
   - 구현: `raw` → 첫 콤마 앞부분(`strings.SplitN(raw, ",", 2)[0]`) → `strings.Fields(...)[0]`(공백 정규화 + 첫 토큰). `Fields`는 leading/trailing/중복 공백을 모두 흡수하므로 `"  a.webp   1x"` 같은 변형도 안전.
   - 빈 결과(`Fields` 길이 0)면 빈 문자열 반환 → 기존 `if src == "" { return }` 가드가 후보를 버린다.
2. **`src` 우선순위 보존.** `src` 속성이 있으면 기존대로 그대로 사용(srcset 미참조). HTML 표준상 `<source>`는 picture 안에서 `srcset`을, video/audio 안에서 `src`를 쓰므로 이 우선순위가 자연스럽다.
3. **절대화는 기존 `buildMediaCandidates`/`absolutize` 경로 그대로.** 추출된 상대 URL(`a.webp`)은 기존 파이프라인이 base로 절대화한다. 새 경로를 만들지 않는다.

## Risks / Trade-offs

- **회귀 위험: 낮음.** 변경은 `srcset`-only `<source>` 경로에만 영향. `src` 있는 `<source>`, 디스크립터 없는 단일 URL srcset(`Fields[0]`이 곧 그 URL)은 동작 불변.
- **Trade-off: 첫 후보만 채택.** 고해상도(2x) 후보를 버리지만, spec이 단일 URL 수집을 규정하므로 계약 준수. 첫 후보는 통상 기준 해상도라 합리적 기본값.
- **롤백:** 단일 함수 변경이라 커밋 revert로 즉시 원복.
