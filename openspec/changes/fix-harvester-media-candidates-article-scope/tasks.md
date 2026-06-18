## 1. 미디어 수집 본문 범위(article-scope) 구현

- [x] 1.1 `apps/api/internal/bot/extractor.go`의 `extractScan`에 article 범위 미디어 버킷(`articleImages`/`articleVideos`/`articleAudios`) 추가, 기존 `mediaImages`/`mediaVideos`/`mediaAudios`를 body 범위(`bodyImages`/`bodyVideos`/`bodyAudios`)로 명명
- [x] 1.2 `walk`의 `<img>`/`<video>`/`<audio>` 분기에서 `inArticle`이면 article 버킷, 아니면 body 버킷에 append
- [x] 1.3 `handleSource`에 `inArticle bool` 매개변수 추가, `case "source"` 호출부에서 전달, type별로 article/body 버킷에 분기 append
- [x] 1.4 `buildMediaCandidates`가 `sawArticle`이면 article 버킷을, 아니면 body 버킷을 절대화/중복제거/50 cap 파이프라인에 사용
- [x] 1.5 썸네일(`firstArticleImage`)·`articleText`·grouping(image→video→audio)·50 cap 기존 동작 보존 확인

## 2. 테스트

- [x] 2.1 `extractor_test.go`에 `<body><img src="/nav-logo.png" alt="logo" width="200" height="80"><article><p>본문</p></article></body>` fixture(article 내부 미디어 없음, 밖에 적격 img) — `media_candidates`가 빈 배열(nav-logo 미수집)임을 검증(수정 전 버그 재현)
- [x] 2.2 `<article>` 내부 미디어 + 밖 미디어 혼재 fixture — `media_candidates`가 article 내부 미디어만 포함함을 검증
- [x] 2.3 회귀: `<article>` 없는 페이지(body 전체 수집)·기존 미디어 type/50cap/srcset 첫 URL 테스트가 기존과 동일하게 통과하는지 확인
- [x] 2.4 `cd apps/api && go vet ./... && go build ./... && go test ./...` 통과

## 3. 실 환경 QA

- [x] 3.1 fixture HTML(article 내부 미디어 없음, 밖에 img 있음)을 `httptest` 서버로 서빙하고 production `GenericExtractor.Extract`로 페치+추출하여 `media_candidates`가 article 밖 미디어를 포함하지 않음을 관찰(수정 전 오염 재현 → 수정 후 제외)
- [x] 3.2 회귀: `<article>` 없는 페이지의 미디어 수집이 무회귀(body 전체 수집)임을 확인
