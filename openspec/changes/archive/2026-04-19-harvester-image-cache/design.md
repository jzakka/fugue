## Context

Harvester는 현재 `apps/api/internal/bot/harvest_pipeline.go`의 `HarvestPipeline.Process` → `downloadAndUpload`에서 **본문 미디어**(item.MediaURL, 이미지/오디오/비디오 파일 자체)만 object storage로 업로드하고, Pin row의 `media_url`로 사용한다. 반면 `og_image`에 해당하는 **소셜 미리보기/대표 이미지**는 비어 있다(`OgImage: sql.NullString{}`로 하드코딩). `apps/api/fuguebot_pseudo.go`의 `ParseDocument`도 빈 stub이다.

`docs/erd.md` 및 실제 마이그레이션 확인 결과 `pins` 테이블에는 `og_image`(VARCHAR 1000, nullable) 컬럼만 존재하며, `thumbnail_url` 컬럼은 **존재하지 않는다**. 본 change는 스키마 변경을 도입하지 않고 기존 `og_image` 한 컬럼에 "캐시 성공 시 storage URL, 실패 시 원본 URL, 후보 없음 시 NULL"을 기록한다. 썸네일용 별도 컬럼 분리가 필요해지면 후속 change(`harvester-pin-document`)에서 다룬다.

후속 change(`harvester-pin-document`)가 Pin 문서 구조를 정리하는 것과 별개로, **이미지 바이트의 영속화**는 독립적으로 결정·적용할 수 있다. 본 change는 그 한 가지 책임만 다룬다.

## Goals / Non-Goals

**Goals:**
- Harvester가 Pin을 만들 때 정확히 1개의 primary 이미지를 우리 object storage로 가져와 영속화한다.
- 추출 우선순위, 저장 키 스킴, 실패 fallback을 spec 수준에서 고정하여, 어떤 source/script든 동일한 이미지 캐싱 동작을 갖는다.
- 캐시 동작의 호출 규약을 `(string, error)` 시그니처로 단순화하여, 호출자가 성공/실패 분기 없이 반환 문자열을 그대로 Pin의 대표 이미지 URL 필드에 쓰게 한다.
- Pin의 대표 이미지 URL 필드(현행 스키마에서 `og_image`)의 의미를 "캐시 성공 시 storage URL, 실패 시 원본 URL, 후보 없음 시 NULL"로 통일한다.

**Non-Goals:**
- WebP/AVIF 변환, multi-size(thumbnail/medium/large) 생성 → `harvester-image-webp`.
- 고아 이미지 GC(Pin 삭제 시 storage 정리) → `harvester-image-gc`.
- CDN 도메인 분리, 서명 URL → `harvester-image-cdn`.
- Pin 문서 스키마 자체 변경, 썸네일 전용 컬럼 추가, 멀티 미디어 → `harvester-pin-document`.
- 본문 미디어(`media_url`, 비디오/오디오 본체)의 저장 정책 변경. 본 change는 **대표 이미지**만 다룬다.
- 이미지 추출에 headless 브라우저/JS 실행 도입. 본 change는 정적 HTML 파싱 범위 안에서 동작한다.
- URL 정규화 edge case(쿼리 정렬/제거, 퍼센트 인코딩 차이), SHA-256 hash 충돌 대응, unix 초 해상도 충돌 등은 본 change 범위 외 — 후속 change에서 다룬다.

## Decisions

### Decision 1: cacheImage 반환 시그니처는 `(url string, err error)`

**선택**: 캐시 헬퍼는 `cacheImage(ctx, url) (string, error)` 시그니처를 가진다.
- **성공**: 반환 `string`은 object storage URL, `err == nil`.
- **실패**(다운로드 실패/업로드 실패/크기 초과): 반환 `string`은 **입력 원본 URL을 그대로**, `err`은 실패 원인(로그 목적).
- **후보 없음**: 호출 자체를 하지 않는다(상위에서 후보가 있을 때만 호출). 후보 없음 시 Pin 대표 이미지 URL 필드는 NULL.

