## Context

Harvester는 Pin 생성 시 page에서 추출한 primary 이미지 후보를 object storage(`images/<hash>/<unix_ts>.<ext>`)에 캐시한다. 이 네임스페이스는 본문 미디어 네임스페이스(`bot/<uuid>`)와 분리되어 있다. 현재 `bot` spec은 이 네임스페이스의 lifecycle을 "capability 외부"로 명시적으로 배제하고 있고, 결과적으로 다음이 누적된다:

- 정상 캐시된 객체: Pin 생성 시마다 계속 쌓인다.
- 재캐시된 객체: 동일 후보 URL을 다른 시점에 다시 저장하면 `unix_ts`가 달라 별도 객체가 되며, 이전 객체는 참조되지 않아도 남는다.
- Pin 삭제/업데이트로 고아가 된 객체: 참조가 사라져도 storage에는 남는다.

Storage는 S3 호환(AWS S3 또는 MinIO, `internal/storage/storage.go` 참조)으로, prefix 기반 lifecycle rule을 네이티브 지원한다. 키에 `unix_ts`(초 단위)가 포함되어 있어 작성 시점은 이미 객체 자체에서 회복 가능하다.

## Goals / Non-Goals

**Goals:**
- 이미지 캐시 네임스페이스에 연령 기반 TTL을 capability 계약으로 보장한다.
- TTL 기본값과 설정 수단을 정의해 환경별 조정을 허용한다.
- Pin 생성 hot path에 영향 없이 만료가 비동기로 이루어지는 메커니즘을 선택한다.
- 기존 키 구조(`images/<hash>/<unix_ts>.<ext>`)를 유지하고, 추가 메타데이터 저장 없이 만료를 구현할 수 있도록 한다.

**Non-Goals:**
- 본문 미디어(`bot/<uuid>`) 네임스페이스의 lifecycle — 본 change 범위 밖.
- Pin 레코드에서 만료된 storage 참조를 실시간으로 감지해 재캐시하는 로직 — 후속 change.
- Pin 삭제 시 연결된 캐시 객체를 즉시 삭제하는 이벤트 기반 GC — 후속 change.
- 객체의 last-accessed 기반 eviction(LRU 등) — age-based TTL만 도입.

## Decisions

### D1. 만료 메커니즘: S3 bucket lifecycle rule 사용
S3(및 MinIO)이 제공하는 prefix 기반 lifecycle rule을 사용해 `images/` prefix의 객체를 연령 기준 만료시킨다.

- 대안: 자체 GC worker로 `ListObjects` → age 계산 → `DeleteObject` — **기각**. 새 worker/스케줄러 인프라가 필요하고, S3 lifecycle이 이미 정확히 이 문제를 해결한다. 키의 `unix_ts` 파싱도 불필요하다(lifecycle이 객체의 `LastModified`를 사용).
- 대안: 제품 코드에서 `DeleteObject`를 Pin 업데이트 이벤트로 트리거 — **기각**. Non-Goal로 명시.
- 합의 지점: bucket policy는 Terraform(`terraform/`)에서 관리. 런타임 애플리케이션 코드는 lifecycle 설정을 직접 쓰지 않는다(읽기도 하지 않는다).

### D2. TTL 기본값: 90일
- 근거: 크롤 재방문 주기 상한(`next_fetch_at = now() + 365 days`)보다 짧게 두어 재캐시 기회를 제공하되, 사용자 UX상 최신 Pin 조회 창을 여유 있게 덮는 보편적 값.
- 대안: 30일(과도한 재다운로드), 365일(축적 억제 효과 제한). 둘 다 기각.

### D3. TTL 설정 수단: 환경 변수 + 런타임 read-only 노출
- 환경 변수 이름: `HARVESTER_IMAGE_CACHE_TTL_DAYS` (기존 `HARVESTER_IMAGE_CACHE_MAX_BYTES` 명명 규칙과 대칭).
- 값 단위: 일(day). 양의 정수. 유효하지 않으면 기본값 90으로 fallback(기존 `MAX_BYTES` 파싱 실패 시 동일한 fallback 패턴 적용).
- 애플리케이션은 이 값을 **Terraform 쪽으로 전달하거나 참조용 메타데이터로 노출**할 뿐, 직접 lifecycle을 설정하지 않는다.
  - 구체 전달 방식: Terraform variable에 의미상 동일한 키(각 도구의 네이밍 관례에 따라 환경 변수는 `HARVESTER_IMAGE_CACHE_TTL_DAYS`, Terraform variable은 `harvester_image_cache_ttl_days`)를 노출하여 bucket lifecycle rule `Expiration.Days`에 주입한다. 두 쪽이 대칭되는 키를 가지는 것으로 "설정 가능성"을 관찰 가능하게 한다.
- 대안: DB 저장 — 기각(운영 설정이라 배포 단위와 수명이 묶여야 자연스럽다).

