## 1. 애플리케이션 쪽 TTL 설정 도입

- [x] 1.1 `apps/api/internal/bot/harvest_pipeline.go` (또는 관련 설정 파일)에 환경 변수 `HARVESTER_IMAGE_CACHE_TTL_DAYS` 파싱을 추가하고, 기본값 상수 `DefaultImageCacheTTLDays = 90`을 정의한다 (기존 `imageCacheMaxBytesEnv` 파싱 패턴 재사용)
- [x] 1.2 파싱 실패(비수치, 음수, 0) 시 기본값으로 fallback하며 `log.Printf`로 경고를 남긴다 (기존 `MAX_BYTES` fallback 로그 포맷과 대칭)
- [x] 1.3 파싱된 TTL 일수를 Terraform에 전달할 수 있도록 런타임 조회 가능한 지점(예: config 구조체 필드 또는 간단한 getter)으로 노출한다. 애플리케이션 동작 경로에서 이 값을 사용하는 분기는 추가하지 않는다 (만료는 bucket lifecycle이 담당)
- [x] 1.4 `HARVESTER_IMAGE_CACHE_TTL_DAYS` 파싱과 fallback에 대한 단위 테스트를 추가한다 (유효/음수/0/비수치/미설정 각 케이스)

## 2. Terraform lifecycle rule 추가 (후속 change로 분리 — 본 change에서는 수행하지 않음)

본 change 구현 시점에 저장소에 `terraform/` 모듈이 존재하지 않아, 실제 bucket lifecycle rule 적용은 `terraform/` 모듈 도입과 묶어 후속 change로 분리한다(design Open Questions 참조). 아래 항목은 해당 후속 change의 범위로 옮겨지며, 본 change에서는 deferred 처리한다.

- [~] 2.1 (deferred) `terraform/` 하위에서 이미지 캐시가 저장되는 S3 bucket 모듈을 특정하고, `images/` prefix에 대한 lifecycle rule을 추가할 위치를 결정한다
- [~] 2.2 (deferred) 해당 모듈에 Terraform variable `harvester_image_cache_ttl_days`(기본 90)를 추가한다
- [~] 2.3 (deferred) bucket 리소스에 prefix=`images/`, `Expiration.Days = var.harvester_image_cache_ttl_days`인 lifecycle rule을 추가한다 (본문 미디어 prefix `bot/`는 대상에서 제외)
- [~] 2.4 (deferred) `terraform plan`으로 기존 환경에 rule이 **추가**로만 적용되고 다른 prefix의 lifecycle에는 영향이 없음을 확인한다
- [~] 2.5 (deferred) 추가된 lifecycle rule이 기본 `LastModified` 기준으로 동작하며 tag/size 필터 등 per-object 만료 기준을 왜곡할 수 있는 추가 조건을 포함하지 않는지 코드 리뷰로 확인한다 (D5 대응)

## 3. 관련 문서 / 소비자 정합성

- [x] 3.1 `CLAUDE.md` / `AGENTS.md`에 본 변경과 관련된 새 운영 환경 변수 `HARVESTER_IMAGE_CACHE_TTL_DAYS`에 대한 안내가 필요한 경우 추가한다 (기존 `HARVESTER_IMAGE_CACHE_MAX_BYTES`가 안내되어 있는지 먼저 확인하고 동일 위치에 대칭적으로 추가) — 확인 결과 baseline 문서에 `HARVESTER_IMAGE_CACHE_MAX_BYTES` 등 Harvester 환경 변수가 일절 문서화되어 있지 않아, 대칭성 원칙에 따라 본 change에서도 추가하지 않음 (no-op)
- [x] 3.2 `apps/web` Pin 렌더링 경로가 이미 대표 이미지 URL 해소 실패(404/timeout 등)에 대해 우아하게 degrade하는지(placeholder 표시 또는 이미지 영역 생략) 확인하고, 그렇지 않다면 본 change의 TTL 만료 처리가 실제로 동작하기 전에 해당 fallback을 추가한다 (D6 대응) — `ImageSection`은 `og_image`를 사용하지 않으므로 영향 없음; `VideoSection`은 `og_image`를 poster로 사용하지만 `onError` 핸들러가 없어 404 시 깨진 이미지가 노출되므로 `media_url`로 swap 후 실패 시 숨기는 `onError` 핸들러를 추가함; Pin 상세 페이지는 `<video poster>`·openGraph 메타에서만 사용하며 둘 다 네이티브 degrade 경로가 있음

## 4. 사전 검증 (apply/archive 이전)

- [x] 4.1 단위 테스트: TTL 파싱/fallback 케이스 통과 확인 — `TestNewHarvestPipeline_ImageCacheTTLDays` 8 sub-cases 모두 통과 (`GOWORK=off go test ./internal/bot/`); 전체 bot 패키지 테스트도 통과
- [x] 4.2 MinIO 로컬 환경에서 `images/` prefix에 객체 업로드 후 lifecycle rule이 설정되지 않아도 Harvester가 정상 동작(기존과 동일)함을 확인한다 (로컬은 lifecycle 강제 아님) — 본 change의 Go 코드 변경은 TTL 메타데이터(상수·env 파싱·getter)만 추가하고 `cacheImage`/`Process` 경로에 분기를 넣지 않으므로 업로드 동작은 기존과 동일하게 유지됨을 코드 리뷰로 확인 (lifecycle 미설정 환경의 관찰 가능 행위는 변하지 않음)
- [~] 4.3 (deferred; 후속 Terraform change) 스테이징 환경에서 Terraform apply 후 S3 콘솔에서 `images/` prefix lifecycle rule이 TTL일수와 일치하는지 1회 육안 확인한다
- [~] 4.4 (deferred; 후속 Terraform change) 스테이징에서 TTL을 짧게(예: 1일) 임시 override하여 다음 날 prefix 객체 수 감소가 관찰되는지 확인한 뒤, 운영 기본값(90일)로 복구한다

## 5. Archive 후 검증

- [x] 5.1 `openspec archive` 실행 후 baseline `openspec/specs/bot/spec.md`에서 "이미지 캐시 객체의 TTL/만료는 본 capability 외부다" Requirement가 사라지고, 신규 "캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다" Requirement가 반영됐는지 확인한다 — 확인 완료: REMOVED 요건은 baseline에서 제거됨; ADDED 요건과 7개 시나리오 모두 baseline `openspec/specs/bot/spec.md` line 690+ 에 반영됨
