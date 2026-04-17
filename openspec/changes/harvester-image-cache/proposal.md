## Why

Harvester는 Pin 문서를 만들 때 외부 페이지의 thumbnail/media 이미지 URL을 그대로 보관할 뿐, 이미지 바이트 자체를 저장하지 않는다. 그 결과 (1) 원본 사이트가 hotlink 차단/리다이렉트/도메인 만료/이미지 교체를 하면 피드의 썸네일이 깨지고, (2) 추천/상세/그리드에서 이미지를 노출할 때마다 외부 origin에 트래픽이 발생하여 원본 서버에 부담을 주는 동시에 우리 사용자 경험도 외부 응답 지연에 종속된다. Pin 문서가 만들어지는 그 순간(Harvester가 한 번 fetch한 시점)에 primary 이미지를 우리 object storage로 한 번만 가져오면, 이후 재사용 비용 0으로 안정적인 썸네일/상세 이미지를 제공할 수 있다.

## What Changes

- Harvester가 Pin 문서를 생성하는 경로에 **primary 이미지 캐싱 단계**를 추가한다. Pin 1건당 정확히 1개의 primary 이미지(썸네일/대표 미디어)를 object storage에 저장한다.
- **이미지 추출 우선순위**를 정의: `og:image` → `twitter:image` → 본문(article) 내 의미 있는 `<img>` → JSON-LD `image`. 첫 번째로 유효한 후보를 채택한다.
- **저장 키 스킴**: `images/<hash>/<timestamp>.<ext>`. `hash`는 원본 이미지 URL 정규화 후 SHA-256, `ext`는 응답 Content-Type / 원본 URL에서 도출.
- **저장 정책**: 원본 바이트를 그대로 저장한다. WebP 변환은 본 change에서는 도입하지 않고 후속 change(`harvester-image-webp`)에서 다룬다(open question 참고).
- **TTL/생명주기**: object는 사실상 영구 보관(만료 정책 없음). 별도 GC는 후속 change(`harvester-image-gc`)에서 다룬다.
- **저장 실패 시 fallback**: 이미지 다운로드 또는 업로드가 실패하면 Pin 생성을 실패시키지 않는다. 캐시된 URL 대신 **원본 URL을 그대로** Pin의 thumbnail/og_image 컬럼에 기록하고, 실패는 로그/메트릭으로만 남긴다.
- ERD 관점에서 `pin.thumbnail_url`, `pin.og_image`의 **의미를 명확화**: "캐시 성공 시 우리 storage URL, 실패 시 원본 URL". 컬럼 추가는 하지 않는다.

## Capabilities

### New Capabilities
- (없음)

### Modified Capabilities
- `bot`: Harvester 동작에 "primary 이미지를 object storage에 캐시한다"는 requirement를 ADDED. 이미지 추출 우선순위, 저장 키 규칙, 실패 시 원본 URL fallback, TTL(영구 보관) 규칙을 신규 requirement로 추가한다. 기존 requirement는 변경/제거하지 않는다.

## Impact

- **코드**: `apps/api/internal/bot/harvest_pipeline.go`의 Pin 생성 경로에 이미지 캐싱 step 추가. 추출 우선순위 로직은 별도 helper(예: `image_picker.go`)로 분리. `Storage` 인터페이스는 그대로 사용(이미 `Upload(ctx, key, contentType, size, body)` 형태).
- **DB**: 스키마 변경 없음. `pin.thumbnail_url`, `pin.og_image`에 들어가는 값의 의미만 명확해진다.
- **Object storage**: `images/` prefix 신설. 기존 `bot/<uuid>` prefix(미디어 본체)와 분리하여 GC/모니터링을 독립적으로 운영 가능.
- **운영**: 외부 origin 호출 횟수 감소, 썸네일 안정성 증가. 동시에 storage 비용(이미지 누적)이 발생 — 후속 GC change에서 다룬다.
- **의존성**: 이미지 추출은 표준 라이브러리(`golang.org/x/net/html`) 또는 기존 파서로 처리 가능. WebP 변환은 본 change 범위 외이므로 추가 의존성 없음.
- **스펙**: `bot` capability에 image cache 관련 requirement 4건 ADDED.
- **후속 change 후보**: `harvester-image-webp`(WebP 변환), `harvester-image-gc`(고아 이미지 GC), `harvester-image-cdn`(CDN 도메인 분리).
