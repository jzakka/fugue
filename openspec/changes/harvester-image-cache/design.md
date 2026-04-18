## Context

Harvester는 현재 `apps/api/internal/bot/harvest_pipeline.go`의 `HarvestPipeline.Process` → `downloadAndUpload`에서 **본문 미디어**(item.MediaURL, 이미지/오디오/비디오 파일 자체)만 object storage로 업로드하고, Pin row의 `media_url`로 사용한다. 반면 `og_image`/thumbnail에 해당하는 **소셜 미리보기/대표 이미지**는 비어 있다(`OgImage: sql.NullString{}`로 하드코딩). `apps/api/fuguebot_pseudo.go`의 `ParseDocument`도 빈 stub이다.

`docs/erd.md`의 `pin.thumbnail_url`(VARCHAR 1000), `pin.og_image`(VARCHAR 1000)는 이미 존재하지만, 어떤 값이 들어가야 하는지(원본 URL인지, 우리 storage URL인지)가 명세되지 않았다. 피드/그리드는 `media_url` fallback으로 동작 중이라 외부 origin 의존이 잠재적으로 깔려 있다.

후속 change(`harvester-pin-document`)가 Pin 문서 구조를 정리하는 것과 별개로, **이미지 바이트의 영속화**는 독립적으로 결정·적용할 수 있다. 본 change는 그 한 가지 책임만 다룬다.

## Goals / Non-Goals

**Goals:**
- Harvester가 Pin을 만들 때 정확히 1개의 primary 이미지를 우리 object storage로 가져와 영속화한다.
- 추출 우선순위, 저장 키 스킴, 실패 fallback을 spec 수준에서 고정하여, 어떤 source/script든 동일한 이미지 캐싱 동작을 갖는다.
- 캐시 동작의 호출 규약을 `(string, error)` 시그니처로 단순화하여, 호출자가 성공/실패 분기 없이 반환 문자열을 그대로 컬럼에 쓰게 한다.
- `pin.thumbnail_url`/`pin.og_image` 컬럼의 의미를 "캐시 성공 시 storage URL, 실패 시 원본 URL"로 통일하고, **두 컬럼에 동일 값**을 기록한다.

**Non-Goals:**
- WebP/AVIF 변환, multi-size(thumbnail/medium/large) 생성 → `harvester-image-webp`.
- 고아 이미지 GC(Pin 삭제 시 storage 정리) → `harvester-image-gc`.
- CDN 도메인 분리, 서명 URL → `harvester-image-cdn`.
- Pin 문서 스키마 자체 변경, 멀티 미디어 → `harvester-pin-document`.
- 본문 미디어(`media_url`, 비디오/오디오 본체)의 저장 정책 변경. 본 change는 **썸네일/대표 이미지**만 다룬다.
- 이미지 추출에 headless 브라우저/JS 실행 도입. 본 change는 정적 HTML 파싱 범위 안에서 동작한다.
- URL 정규화 edge case(쿼리 정렬/제거, 퍼센트 인코딩 차이), SHA-256 hash 충돌 대응, unix 초 해상도 충돌 등은 본 change 범위 외 — 후속 change에서 다룬다.

## Decisions

### Decision 1: cacheImage 반환 시그니처는 `(url string, err error)`

**선택**: 캐시 헬퍼는 `cacheImage(ctx, url) (string, error)` 시그니처를 가진다.
- **성공**: 반환 `string`은 object storage URL, `err == nil`.
- **실패**(다운로드 실패/업로드 실패/크기 초과): 반환 `string`은 **입력 원본 URL을 그대로**, `err`은 실패 원인(로그 목적).
- **후보 없음**: 호출 자체를 하지 않는다(상위에서 후보가 있을 때만 호출). 후보 없음 시 Pin 컬럼은 NULL.

**대안**:
- (A) `(storageURL string, originalURL string, err error)` 3-튜플 — 호출자가 세 가지 경우를 분기해야 함. 분기 실수 위험.
- (B) `(string, error)`에서 에러 시 빈 문자열 반환, 호출자가 fallback 판단 — 호출자에 분기 중복 발생.
- (C) 본 채택안: 실패해도 반환 `string`에 원본 URL을 담아 **호출자는 항상 반환 string을 그대로 Pin 컬럼에 기록**한다. err은 관찰 목적.