**대안**:
- (A) `(storageURL string, originalURL string, err error)` 3-튜플 — 호출자가 세 가지 경우를 분기해야 함. 분기 실수 위험.
- (B) `(string, error)`에서 에러 시 빈 문자열 반환, 호출자가 fallback 판단 — 호출자에 분기 중복 발생.
- (C) 본 채택안: 실패해도 반환 `string`에 원본 URL을 담아 **호출자는 항상 반환 string을 그대로 Pin 대표 이미지 URL 필드에 기록**한다. err은 관찰 목적.

**근거**:
- 호출자(`HarvestPipeline.Process`) 코드가 단순해진다: `val, err := cacheImage(ctx, url); if err != nil { log }; pin.OgImage = val`.
- err이 nil인지와 "무엇이 Pin 컬럼에 들어가야 하는가"가 분리된다.

### Decision 2: Pin의 대표 이미지 URL은 단일 필드(`og_image`)에만 기록한다

**선택**: 본 change는 현행 `pins` 스키마를 변경하지 않으며, Pin의 대표 이미지 URL은 **기존 `og_image` 컬럼 하나**에만 기록한다. 캐시 헬퍼 결과(성공 시 storage URL, 실패 시 원본 URL, 후보 없음 시 NULL)가 이 컬럼의 값이 된다.

**대안**:
- (A) 본 change에서 `thumbnail_url` 컬럼을 신규 추가하는 마이그레이션을 포함하고 `og_image`와 동일 값을 두 컬럼에 기록 — 스키마 변경이라는 별개 책임이 본 change scope에 섞여 커지고, 두 컬럼 의미 분리는 정의되지 않은 채 코드/스펙에만 "둘이 같다"는 제약이 남는다.
- (B) `og_image`에는 원본 URL, (가상의) `thumbnail_url`에는 storage URL — 두 소스가 섞여 소비 코드가 fallback 규칙을 들고 있어야 함. 또한 존재하지 않는 컬럼 의존.
- (C) 본 채택안: 단일 컬럼 `og_image`만 사용. 소비 코드는 `og_image` 하나만 읽으면 된다. 썸네일 전용 필드가 필요해지면 후속 change(`harvester-pin-document`)에서 마이그레이션과 함께 도입.

**근거**:
- 본 change의 책임은 "이미지 바이트 영속화"이며 "Pin 문서 스키마 개편"이 아니다.
- 실제 코드베이스(`apps/api/db/queries/pins.sql`, `apps/api/internal/db/pins.sql.go`)는 `og_image`만 가진다. 새 컬럼을 도입하면 `CreatePin` SQL·sqlc 재생성·조회 쿼리 수정까지 연쇄 변경이 발생하여 scope가 커진다.
- 소비 코드(피드/그리드)는 `og_image`가 "Pin의 대표 이미지 URL"이라는 단일 사실만 알면 된다.

### Decision 3: URL 정규화 규칙 (hash 입력용)

**선택**: 캐시 hash를 만들기 전, 후보 URL에 다음 정규화를 적용한다.
1. 페이지 URL 기준 **절대 URL**로 resolve.
2. **fragment(`#...`) 제거**.
3. **scheme은 lower-case**(RFC 3986상 scheme은 case-insensitive).
4. **host는 lower-case**.
5. **쿼리 파라미터는 보존**(이미지 CDN이 서명/변환 파라미터로 응답을 구분하는 경우가 많음).
6. **위에 명시되지 않은 컴포넌트(path, query, userinfo 등)는 원문 그대로 보존**한다. 특히 path는 RFC 3986상 case-sensitive이므로 소문자화하지 않는다(대소문자가 다른 path는 서버 입장에서 서로 다른 리소스일 수 있다).

**근거**:
- (1)은 상대/절대 URL이 같은 이미지에 대해 다른 hash를 만드는 것을 방지.
- (2)는 fragment가 서버 응답에 영향을 주지 않으므로 제거.
- (3)은 `HTTP://` vs `http://` 대소문자 차이로 hash가 달라져 dedup이 깨지는 것을 방지. RFC 3986 §3.1에서 scheme은 case-insensitive로 명시됨.
- (4)는 호스트 대소문자 차이로 hash가 달라지는 것을 방지.
- (5)는 Cloudflare Images/imgix/Akamai 등에서 같은 원본에 다른 변환을 적용한 결과를 **다른 객체로 취급**(별도 저장)하기 위함.

### Decision 4: 실패 시 원본 URL fallback — Pin 생성은 절대 막지 않는다

