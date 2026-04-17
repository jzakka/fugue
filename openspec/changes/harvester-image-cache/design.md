## Context

Harvester는 현재 `apps/api/internal/bot/harvest_pipeline.go`의 `HarvestPipeline.Process` → `downloadAndUpload`에서 **본문 미디어**(item.MediaURL, 이미지/오디오/비디오 파일 자체)만 object storage로 업로드하고, Pin row의 `media_url`로 사용한다. 반면 `og_image`/thumbnail에 해당하는 **소셜 미리보기/대표 이미지**는 비어 있다(`OgImage: sql.NullString{}`로 하드코딩). `apps/api/fuguebot_pseudo.go`의 `ParseDocument`도 빈 stub이다.

`docs/erd.md`의 `pin.thumbnail_url`(VARCHAR 1000), `pin.og_image`(VARCHAR 1000)는 이미 존재하지만, 어떤 값이 들어가야 하는지(원본 URL인지, 우리 storage URL인지)가 명세되지 않았다. 피드/그리드는 `media_url` fallback으로 동작 중이라 외부 origin 의존이 잠재적으로 깔려 있다.

후속 change(`harvester-pin-document`)가 Pin 문서 구조를 정리하는 것과 별개로, **이미지 바이트의 영속화**는 독립적으로 결정·적용할 수 있다. 본 change는 그 한 가지 책임만 다룬다.

## Goals / Non-Goals

**Goals:**
- Harvester가 Pin을 만들 때 정확히 1개의 primary 이미지를 우리 object storage로 가져와 영속화한다.
- 추출 우선순위, 저장 키 스킴, 실패 fallback을 spec 수준에서 고정하여, 어떤 source/script든 동일한 이미지 캐싱 동작을 갖는다.
- `pin.thumbnail_url`/`pin.og_image` 컬럼의 의미를 "캐시 성공 시 storage URL, 실패 시 원본 URL"로 통일한다.

**Non-Goals:**
- WebP/AVIF 변환, multi-size(thumbnail/medium/large) 생성 → `harvester-image-webp`.
- 고아 이미지 GC(Pin 삭제 시 storage 정리) → `harvester-image-gc`.
- CDN 도메인 분리, 서명 URL → `harvester-image-cdn`.
- Pin 문서 스키마 자체 변경, 멀티 미디어 → `harvester-pin-document`.
- 본문 미디어(`media_url`, 비디오/오디오 본체)의 저장 정책 변경. 본 change는 **썸네일/대표 이미지**만 다룬다.
- 이미지 추출에 headless 브라우저/JS 실행 도입. 본 change는 정적 HTML 파싱 범위 안에서 동작한다.

## Decisions

### Decision 1: 추출 우선순위는 og:image → twitter:image → article 내 주요 img → JSON-LD `image`

**선택**: 위 4단계를 순서대로 시도하고, 첫 번째 "유효" 후보를 채택. "유효"는 (1) URL 파싱 성공, (2) http/https 스킴, (3) data: URI 제외, (4) 1×1 픽셀 추적용 의심 패턴(`pixel`, `1x1`, `spacer` 등) 제외를 의미한다.

**대안**:
- (A) 첫 번째 `<img>` 무조건 채택 — 광고/로고/네비 아이콘이 잡힐 위험.
- (B) 모든 후보를 다 받아 점수화(크기, 위치, alt 텍스트) — 구현 복잡, 본 change 범위 초과.
- (C) 본 채택안: 우선순위 시퀀스. og/twitter는 사이트 운영자가 명시한 대표 이미지라 신뢰도가 높고, 없으면 본문 img, 그래도 없으면 schema.org JSON-LD로 폴백.

**근거**:
- 현실의 콘텐츠 사이트 대부분이 og:image 또는 twitter:image를 제공한다(SEO/SNS 공유 이유).
- "article 내 주요 img"는 `<article>` 또는 `<main>` 안에 있는 `<img>` 중 alt가 비어있지 않거나 width/height가 둘 다 100px 이상인 첫 번째 요소로 정의(휴리스틱). 정의가 흔들리지 않도록 spec scenario에서 고정.
- JSON-LD `image`는 schema.org Article/NewsArticle/Recipe 등에 있는 string|string[]|ImageObject. 첫 string으로 평탄화.

