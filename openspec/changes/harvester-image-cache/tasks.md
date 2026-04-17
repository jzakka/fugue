## 1. 이미지 후보 추출 모듈

- [ ] 1.1 `apps/api/internal/bot/image_picker.go` 신설: 입력으로 (HTML 바이트, 페이지 URL)을 받아 후보 URL을 반환하는 `PickPrimaryImage` 함수 시그니처 정의
- [ ] 1.2 og:image 추출 구현 (`<meta property="og:image">`)
- [ ] 1.3 twitter:image 추출 구현 (`<meta name="twitter:image">` 와 `<meta property="twitter:image">` 모두 지원)
- [ ] 1.4 article/main 내부 의미 있는 `<img>` 추출 구현 (width/height ≥ 100 또는 비어있지 않은 alt 휴리스틱)
- [ ] 1.5 JSON-LD `image` 추출 구현 (`<script type="application/ld+json">`, string | string[] | {url} 케이스 모두 평탄화)
- [ ] 1.6 후보 유효성 검사: 절대 URL resolve, http/https 스킴 체크, data: URI 거부, 1×1 추적 픽셀 패턴(`pixel`, `1x1`, `spacer`) 거부
- [ ] 1.7 우선순위(og → twitter → article img → JSON-LD) 적용 후 첫 유효 후보 반환
- [ ] 1.8 단위 테스트 `image_picker_test.go`: spec scenario(8건)을 1:1 테이블 테스트로 작성

## 2. URL 정규화 및 키 생성

- [ ] 2.1 정규화 함수 구현: 페이지 URL 기준 절대화 → fragment 제거 → host lower-case (쿼리는 보존)
- [ ] 2.2 SHA-256 hex(소문자, 64자) 해시 함수 적용
- [ ] 2.3 Content-Type → 확장자 매핑 (`image/jpeg→.jpg`, `image/png→.png`, `image/webp→.webp`, `image/gif→.gif`)
- [ ] 2.4 확장자 fallback 로직: Content-Type 매핑 실패 → URL path 확장자 → `.bin`
- [ ] 2.5 키 조립: `images/<hash>/<unix_ts>.<ext>` 포맷 함수 구현
- [ ] 2.6 단위 테스트: 정규화/키 생성에 대한 테이블 테스트

## 3. Harvest pipeline 통합

- [ ] 3.1 `apps/api/internal/bot/harvest_pipeline.go`에 `cacheImage(ctx, pageURL, html) (storageURL string, originalURL string, err error)` 헬퍼 추가 — 항상 (a) 캐시 성공 시 (storage URL, original URL, nil), (b) 후보 없음 시 ("", "", nil), (c) 캐시 실패 시 ("", original URL, err)를 명확히 구분해서 반환
- [ ] 3.2 다운로드 단계: 기존 `http.Client` 재사용, 응답 Content-Length 또는 누적 read 바이트가 임계치(기본 20MB) 초과 시 중단
- [ ] 3.3 업로드 단계: `Storage.Upload(ctx, key, contentType, size, body)` 호출, prefix는 반드시 `images/`
- [ ] 3.4 `HarvestPipeline.Process` 안에서 Pin 생성 직전에 `cacheImage` 호출
- [ ] 3.5 `CreatePinParams`에 thumbnail/og_image 값 채우기:
  - 캐시 성공 → storage URL을 OgImage(및 thumbnail 컬럼이 별도이면 동일 값)에 기록
  - 캐시 실패(원본 URL은 있음) → 원본 URL을 OgImage에 기록
  - 후보 없음 → NULL(`sql.NullString{}`) 유지
- [ ] 3.6 임계치(`ImageCacheMaxBytes`)와 활성화 플래그를 `HarvestPipeline` 생성자 옵션 또는 환경변수로 노출
- [ ] 3.7 어떤 실패도 Pin 생성을 중단시키지 않음을 보장 (try/catch 누락 없는지 리뷰)

## 4. 관찰성

- [ ] 4.1 캐시 시도/성공/실패(다운로드/업로드/크기초과) 로그 라인 추가 — Pin URL, 후보 URL, 사유 포함
- [ ] 4.2 가능한 경우 메트릭(prometheus 등 기존 사용처가 있다면) 카운터 추가: `harvester_image_cache_total{result=success|fallback_download|fallback_upload|fallback_oversize|no_candidate}`
- [ ] 4.3 `images/` prefix 객체 수/총 용량을 운영자가 조회할 수 있는 1회성 SQL 또는 스크립트 메모(별도 GC change에서 본격화)

## 5. 테스트

- [ ] 5.1 `harvest_pipeline_test.go`에 케이스 추가: 캐시 성공 → Pin.OgImage가 storage URL과 동일
- [ ] 5.2 캐시 실패(다운로드 403) → Pin.OgImage가 원본 URL, Pin은 정상 생성
- [ ] 5.3 후보 없음(빈 HTML) → Pin.OgImage NULL, Pin은 정상 생성
- [ ] 5.4 임계치 초과(20MB+) → Pin.OgImage가 원본 URL, 다운로드는 중단
- [ ] 5.5 Storage 업로드만 실패하는 mock 시나리오 → fallback 동작 확인
- [ ] 5.6 동일 URL 두 번 캐시 → 같은 hash 디렉터리, 다른 timestamp 파일명 (key 생성 함수 단위로도 검증)

## 6. 문서 및 검증

- [ ] 6.1 `docs/erd.md`의 `pin.thumbnail_url`, `pin.og_image` 설명 갱신: "캐시 성공 시 우리 storage URL, 실패 시 원본 URL, 후보 없음 시 NULL"
- [ ] 6.2 `apps/api/internal/bot/README.md`에 image cache 동작 1단락 추가 (우선순위, 키 스킴, fallback 정책)
- [ ] 6.3 `openspec validate harvester-image-cache --strict` 통과 확인
- [ ] 6.4 PR 설명에 후속 change 후보(`harvester-image-webp`, `harvester-image-gc`, `harvester-image-cdn`) 명시