**선택**: 이미지 다운로드/업로드/크기 초과 단계의 모든 에러는 catch하여 로그/메트릭으로만 남기고, Pin row의 대표 이미지 URL 필드(`og_image`)에 **채택된 원본 후보 URL**을 그대로 저장한다. 이미지 후보 자체를 못 찾았으면 해당 컬럼을 NULL로 둔다.

**근거**:
- 이미지 캐싱은 부가 기능. Harvester 본업(Pin 생성)을 막아서는 안 된다.
- 원본 URL을 보존하면, 후속 재시도 작업(`harvester-image-recache`, 본 change 범위 외)이 Pin row만 보고 다시 시도할 수 있다.
- fallback 경로가 단일(모든 실패를 하나로 취급)이라 호출자 분기가 없다.

### Decision 5: 크기 초과 처리는 단일 fallback 경로로 흡수한다

**선택**: 다음 두 경우 모두 다운로드를 **중단**하고, 부분 다운로드 바이트를 버리고, 원본 URL fallback 경로로 진입한다.
- 응답 헤더의 **Content-Length가 임계치 초과**(선제 검사).
- read 누적 바이트가 임계치 초과(헤더가 없거나 거짓말할 때).

**기본 임계치**: `20 MiB` (= `20 * 1024 * 1024` bytes = `20,971,520` bytes). 환경변수(`HARVESTER_IMAGE_CACHE_MAX_BYTES` 또는 `HarvestPipeline` 생성자 옵션 `ImageCacheMaxBytes`)로 덮어쓸 수 있다. proposal/tasks/spec/Risks가 참조하는 "임계치"·"20MB"·"수십 MB"의 **구속력 있는 기본값은 본 Decision의 `20 MiB`**로 통일한다(이진 단위).

**근거**:
- "크기 초과"는 실패의 한 종류일 뿐이므로 Decision 4의 단일 fallback 경로로 흡수하는 것이 API/테스트 표면을 줄인다.
- 부분 데이터 업로드는 재생 시 깨진 이미지를 만들 수 있어 무조건 버린다.

**부분 commit 객체 처리**: 업로드 도중(예: 멀티파트 업로드 일부 파트 commit 후) 실패가 발생하여 storage에 부분 객체가 남을 수 있다. 본 change는 별도 abort/cleanup 루틴을 추가하지 않는다. 남은 부분 객체의 정리는 후속 change(`harvester-image-gc`)가 고아 객체 GC 범위에 포함하여 처리한다. 운영 중 `images/` prefix 누적량이 이상치를 보이면 GC change 우선순위를 상향한다.

### Decision 6: URL 쿼리 파라미터 정렬/제거는 도입하지 않는다

**선택**: Decision 3의 정규화는 스킴/호스트/fragment만 건드린다. 쿼리 파라미터는 이름순 정렬하지 않고, 특정 파라미터(`utm_*` 등)를 제거하지도 않는다.

**근거**:
- CDN 변환 파라미터(`?w=800`, `?format=webp`, 서명 토큰 등)가 존재할 때, 파라미터 순서/유무에 따라 **서버 응답이 달라지는** 것이 현실이다. 같은 이미지처럼 보여도 다른 바이트를 낸다.
- 따라서 쿼리가 다른 URL은 **다른 객체로 취급**한다(즉, 다른 hash → 다른 storage 키). 이는 dedup을 일부 희생하지만, 안정성(틀린 이미지 반환 방지)을 얻는다.
- 정렬/제거 규칙은 사이트마다 차이가 커서 본 change에서 도입하면 오히려 버그 원인이 된다. 필요해지면 후속 change로 분리.

### Decision 7: 본문 미디어(`media_url`)와는 별개 경로

**선택**: 기존 `downloadAndUpload`(item.MediaURL → `bot/<uuid>` prefix)는 그대로 둔다. 신설되는 image cache는 `images/<hash>/<ts>` prefix로 별도 함수에서 처리한다.

**근거**:
- 본문 미디어는 "Pin의 본체"(필수, 1차 콘텐츠)이고 대표 이미지는 "메타데이터"(부가, 실패 허용). 실패 의미가 다르므로 코드 경로도 분리.
- Prefix가 분리되어야 모니터링/GC 정책을 독립적으로 운영할 수 있다.