**근거**:
- 호출자(`HarvestPipeline.Process`) 코드가 단순해진다: `val, err := cacheImage(ctx, url); if err != nil { log }; pin.OgImage = val; pin.Thumbnail = val`.
- err이 nil인지와 "무엇이 Pin 컬럼에 들어가야 하는가"가 분리된다.

### Decision 2: thumbnail_url과 og_image는 동일 값으로 기록한다

**선택**: `pin.thumbnail_url`과 `pin.og_image`는 `cacheImage`가 반환한 **동일 문자열**을 둘 다 받는다. 별도 처리 분기는 두지 않는다.

**대안**:
- (A) og_image에는 원본 URL, thumbnail_url에는 캐시 URL — 두 소스가 섞여 피드/그리드에서 fallback 규칙이 복잡해진다.
- (B) og_image만 캐시하고 thumbnail_url은 비워 둠 — 피드/그리드 코드가 ifnull 로직을 들고 있어야 함.
- (C) 본 채택안: 항상 동일 값. 소비 쪽이 "둘 중 아무거나" 써도 동일.

**근거**:
- 두 컬럼의 "의미"는 본 change 기준으로 동일("Pin의 대표 이미지 URL — 캐시 성공 시 storage, 실패 시 원본").
- 컬럼 분리를 유지하는 이유는 후속 change(`harvester-pin-document`)에서 "og 원본 URL"과 "우리 썸네일 URL"을 분리할 여지를 남기기 위함. 본 change는 단순화를 택한다.

### Decision 3: URL 정규화 규칙 (hash 입력용)

**선택**: 캐시 hash를 만들기 전, 후보 URL에 다음 정규화를 적용한다.
1. 페이지 URL 기준 **절대 URL**로 resolve.
2. **fragment(`#...`) 제거**.
3. **host는 lower-case**.
4. **쿼리 파라미터는 보존**(이미지 CDN이 서명/변환 파라미터로 응답을 구분하는 경우가 많음).

**근거**:
- (1)은 상대/절대 URL이 같은 이미지에 대해 다른 hash를 만드는 것을 방지.
- (2)는 fragment가 서버 응답에 영향을 주지 않으므로 제거.
- (3)은 대소문자 차이로 hash가 달라지는 것을 방지.
- (4)는 Cloudflare Images/imgix/Akamai 등에서 같은 원본에 다른 변환을 적용한 결과를 **다른 객체로 취급**(별도 저장)하기 위함.

### Decision 4: 실패 시 원본 URL fallback — Pin 생성은 절대 막지 않는다

**선택**: 이미지 다운로드/업로드/크기 초과 단계의 모든 에러는 catch하여 로그/메트릭으로만 남기고, Pin row의 `thumbnail_url`/`og_image`에 **채택된 원본 후보 URL**을 그대로 저장한다. 이미지 후보 자체를 못 찾았으면 두 컬럼을 NULL로 둔다.

**근거**:
- 이미지 캐싱은 부가 기능. Harvester 본업(Pin 생성)을 막아서는 안 된다.
- 원본 URL을 보존하면, 후속 재시도 작업(`harvester-image-recache`, 본 change 범위 외)이 Pin row만 보고 다시 시도할 수 있다.
- fallback 경로가 단일(모든 실패를 하나로 취급)이라 호출자 분기가 없다.

### Decision 5: 크기 초과 처리는 단일 fallback 경로로 흡수한다

**선택**: 다음 두 경우 모두 다운로드를 **중단**하고, 부분 다운로드 바이트를 버리고, 원본 URL fallback 경로로 진입한다.
- 응답 헤더의 **Content-Length가 임계치 초과**(선제 검사).
- read 누적 바이트가 임계치 초과(헤더가 없거나 거짓말할 때).

**근거**:
- "크기 초과"는 실패의 한 종류일 뿐이므로 Decision 4의 단일 fallback 경로로 흡수하는 것이 API/테스트 표면을 줄인다.
- 부분 데이터 업로드는 재생 시 깨진 이미지를 만들 수 있어 무조건 버린다.

### Decision 6: URL 쿼리 파라미터 정렬/제거는 도입하지 않는다

