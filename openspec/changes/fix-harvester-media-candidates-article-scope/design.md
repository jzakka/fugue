## Context

`GenericExtractor.Extract`(`apps/api/internal/bot/extractor.go:40-105`)는 `extractScan`을 DOM 위로 단일 패스 순회(`walk`, L153-264)시켜 추출값을 모은 뒤 필드를 해소한다. 미디어 후보는 `walk`이 모은 `mediaImages`/`mediaVideos`/`mediaAudios` 슬라이스를 `buildMediaCandidates`(L526-553)가 절대화·중복제거·50 cap을 거쳐 만든다.

현재 미디어 수집은 `inArticle`을 무시한다:

```go
// walk(): case "img"
if (w >= 100 && h >= 100) || alt != "" {
    s.mediaImages = append(s.mediaImages, MediaCandidate{...}) // inArticle 무관
    if inArticle && s.firstArticleImage == "" {
        s.firstArticleImage = src // 썸네일만 article-scoped
    }
}
```

`<article>` 분기(L188-196)는 자식을 `inArticle=true`로 순회하고 `s.sawArticle=true`를 세팅한 뒤 return한다. article 밖의 미디어는 일반 순회(L255-257, `inArticle=false`)에서 그대로 슬라이스에 들어간다.

반면 텍스트와 썸네일은 이미 본문 범위를 구분한다:
- `articleTextBuf`는 `inArticle`일 때만, `bodyTextBuf`는 `!inArticle`일 때만 기록(L246-252). `Extract`는 `scan.articleText`(있으면) → `scan.bodyDensityText` 순으로 본문을 고른다(L74-81).
- `firstArticleImage`는 `inArticle`일 때만 세팅(L212).

spec("media_candidates 수집")은 본문 범위를 "`<article>` 있으면 그 내부, 없으면 `<body>` 전체"로 정의하므로, 미디어도 텍스트·썸네일과 동일한 본문 범위를 따라야 한다.

## Goals / Non-Goals

**Goals:**
- 미디어 수집 범위를 spec의 본문 범위 정의에 일치시킨다: `<article>` 있으면 그 내부 미디어만, 없으면 `<body>` 전체.
- 텍스트 추출이 이미 검증한 dual-buffer 패턴을 미디어에 동형으로 적용해 일관성을 유지한다.
- 변경을 `extractor.go` 내부(구조체·`walk`·`handleSource`·`buildMediaCandidates`)에 한정한다.

**Non-Goals:**
- 미디어 후보의 순서/grouping 변경 안 함. 기존대로 image → video → audio 순 grouping을 선택된 범위 내에서 유지한다.
- `<main>` 등 추가 본문 컨테이너 인식 확장 안 함. spec이 정의한 `<article>`/`<body>`만 따른다(보수적 선택).
- 썸네일(`firstArticleImage`)·`articleText`·50 cap·srcset 파싱 등 기존 동작 변경 안 함.
- `image_picker.go`(별개 썸네일 우선순위 경로)·`MediaValidator` wiring(별개 change)과 무관.

## Decisions

1. **미디어 버킷을 article 범위와 body 범위로 분리한다(dual-bucket).** 텍스트의 `articleTextBuf`/`bodyTextBuf`와 동형. `extractScan`의 `mediaImages`/`mediaVideos`/`mediaAudios`를 `bodyImages`/`bodyVideos`/`bodyAudios`로 명명하고, `articleImages`/`articleVideos`/`articleAudios`를 추가한다.
   - `walk`의 `<img>`/`<video>`/`<audio>`와 `handleSource`는 `inArticle`이면 article 버킷, 아니면 body 버킷에 append.
   - `handleSource`는 현재 `inArticle`을 받지 않으므로 시그니처에 `inArticle bool`을 추가하고 호출부(`case "source"`)에서 전달한다.
2. **`buildMediaCandidates`가 `sawArticle`로 범위를 선택한다.**
   - `sawArticle`이면 article 버킷, 아니면 body 버킷을 절대화/중복제거/50 cap 파이프라인에 넣는다.
   - 이렇게 하면 `<article>` 있는 페이지는 그 내부만, 없는 페이지는 body 전체(모두 `inArticle=false`로 body 버킷에 누적)를 수집한다.
3. **절대화·cap·grouping 경로는 그대로 둔다.** 입력 슬라이스 선택만 바꾸고 `buildMediaCandidates`의 나머지 로직(absolutize/seen dedup/limit)은 손대지 않아 회귀 표면을 최소화한다.

## Risks / Trade-offs

- **회귀 위험: 낮음~중간.** `<article>` 없는 페이지는 모든 미디어가 `inArticle=false`로 body 버킷에 들어가 기존과 동일한 집합·순서를 유지한다. `<article>` 있는 페이지에서만 출력이 바뀌며, 이는 spec 준수 방향의 의도된 변경이다.
- **관찰 가능한 행위 변화.** article 밖 미디어가 제외되어 일부 페이지의 `media_candidates`가 줄거나 비고, `no_primary_media` skip이 늘 수 있다. spec의 의도된 행위이며 운영 지표에 명시한다.
- **Trade-off: `<article>` 우선.** 한 페이지에 `<article>`이 여럿이면 각 article 내부 미디어가 모두 article 버킷에 누적된다(기존 `articleText`/sawArticle 시맨틱과 동일). spec은 "`<article>` 태그가 있으면 그 내부"라 복수 article을 별도 구분하지 않으므로 일관 동작.
- **롤백:** extractor.go 내부 변경이라 커밋 revert로 즉시 원복. DB/스키마 영향 없음.