### Decision 8: 신규 `image_picker.go`를 두고, 기존 `internal/og` 패키지는 재사용하지 않는다

**선택**: 이미지 후보 추출은 `apps/api/internal/bot/image_picker.go`에 새로 구현한다. 기존 `apps/api/internal/og/service.go`의 `og:image`/`twitter:image` 파서를 호출하지 않는다.

**대안**:
- (A) `internal/og`의 `ParseHTML`을 호출하여 `og:image`, `twitter:image`를 얻고, 나머지(article img, JSON-LD, 유효성 검사)는 본 change에서 추가.
- (B) `internal/og`를 확장하여 article img, JSON-LD, 유효성 검사까지 수용.
- (C) 본 채택안: `internal/bot/image_picker.go` 신설, 내부에서 4단계 추출을 일체로 처리.

**근거**:
- `internal/og/service.go`는 **OG/Twitter 메타데이터 전반**(title/description/image/site_name/...)을 파싱하여 단일 `OGResult`를 만드는 범용 서비스이며, 본 change의 책임(이미지 후보 선정 + 유효성 검사 + 우선순위 fallback)과 결이 다르다.
- 본 change는 og_image 외에도 article img, JSON-LD를 포괄해야 하며 data:/1×1 픽셀 등 **이미지 고유의 유효성 규칙**을 가진다. 이는 og 메타데이터 파싱 책임과 다르다.
- (A)로 일부 소스만 위임하면 "추출은 두 곳, 유효성 검사는 한 곳"으로 경로가 갈라져 디버깅이 복잡해진다.
- (B)로 og 패키지를 확장하면 비이미지 메타데이터 소비자(다른 bot/parse 경로)에 불필요한 로직이 섞인다.
- (C)가 경계를 명확히 한다. 후속 중복 우려가 커지면 그때 공통화 change를 별도로 낸다.

### Decision 9: Content-Type ↔ 확장자 매핑과 fallback 체인

**선택**: 저장 키의 확장자는 다음 결정적 규칙으로 도출한다.

1. 응답 Content-Type이 아래 표에 있으면 대응 확장자를 사용:
   - `image/jpeg` → `.jpg`
   - `image/png` → `.png`
   - `image/webp` → `.webp`
   - `image/gif` → `.gif`
2. 매핑에 없으면 **원본 URL의 path 확장자**(소문자)로 fallback.
3. 그래도 확장자를 얻지 못하면 **`.bin`** 으로 확정.

**근거**:
- 위 4개 mime은 harvest 대상 현실 트래픽에서 압도적 비중. 외에는 빈도가 낮아 URL path fallback이 충분히 안전하다.
- `.bin`은 "확장자 없음" 대신 "식별 불가"를 명시적으로 표기한다. 후속 파이프라인에서 로그/집계가 가능.
- spec의 "저장 키는 응답 Content-Type에서 파생된 확장자를 포함"이라는 약속을 본 Decision이 구속력 있게 확정한다. tasks 2.3/2.4는 본 Decision을 참조한다.

### Decision 10: 1×1 추적 픽셀 휴리스틱

**선택**: 후보 URL이 다음 중 하나에 해당하면 추적 픽셀로 간주하여 제외한다.

- 파일명(URL path의 마지막 segment, 대소문자 무시)에 `pixel`, `1x1`, `spacer` 중 하나가 포함.
- `<img>` 요소로부터 온 후보인 경우, `width` 속성과 `height` 속성이 모두 존재하고 둘 다 `≤ 1`.

**근거**:
- 키워드 목록은 운영 중 조정이 잦은 파라미터다. spec은 "관례적 패턴"으로 추상화하고, 구체 키워드는 본 Decision으로 고정하여 구현자/리뷰어의 합의 지점을 명시화한다.
- 이미지 크기 기반 판정은 메타(og/twitter/JSON-LD)에는 적용 불가(크기 정보 없음). `<img>` 경로에만 적용.
- 키워드 추가가 필요하면 본 Decision을 확장하거나 후속 change로 분리. tasks 1.6은 본 Decision을 참조한다.

### Decision 11: article/main 내 `<img>`의 "의미 있음" 판정

**선택**: 다음 조건 중 **하나 이상**을 만족하는 `<img>`만 후보로 채택한다.