**선택**: Decision 3의 정규화는 스킴/호스트/fragment만 건드린다. 쿼리 파라미터는 이름순 정렬하지 않고, 특정 파라미터(`utm_*` 등)를 제거하지도 않는다.

**근거**:
- CDN 변환 파라미터(`?w=800`, `?format=webp`, 서명 토큰 등)가 존재할 때, 파라미터 순서/유무에 따라 **서버 응답이 달라지는** 것이 현실이다. 같은 이미지처럼 보여도 다른 바이트를 낸다.
- 따라서 쿼리가 다른 URL은 **다른 객체로 취급**한다(즉, 다른 hash → 다른 storage 키). 이는 dedup을 일부 희생하지만, 안정성(틀린 이미지 반환 방지)을 얻는다.
- 정렬/제거 규칙은 사이트마다 차이가 커서 본 change에서 도입하면 오히려 버그 원인이 된다. 필요해지면 후속 change로 분리.

### Decision 7: 본문 미디어(`media_url`)와는 별개 경로

**선택**: 기존 `downloadAndUpload`(item.MediaURL → `bot/<uuid>` prefix)는 그대로 둔다. 신설되는 image cache는 `images/<hash>/<ts>` prefix로 별도 함수에서 처리한다.

**근거**:
- 본문 미디어는 "Pin의 본체"(필수, 1차 콘텐츠)이고 thumbnail은 "메타데이터"(부가, 실패 허용). 실패 의미가 다르므로 코드 경로도 분리.
- Prefix가 분리되어야 모니터링/GC 정책을 독립적으로 운영할 수 있다.

## Risks / Trade-offs

- **광고/추적 픽셀이 article img 단계에서 잡힐 수 있다** → 1×1 픽셀 휴리스틱과 width/height 100px 컷오프로 1차 방어. 그래도 통과한 경우는 운영 모니터링으로 감지 후 휴리스틱 보강.
- **대용량 이미지(수십 MB) 다운로드로 Harvester가 느려질 수 있다** → Content-Length 검사 + read 누적 검사 양쪽으로 임계치(예: 20MB) 초과 시 fallback 경로 진입. 임계치는 구현 단계에서 환경변수화.
- **원본 서버가 Referer/User-Agent로 hotlink 차단** → 첫 시도 실패 시 fallback이 동작하므로 spec 동작은 유지. Referer/UA 우회는 본 change 범위 외.
- **Storage 비용 누적** → `images/` prefix 단위 모니터링. 일정 임계 이상이면 `harvester-image-gc` 우선순위 상향.
- **쿼리 파라미터 보존으로 dedup 실패** → CDN 변환 URL이 많을수록 같은 원본 이미지에 대해 여러 객체가 생긴다. Decision 6의 근거처럼 안정성을 우선.
- **Object storage 장애 시 Pin 생성이 매번 fallback으로 빠짐** → 대량 Pin이 원본 URL로 저장되어 storage 복구 후 일관성 깨짐. 메트릭(가능하면)/로그로 fallback 비율을 노출하여 감지. 재캐시 잡은 후속 change.
- **thumbnail/og_image 동일값 기록은 후속 차별화를 어렵게 한다** → 후속 change(`harvester-pin-document`)에서 컬럼 의미를 분리할 때 본 change의 기록을 마이그레이션할 필요가 생길 수 있음. 그 비용은 본 change 단순화 이득으로 상쇄된다고 판단.

## Migration Plan

1. 기존 Pin row(이미 만들어진 것)는 건드리지 않는다. 본 change는 **새로 만들어지는 Pin**부터 적용된다.
2. 코드 변경: `harvest_pipeline.go`의 Pin 생성 직전에 image cache step 추가, 추출 로직은 새 파일(`image_picker.go` 등)로 분리. 캐시 헬퍼는 `(string, error)` 시그니처. `Storage.Upload` 그대로 사용.
3. Object storage 정책 변경 없음(만료 미설정 — storage 운영자가 필요 시 별도 결정). `images/` prefix가 신설된다.
4. 롤백: 코드 revert. 이미 `images/` prefix에 들어간 객체는 그대로 두어도 무해(아무도 읽지 않음). 추후 `harvester-image-gc`가 정리.
5. 백필(과거 Pin의 og_image/thumbnail 캐시)은 본 change 범위 외.