### D4. 만료 스윕과 Pin 생성의 독립성
- Pin 생성은 `HarvestPipeline.cacheImage`를 통해 동기적으로 storage에 `Upload`만 수행한다. 만료와의 상호작용은 없다.
- Pin이 저장한 참조가 만료로 404가 되어도 Harvester 측 실패 집계(생성/중복/실패 카운트)에 영향 없음. 이는 기존 spec의 "Pin 생성 시점" 관찰 기준과 일치.

### D5. 재캐시 객체의 TTL 평가 단위: per-object, `LastModified` 기준
- S3 lifecycle의 기본 기준인 `LastModified`를 그대로 사용. 키의 `unix_ts`는 lifecycle 판정 입력이 아니라 키 충돌 회피 용도로만 존재.
- 동일 후보 URL의 여러 객체(T1, T2)는 각자의 `LastModified` 기준으로 독립 만료.

### D6. Pin의 대표 이미지 참조 속성의 사후 404 허용
- Pin 소비자(웹 프런트/API)는 image 참조 해소 실패 시 이미지 영역을 비우거나 placeholder를 보여주는 기존 외부 URL 실패와 동일한 UX 경로를 따른다.
- 본 change에서는 새 코드 경로를 추가하지 않고, 소비자 측 404 허용을 **spec의 SHALL NOT block 조항으로만 계약**한다.

## Risks / Trade-offs

- **[Risk] 만료 후 사용자에게 깨진 이미지 노출** → Mitigation: TTL 기본값을 90일로 두고, 향후 change에서 Pin 소비 시점에 fallback(원본 후보 URL로 재다운로드/재캐시)을 추가할 수 있도록 spec은 "참조 해소 실패는 capability 실패가 아니다"라는 조항으로 열어 둔다.
- **[Risk] MinIO 개발 환경에서 lifecycle rule이 설정되지 않은 채 배포되어 "TTL 계약"이 운영상 실효되지 않을 수 있음** → Mitigation: Terraform으로 관리하므로 프로비저닝 시점에 rule이 보장된다. 로컬 개발(`docker-compose`)에서는 누적이 제한적이므로 명시적 lifecycle 설정을 강제하지 않는다.
- **[Trade-off] `LastModified` 기반 판정은 키의 `unix_ts`와 이론적으로 다를 수 있다(예: 객체 복사 시 `LastModified`가 갱신됨)** → 복사/재업로드 자체가 재캐시이므로 "per-object, 최신 쓰기 기준"으로 수렴하며 관찰 가능 동작과 일치한다.
- **[Trade-off] TTL 변경 시 이미 저장된 객체에 새 TTL이 소급 적용됨(S3 lifecycle은 현재 `LastModified`와 현재 rule만 본다)** → 전역 TTL만 지원하므로 관찰 가능 동작과 일치. per-object TTL은 Non-Goal.

## Migration Plan

1. **코드 배포(선행)**: 환경 변수 `HARVESTER_IMAGE_CACHE_TTL_DAYS` 파싱과 참조 메타데이터 노출을 추가한다(Terraform에 전달할 값의 단일 출처). 애플리케이션 동작에는 변화 없음.
2. **Terraform 적용**: `images/` prefix에 대한 lifecycle rule을 추가한다. 기본 90일, 환경별 override 가능. MinIO는 로컬 개발에서만 사용되므로 rule 미적용이 기본.
3. **관찰**: 첫 스윕 이후 `images/` prefix 객체 수가 감소 추세로 전환되는지 storage 사이드에서 모니터링.
4. **Rollback**: Terraform에서 lifecycle rule을 제거하면 즉시 만료가 중단된다(미래 제거만 멈추고, 이미 제거된 객체는 복구하지 않음 — 복구 불가능이지만 capability의 success 판정은 과거 시점 기준이므로 실패로 집계되지 않는다). 코드 변경은 no-op이므로 별도 롤백 불필요.

## Open Questions

- `terraform/` 모듈이 아직 생성되어 있지 않다. 본 change는 **애플리케이션 쪽 TTL 메타데이터까지만** 랜딩하고, 실제 bucket lifecycle rule 적용은 `terraform/` 모듈 도입과 함께 이뤄지는 후속 change로 분리한다. 본 change의 Go getter가 후속 change에서 Terraform variable의 값 출처가 될 수 있다.
- 후속 change가 랜딩하기 전까지 운영 환경의 `images/` prefix는 기존과 동일하게 무기한 누적된다. 본 change는 capability 계약만 "연령 기반 TTL이 정의되어야 한다"로 변경하고, 물리적 lifecycle 적용 시점은 후속 change의 스코프다.
- TTL 환경별 override 방침(dev/staging/prod 각각 다른 값을 쓸지)은 운영 결정. 기본값은 90.
