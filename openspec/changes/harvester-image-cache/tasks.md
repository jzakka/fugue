## 1. 이미지 후보 추출 모듈

- [x] 1.1 `apps/api/internal/bot/image_picker.go` 신설: 입력으로 (HTML 바이트, 페이지 URL)을 받아 후보 URL을 반환하는 `PickPrimaryImage` 함수 시그니처 정의
- [x] 1.2 og:image 추출 구현 (`<meta property="og:image">`)
- [x] 1.3 twitter:image 추출 구현 (`<meta name="twitter:image">` 와 `<meta property="twitter:image">` 모두 지원)
- [x] 1.4 article/main 내부 의미 있는 `<img>` 추출 구현 (width/height ≥ 100 또는 비어있지 않은 alt 휴리스틱)
- [x] 1.5 JSON-LD `image` 추출 구현 (`<script type="application/ld+json">`, string | string[] | {url} 케이스 모두 평탄화)
- [x] 1.6 후보 유효성 검사: 절대 URL resolve, http/https 스킴 체크, data: URI 거부, 1×1 추적 픽셀 패턴(`pixel`, `1x1`, `spacer`) 거부
- [x] 1.7 우선순위(og → twitter → article img → JSON-LD) 적용 후 첫 유효 후보 반환
- [x] 1.8 단위 테스트 `image_picker_test.go`: spec scenario(기본 성공/우선순위/픽셀 필터 등)를 1:1 테이블 테스트로 작성

## 2. URL 정규화 및 키 생성

- [x] 2.1 정규화 함수 구현: 페이지 URL 기준 절대화 → fragment 제거 → host lower-case. **쿼리 파라미터는 보존**(CDN 서명/변환 파라미터 고려). 쿼리 정렬/제거는 도입하지 않는다
- [x] 2.2 SHA-256 hex(소문자, 64자) 해시 함수 적용
- [x] 2.3 Content-Type → 확장자 매핑 (`image/jpeg→.jpg`, `image/png→.png`, `image/webp→.webp`, `image/gif→.gif`, 그 외 → `.bin`)
- [x] 2.4 확장자 fallback 로직: Content-Type 매핑 실패 → URL path 확장자 → `.bin`
- [x] 2.5 키 조립: `images/<hash>/<unix_ts>.<ext>` 포맷 함수 구현
- [x] 2.6 단위 테스트: 정규화/키 생성에 대한 기본 테이블 테스트(정규화 edge case는 후속 change)

## 3. Harvest pipeline 통합

- [x] 3.1 `apps/api/internal/bot/harvest_pipeline.go`에 `cacheImage(ctx, url) (string, error)` 헬퍼 추가 — **성공 시 storage URL을 반환**, **실패 시 입력된 원본 URL을 반환**. `error`는 실패 원인(로그 목적). 호출자는 항상 반환 `string`을 그대로 Pin 컬럼에 기록한다
- [x] 3.2 다운로드 단계: 기존 `http.Client` 재사용, 응답 Content-Length 또는 누적 read 바이트가 임계치(기본 20MB) 초과 시 중단하고 원본 URL fallback 경로 진입(부분 데이터는 버린다)
- [x] 3.3 업로드 단계: `Storage.Upload(ctx, key, contentType, size, body)` 호출, prefix는 반드시 `images/`
- [x] 3.4 `HarvestPipeline.Process` 안에서 Pin 생성 직전에 후보 추출 → 후보 존재 시 `cacheImage` 호출
- [x] 3.5 `CreatePinParams` 채우기: 후보 존재 시 `cacheImage` 결과 문자열을 **`OgImage`와 `ThumbnailURL`(혹은 thumbnail 컬럼)에 동일 값으로** 기록. 후보 없음 → 두 컬럼 모두 NULL(`sql.NullString{}`)
- [x] 3.6 임계치(`ImageCacheMaxBytes`)와 활성화 플래그를 `HarvestPipeline` 생성자 옵션 또는 환경변수로 노출
- [x] 3.7 어떤 실패(다운로드/업로드/크기 초과)도 Pin 생성을 중단시키지 않음을 보장 — `cacheImage`가 항상 err 대신 반환 string으로 fallback을 표현하므로 호출자는 분기 없이 결과를 기록

## 4. 관찰성

- [x] 4.1 캐시 시도/성공/실패(다운로드/업로드/크기초과) 로그 라인 추가 — Pin URL, 후보 URL, 사유 포함. `cacheImage`가 반환한 `error`는 여기에서 로그로만 소비한다
- [x] 4.2 **선택적**: 기존 metrics 인프라(prometheus 등)가 있으면 카운터 `harvester_image_cache_total{result=success|fallback_download|fallback_upload|fallback_oversize|no_candidate}` 추가. 없으면 로그만으로 충분(본 change에서 신규 메트릭 인프라를 만들지 않는다)
- [x] 4.3 `images/` prefix 객체 수/총 용량 조회 스크립트는 본 change 범위 외(후속 GC change)

## 5. 테스트

본 change는 **기본 케이스만** 다룬다. edge case(URL 정규화 실패, SHA-256 hash 충돌, unix 초 해상도 충돌 등)는 **본 change 범위 외**이며 후속 change에서 다룬다.

- [x] 5.1 `harvest_pipeline_test.go`에 케이스 추가: 캐시 성공 → `Pin.OgImage`와 `Pin.ThumbnailURL`이 **동일한 storage URL**
- [x] 5.2 캐시 실패(다운로드 403) → 두 컬럼이 **동일한 원본 URL**, Pin은 정상 생성
- [x] 5.3 후보 없음(빈 HTML) → 두 컬럼 모두 NULL, Pin은 정상 생성
- [x] 5.4 (필요 시) 임계치 초과 mock → 두 컬럼이 원본 URL로 fallback (Decision 5 동작 검증)
- [x] 5.5 (필요 시) Storage 업로드 실패 mock → fallback 동작 확인

## 6. 문서 및 검증

- [x] 6.1 `docs/erd.md`의 `pin.thumbnail_url`, `pin.og_image` 설명 갱신: "캐시 성공 시 storage URL, 실패 시 원본 URL, 후보 없음 시 NULL — **두 컬럼은 본 capability에서 동일 값**"
- [x] 6.2 `apps/api/internal/bot/README.md`에 image cache 동작 1단락 추가 (우선순위, 키 스킴, fallback 정책, 두 컬럼 동일값 기록)
- [x] 6.3 `openspec validate harvester-image-cache --strict` 통과 확인
- [x] 6.4 PR 설명에 후속 change 후보(`harvester-image-webp`, `harvester-image-gc`, `harvester-image-cdn`, `harvester-image-cache-edge-cases`) 명시