### Decision 2: 저장 키 = `images/<sha256(normalized_url)>/<unix_ts>.<ext>`

**선택**: prefix `images/`, 그 아래 디렉터리는 정규화된 원본 이미지 URL의 SHA-256 hex 전체(64자), 파일명은 캐시 시각의 unix epoch + 확장자.

**대안**:
- (A) `images/<uuid>.<ext>` — 동일 원본 이미지가 여러 Pin에서 참조될 때 dedup 불가, 디버깅 시 원본 추적 어려움.
- (B) `images/<host>/<hash>.<ext>` — host 단위 grouping은 GC/모니터링에 유용하지만 도메인 변경/리다이렉트 시 위치가 흔들림.
- (C) 본 채택안: hash 디렉터리 + timestamp 파일.

**근거**:
- **dedup 친화**: hash가 동일하면 같은 디렉터리에 모인다. dedup 자체는 본 change에서 하지 않지만, 후속 change에서 "동일 hash 디렉터리 내 가장 최근 파일을 쓴다" 식의 정책을 추가 비용 없이 도입할 수 있다.
- **재캐시 안전**: 동일 URL을 다시 캐시해도 timestamp가 다르므로 덮어쓰기 충돌 없음. 이전 버전도 보존되어 "원본 이미지가 바뀌었는지" 추적 가능.
- **GC 친화**: `images/` prefix를 통째로 LIST하여 후속 GC change가 "DB 어느 Pin도 참조하지 않는 키"를 식별 가능.
- `<ext>`는 응답 `Content-Type`(`image/jpeg` → `.jpg`, `image/png` → `.png`, `image/webp` → `.webp`, `image/gif` → `.gif`)에서 도출하고, 매핑 실패 시 원본 URL의 확장자, 그래도 없으면 `.bin`.

### Decision 3: TTL은 두지 않는다 (사실상 영구 보관)

**선택**: object storage 측 lifecycle 만료 정책을 적용하지 않는다. 본 change는 GC를 정의하지 않는다.

**대안**:
- (A) 365일 만료 — 만료 후 Pin 썸네일이 다시 깨지면 본 change의 가치가 사라진다.
- (B) Pin 삭제 시점에 같이 삭제(연쇄) — Pin 삭제 트랜잭션과 storage 호출이 묶여 운영 복잡도 상승. `harvester-image-gc`의 별도 비동기 작업이 더 안전.
- (C) 본 채택안: 일단 무기한 보관, GC는 후속 change.

**근거**:
- Pin은 "한 번 만들어 영속적으로 노출"되는 개념이라 thumbnail도 동일 수명을 가져야 한다.
- Storage 비용 증가는 모니터링으로 감지 가능하며, 비용이 문제가 되기 전에 `harvester-image-gc`를 도입할 시간이 충분하다.

### Decision 4: 실패 시 원본 URL fallback, Pin 생성은 절대 막지 않는다

**선택**: 이미지 다운로드/업로드 단계의 모든 에러는 catch하여 로그/메트릭으로만 남기고, Pin row의 `thumbnail_url`/`og_image`에 **원본 후보 URL**을 그대로 저장한다. 이미지 후보 자체를 못 찾았으면 두 컬럼을 NULL로 둔다.

**근거**:
- 이미지 캐싱은 부가 기능. Harvester 본업(Pin 생성)을 막아서는 안 된다.
- 원본 URL을 보존하면, 후속 재시도 작업(`harvester-image-recache`, 본 change 범위 외)이 Pin row만 보고 다시 시도할 수 있다.

### Decision 5: 본문 미디어(`media_url`)와는 별개 경로

**선택**: 기존 `downloadAndUpload`(item.MediaURL → `bot/<uuid>` prefix)는 그대로 둔다. 신설되는 image cache는 `images/<hash>/<ts>` prefix로 별도 함수에서 처리한다.

**근거**:
- 본문 미디어는 "Pin의 본체"(필수, 1차 콘텐츠)이고 thumbnail은 "메타데이터"(부가, 실패 허용). 실패 의미가 다르므로 코드 경로도 분리.
- Prefix가 분리되어야 모니터링/GC 정책을 독립적으로 운영할 수 있다.

### Decision 6: 추출 후보 URL 정규화 규칙