- `width` 속성과 `height` 속성이 모두 존재하고 둘 다 `≥ 100` (px),
- 또는 `alt` 속성이 존재하고 공백이 아닌 문자 1개 이상 포함.

두 조건 중 하나라도 만족하면 "의미 있음"으로 본다(OR 논리). 둘 중 어느 것도 만족하지 않으면 후보에서 제외.

**근거**:
- 아이콘/장식 img는 대체로 alt가 비어있거나 매우 작다. 위 OR 조건은 이 두 신호 모두를 받아들여 광고/장식 img를 걸러낸다.
- spec의 "의미 있는 `<img>`"를 본 Decision이 구속력 있게 확정한다. tasks 1.4는 "(design Decision 11과 일치)"로 본 Decision을 참조한다(tasks 1.4 주석을 그에 맞게 정정한다).

## Risks / Trade-offs

- **광고/추적 픽셀이 article img 단계에서 잡힐 수 있다** → 1×1 픽셀 휴리스틱과 width/height 100px 컷오프로 1차 방어. 그래도 통과한 경우는 운영 모니터링으로 감지 후 휴리스틱 보강.
- **대용량 이미지(수 MiB 이상) 다운로드로 Harvester가 느려질 수 있다** → Content-Length 검사 + read 누적 검사 양쪽으로 임계치(Decision 5의 기본값 `20 MiB`) 초과 시 fallback 경로 진입. 임계치는 환경변수화되어 운영에서 조정 가능.
- **원본 서버가 Referer/User-Agent로 hotlink 차단** → 첫 시도 실패 시 fallback이 동작하므로 spec 동작은 유지. Referer/UA 우회는 본 change 범위 외.
- **Storage 비용 누적** → `images/` prefix 단위 모니터링. 일정 임계 이상이면 `harvester-image-gc` 우선순위 상향.
- **쿼리 파라미터 보존으로 dedup 실패** → CDN 변환 URL이 많을수록 같은 원본 이미지에 대해 여러 객체가 생긴다. Decision 6의 근거처럼 안정성을 우선.
- **Object storage 장애 시 Pin 생성이 매번 fallback으로 빠짐** → 대량 Pin이 원본 URL로 저장되어 storage 복구 후 일관성 깨짐. 메트릭(가능하면)/로그로 fallback 비율을 노출하여 감지. 재캐시 잡은 후속 change.
- **`og_image` 단일 필드 정책은 "원본 URL"과 "우리 캐시 URL"이 컬럼 레벨에서 구분되지 않음** → 두 상태가 같은 컬럼에 섞이므로, 후속 분리 시 과거 row의 "어느 쪽인지"를 URL 접두(storage 도메인 매칭)로 판별해야 할 수 있다. 이 비용은 스키마 변경 없이 본 change를 가볍게 유지하는 이득으로 상쇄된다고 판단. 컬럼 분리 필요성이 생기면 `harvester-pin-document`에서 마이그레이션과 함께 도입.
- **og 패키지와 이미지 파싱 중복 가능성** → Decision 8에서 경계를 명시. 중복이 실제 유지보수 부담이 되면 공통화 change를 따로 제기한다.

## Migration Plan

1. 기존 Pin row(이미 만들어진 것)는 건드리지 않는다. 본 change는 **새로 만들어지는 Pin**부터 적용된다.
2. 코드 변경: `harvest_pipeline.go`의 Pin 생성 직전에 image cache step 추가, 추출 로직은 새 파일(`image_picker.go`)로 분리. 캐시 헬퍼는 `(string, error)` 시그니처. `Storage.Upload` 그대로 사용. DB 스키마/쿼리/sqlc 생성물 변경 없음.
3. Object storage 정책 변경 없음(만료 미설정 — storage 운영자가 필요 시 별도 결정). `images/` prefix가 신설된다.
4. 롤백: 코드 revert. 이미 `images/` prefix에 들어간 객체는 그대로 두어도 무해(아무도 읽지 않음). 추후 `harvester-image-gc`가 정리.
5. 문서 동기화(스키마 변경 없음): `docs/erd.md`의 `pin.og_image` 컬럼 설명을 본 capability 의미("캐시 성공 시 storage URL, 실패 시 원본 URL, 후보 없음 시 NULL")로 갱신. tasks 6.1과 대응.
6. 백필(과거 Pin의 `og_image` 캐시)은 본 change 범위 외.
