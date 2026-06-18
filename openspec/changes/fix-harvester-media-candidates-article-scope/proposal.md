## Why

`openspec/specs/harvester/spec.md`의 "media_candidates 수집" Scenario는 SHALL 계약으로, 미디어 후보의 **본문 범위**를 "`<article>` 태그가 있으면 그 내부, 없으면 `<body>` 전체"로 정의한다. 즉 `<article>`이 존재하면 그 밖(헤더/네비/사이드바/푸터/관련글)의 미디어는 `media_candidates`에서 제외되어야 한다.

그러나 `apps/api/internal/bot/extractor.go`의 `walk`(L153-264)는 `<img>`(L197-216)·`<video>`(L217-225)·`<audio>`(L226-232)·`<source>`(L233/`handleSource` L323-344) 태그를 `inArticle` 값과 무관하게 **문서 전체(whole-tree)**에서 수집한다. `inArticle` 플래그는 `firstArticleImage`(L212)와 텍스트 버퍼(L246-252)에만 영향을 줄 뿐, 미디어 수집 슬라이스(`mediaImages`/`mediaVideos`/`mediaAudios`)에는 적용되지 않는다. `buildMediaCandidates`(L526-553)도 절대화·중복제거·50 cap만 수행하고 article 범위 필터가 없다.

그 결과 `<article>`을 가진 페이지에서 article 밖 미디어가 `media_candidates`에 섞여 SHALL이 위반된다. 두 가지 외부 관찰 가능한 결함이 발생한다:

1. `og_data.media_candidates`(MediaValidator가 sync)에 article 밖 미디어가 항상 오염되어 저장된다.
2. `pickMediaForPin`(`harvest_pipeline.go:316-337`)은 `ThumbnailURL`이 비면 첫 `MediaCandidate` URL을 Pin 대표 미디어로 채택한다. 동일 추출기의 썸네일 fallback(`firstArticleImage`)은 이미 article-scoped이므로, **article 안에 적격 미디어가 없고 밖에만 있는 페이지**에서 스펙대로면 `media_candidates`가 공집합 → classifier `no_primary_media` → skip 되어야 하나, 현 구현은 article 밖 미디어를 대표로 채택해 Pin을 생성한다. classifier 판정과 대표 미디어가 모두 스펙과 어긋난다.

같은 추출기 안에서 `thumbnail`/`firstArticleImage`/`articleText`는 article-scoped인데 `media_candidates`만 whole-tree라 내부 정의가 불일치한다. 본 change는 미디어 수집 범위를 spec의 본문 범위 정의에 맞추는 한 곳만 닫는다.

## What Changes

- `walk`이 미디어(`<img>`/`<video>`/`<audio>`/`<source>`)를 수집할 때 `inArticle` 값으로 **article 범위 버킷**과 **body 범위 버킷**에 나눠 담는다. 텍스트 추출이 이미 쓰는 `articleTextBuf`/`bodyTextBuf` dual-buffer 패턴을 미디어에도 동일하게 적용한다.
- `buildMediaCandidates`가 `<article>` 존재(`sawArticle`) 시 article 버킷을, 없으면 body 버킷을 사용한다. 절대화·중복제거·50 cap·type별 grouping의 기존 동작은 변경하지 않는다.
- `<article>`이 없는 페이지(body 전체 수집), `<article>` 내부 미디어, 50 cap, srcset 첫 URL, video/audio type 수집의 기존 동작은 변경하지 않는다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities
- `harvester`: "media_candidates 수집" Requirement에 본문 범위(article-scope) 배제 규칙을 명시하는 Scenario를 추가한다. 기존 SHALL 본문(`<article>` 있으면 그 내부, 없으면 `<body>` 전체)은 변경하지 않고, `<article>`이 존재할 때 그 밖의 미디어가 제외되는 경계 행위를 명문화한다.

## Impact

- 영향 코드: `apps/api/internal/bot/extractor.go`의 `extractScan` 구조체(미디어 버킷)·`walk`·`handleSource`·`buildMediaCandidates`. 공개 시그니처(`Extract`)·반환 타입 불변.
- 외부 계약: `media_candidates` 배열의 스키마/타입 불변. 값의 정합성(article 밖 미디어 제외)만 개선.
- 운영 지표: `<article>`을 가진 페이지에서 수집되던 article 밖 미디어가 제거된다. article 내부에 적격 미디어가 없던 페이지 일부가 `no_primary_media`로 skip되어 Pin 신규 생성률이 일부 하락 가능(=spec의 의도된 행위). `og_data.media_candidates` 품질 개선.
- 마이그레이션/롤백: DB 스키마 변경 없음. extractor.go 내부 변경이라 롤백은 커밋 revert로 즉시 가능.