**선택**: 캐시 hash를 만들기 전, 후보 URL에 다음 정규화를 적용한다 — (1) 페이지 URL 기준 절대 URL로 resolve, (2) fragment(`#...`) 제거, (3) 쿼리 파라미터는 보존(이미지 CDN이 서명 파라미터로 응답을 구분하는 경우가 많음), (4) host는 lower-case.

**근거**:
- 동일 이미지를 가리키는 상대 URL/절대 URL이 서로 다른 hash를 갖는 것을 방지(절대화).
- 쿼리 보존은 이미지 CDN(Cloudflare Images, imgix 등)이 같은 이미지에 다른 변환을 적용한 결과를 다른 자원으로 본다는 사실에 맞춘다.
- 더 강한 정규화(쿼리 파라미터 정렬/제거)는 사이트별 차이가 커서 본 change에서는 도입하지 않는다.

## Risks / Trade-offs

- **광고/추적 픽셀이 article img 단계에서 잡힐 수 있다** → 1×1 픽셀 휴리스틱과 width/height 100px 컷오프로 1차 방어. 그래도 통과한 경우는 운영 모니터링으로 감지 후 휴리스틱 보강.
- **대용량 이미지(수십 MB) 다운로드로 Harvester가 느려질 수 있다** → 다운로드 시 Content-Length 검사하여 임계치(예: 20MB) 초과 시 캐시 포기, 원본 URL fallback. 임계치 자체는 구현 단계에서 환경변수화.
- **원본 서버가 Referer/User-Agent로 hotlink 차단** → 첫 시도 실패 시 fallback이 동작하므로 spec 동작은 유지. Referer/UA 우회는 본 change 범위 외.
- **Storage 비용 누적** → `images/` prefix 단위 모니터링. 일정 임계 이상이면 `harvester-image-gc` 우선순위 상향.
- **동일 이미지 재캐시로 인한 storage 낭비** → hash 디렉터리 안에 timestamp 파일이 누적될 수 있음. 후속 GC change에서 "디렉터리당 최신 1개만 유지" 정책으로 정리 가능. 본 change에서는 단순성을 위해 항상 새 timestamp로 저장.
- **Object storage 장애 시 Pin 생성이 매번 fallback으로 빠짐** → 대량 Pin이 원본 URL로 저장되어 storage 복구 후 일관성 깨짐. 메트릭으로 fallback 비율을 노출하여 alert. 재캐시 잡은 후속 change.

## Migration Plan

1. 기존 Pin row(이미 만들어진 것)는 건드리지 않는다. 본 change는 **새로 만들어지는 Pin**부터 적용된다.
2. 코드 변경: `harvest_pipeline.go`의 Pin 생성 직전에 image cache step 추가, 추출 로직은 새 파일(`image_picker.go` 등)로 분리. `Storage.Upload` 그대로 사용.
3. Object storage 정책 변경 없음(만료 미설정). `images/` prefix가 신설된다.
4. 롤백: 코드 revert. 이미 `images/` prefix에 들어간 객체는 그대로 두어도 무해(아무도 읽지 않음). 추후 `harvester-image-gc`가 정리.
5. 백필(과거 Pin의 og_image/thumbnail 캐시)은 본 change 범위 외. 별도 one-off job으로 진행 가능.

## Open Questions

- **WebP 변환을 본 change에 포함할지** — 현재 결론: 포함하지 않음. 변환 비용/품질 손실 트레이드오프와 의존성(libwebp)이 별도 검토 가치를 가지므로 `harvester-image-webp`로 분리. 사용자 결정값(`optional webp 변환`)도 "옵션"으로 명시되어 있어 별도 change가 적절.
- **이미지 추출이 Harvester 스크립트(JS) 결과물에서 직접 와야 하는가, HTML에서 추출해야 하는가** — 본 change는 HTML 정적 파싱 기준. 스크립트가 명시적으로 `thumbnailURL`을 반환하면 그것을 1순위로 쓰는 옵션은 후속 검토(`harvester-script-thumbnail-hint`).
- **다운로드 크기 임계치(20MB) 기본값이 적절한가** — 구현 시 환경변수 default로 두고 운영 데이터로 튜닝.
- **동일 hash 디렉터리에 timestamp 파일이 무한 누적되는 것을 본 change에서 막을지** — 막지 않음. GC change로 위임.
