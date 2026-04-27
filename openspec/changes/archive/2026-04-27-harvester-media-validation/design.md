## Context

Harvester는 외부 페이지를 fetch → parse → PinDocument 생성 → ObjectStorage(MinIO/S3) 미디어 업로드 → DB Pin 생성 흐름을 가진다. 현재 운영 환경에서 일부 페이지는 본문에 placeholder 이미지(예: 사이트가 lazy-loading용으로 사용하는 1x1 transparent GIF, 또는 외부 fetch 실패 시 fallback 처리한 결과물)를 노출한다. Harvester는 이 placeholder를 그대로 미디어 후보로 채택하여 ObjectStorage에 업로드하고 Pin의 정본 미디어 URL로 저장한다.

QA 조사 결과 핀 `a25429e1`의 `media_url`이 minio에 존재하는 1x1 GIF (`b2136cc2-...gif`, 37바이트)임이 확인되었다. 즉 placeholder가 실제 파일로 영속화되어 있다. 사용자에게는 메인 이미지가 깨진 빈 핀으로 보인다.

기존 classifier는 `media_candidates`/`thumbnail_url`이 "비어있을 때만" `no_primary_media`로 분류한다. "값이 채워져 있으나 그 값이 사실상 무효한 경우"는 식별하지 않는다. Prior learning `[fuguebot-media-crawl]`이 정확히 이 경계를 경고했었다.

본 design은 무효 미디어 후보를 외부 관찰 가능한 행위 기준으로 정의하고, classifier/업로드 경로의 책임 경계를 결정한다.

## Goals / Non-Goals

**Goals:**
- 무효 미디어 후보가 Pin의 정본 미디어로 채택되지 않도록 행위 계약을 정의한다.
- 무효 미디어 파일이 ObjectStorage에 영속화되지 않도록 한다.
- 검증 실패 사유를 관찰 가능한 형태로 기록한다 (디버깅/메트릭).
- 운영 중 누적된 placeholder 미디어 핀들에 대한 일회성 backfill 절차를 정의한다.

**Non-Goals:**
- 외부 페이지의 본문 분석 정확도 향상은 본 변경의 범위 밖이다 (extractor 개선은 별도).
- 사용자 업로드 미디어 검증 정책은 변경하지 않는다 (creator 직접 업로드 경로는 별개 정책).
- 미디어 파일 자체의 품질(저해상도/저용량) 평가는 본 변경에서는 "유효성"의 단순 임계값 검증까지만 다룬다. 고도화된 품질 평가는 후속 과제.
- "image not available" 텍스트가 그려진 PNG처럼 의미 분석이 필요한 placeholder 형태는 본 변경에서 다루지 않는다 (디코딩 가능 + 최소 크기 기준만 적용).
- backfill을 통한 재harvest로도 정상 미디어를 얻지 못한 Pin들의 정리 정책(삭제/비활성화/숨김)은 본 변경에서 결정하지 않고 후속 과제로 분리한다. 이런 Pin들은 메트릭으로 노출하여 운영자가 인지하도록 한다.

## Decisions

### D1. 무효 미디어 판정은 classifier의 `no_primary_media` 사유에 통합한다

**선택:** 새 reason enum (예: `invalid_media`)을 추가하지 않고, **기존 `no_primary_media` reason의 의미를 "primary media가 부재하거나 유효하지 않다"로 확장**한다. 후보 검증 후 살아남은 미디어가 0개이면 기존 경로 그대로 `no_primary_media`로 분류된다.

**이유:**
- classifier reason enum이 3개로 제한된다는 기존 계약(harvester spec)을 깨지 않는다.
- 외부 관찰자(메트릭/대시보드)가 보는 "primary media 부재"라는 결론은 동일하다.
- 무효 사유는 reason이 아니라 `og_data.classifier`의 디버깅 필드로 보존하면 충분하다.

**대안:** `invalid_media` reason을 추가. **기각** — enum 확장은 BREAKING이며, "사실상 부재"와 동등한 상태에 별도 reason을 도입할 실익이 작다.

### D2. 무효 미디어 식별은 "후보 단계"에서 수행한다 (PinDocument 생성 전)

**선택:** PinDocument의 `media_candidates`/`thumbnail_url`을 채우기 **전에** 각 후보를 검증하고 무효한 것은 후보 목록에서 제외한다.

**이유:**
- PinDocument는 이미 다운스트림에서 SSOT다. PinDocument에 무효 후보가 들어가지 않으면 classifier 변경 범위가 최소화된다.
- 기존 classifier 행위(빈 배열 → `no_primary_media`)가 그대로 동작한다.

**대안:** PinDocument에는 후보를 다 넣고, classifier에서 검증. **기각** — classifier가 외부 I/O(미디어 다운로드/디코딩)에 의존하게 되어 단위 테스트성이 악화된다.

### D3. 미디어 파일은 검증 통과 후에만 ObjectStorage 정본 키에 영속된다

여기서 "정본 키"란 Pin의 `media_url`이 가리키는 ObjectStorage 객체 키를 의미한다 (canonical URL과 별개의 개념).

**선택:** "다운로드 → 임시 버퍼/임시 키에 보관 → 검증 → 통과 시 정본 키로 영속, 탈락 시 폐기"의 행위 계약을 명시한다. 정본 키에는 무효 파일이 도달하지 않는다.

**이유:**
- 운영 데이터에 placeholder가 영구히 누적되는 것을 막는다.
- 정본 키 = "Pin이 참조하는 안정적인 미디어"라는 invariant를 유지한다.

**대안:** 일단 정본 키에 업로드하고 사후에 정리. **기각** — 영속 데이터에 무효 파일이 일시적으로라도 존재하는 윈도우가 생기며, 사용자에게 노출되는 시간을 0으로 만들 수 없다.

### D4. 유효성 임계값은 design.md에 위치하고 spec에는 행위 계약만 둔다

**선택:** 스펙(specs/harvester/spec.md)에는 다음만 명시한다.
- 미디어는 선언된 타입의 디코더로 디코딩 가능해야 한다.
- 미디어는 의미 있는 콘텐츠 크기 임계값을 만족해야 한다.
- 검증 실패 사유는 PinDocument의 `og_data`에 관찰 가능한 형태로 기록된다.

타입별 측정 축과 구체 임계값(픽셀 N, 재생시간 N, 바이트 N)은 본 결정 항목에 기록하되 spec에는 적지 않는다. 이렇게 하면 운영 학습으로 임계값을 조정할 때 스펙을 수정하지 않아도 된다.

**Initial threshold (구현 기본값):**

| 타입 | 측정 축 | 임계값 |
|---|---|---|
| 이미지 | 디코딩 가능 (header) | OK |
| 이미지 | 가로 픽셀 | ≥ 32 |
| 이미지 | 세로 픽셀 | ≥ 32 |
| 이미지 | 바이트 크기 | ≥ 1024 |
| 비디오 | 디코딩 가능 (probe) | OK |
| 비디오 | 재생시간 | ≥ 1초 |
| 오디오 | 디코딩 가능 (probe) | OK |
| 오디오 | 재생시간 | ≥ 3초 |

비디오/오디오는 바이트 크기 하한을 두지 않는다 (재생시간 검증으로 충분). 이미지는 디코딩만으로는 1x1 placeholder를 거를 수 없으므로 픽셀 + 바이트 두 축을 모두 적용한다.

또한 검증 단계에서 다운로드 자체가 무한히 메모리를 소비하지 않도록 **per-candidate 다운로드 상한**을 두고, 이를 초과하는 후보는 `download_failed` 사유로 탈락 처리한다. 초기 상한값은 50 MiB이며, 운영자는 검증기 인스턴스의 해당 필드를 직접 조정해 페이지 분포에 맞춰 튜닝할 수 있다. 이 상한은 행위 계약이 아니라 워커 메모리 안정성을 위한 구현 안전장치다.

**이유:** 임계값이 spec에 들어가면 튜닝마다 spec 변경이 강제된다. 행위 계약("디코딩 가능 + 의미 있는 크기")은 안정적이고, 임계값은 운영 학습으로 진화한다.

### D5. 비디오/오디오 probing은 ffprobe (subprocess)를 사용한다

**선택:** 비디오/오디오의 디코딩 가능성과 재생시간 추출에 시스템에 설치된 `ffprobe` 바이너리를 subprocess로 호출하여 사용한다.

**이유:**
- 프로젝트는 이미 ffmpeg/ffprobe를 의존성으로 갖는다 (사용자 비디오 업로드 시 서버 사이드 duration 검증; CLAUDE.md에 명시). 동일 도구를 재사용하면 의존성이 추가되지 않는다.
- ffprobe는 광범위한 컨테이너/코덱을 지원해 외부 사이트의 다양한 미디어를 안전하게 처리한다.

**대안:** 순수 Go demuxer (예: `goav`, `mp4ff`). **기각** — 코덱 커버리지가 좁고 설치/유지비가 ffmpeg 대비 높다.

**Trade-off:** subprocess 호출은 호출당 수십~수백 ms 비용이 발생할 수 있다 (D6 Risks 참조). 이미지 검증은 in-process 디코더로 처리하여 핵심 경로 비용을 낮춘다.

### D6. 기존 placeholder 핀은 "재크롤 큐 재투입"으로 backfill한다

**선택:** 일회성 마이그레이션 스크립트가 `media_url`이 알려진 placeholder 패턴(객체 크기 ≤ 256 bytes 등)을 가진 Pin들을 식별하여 해당 페이지를 scheduler 큐에 재투입한다. 재harvest 시 새 검증 로직이 적용되어 자연스럽게 (a) 정상 미디어로 갱신되거나 (b) Pin이 재생성되지 않는다.

**대안:** 즉시 삭제 또는 비활성화 플래그. **기각** — Pin idempotency 계약(canonical URL 단일 정본)을 깨거나 데이터 손실 가능성이 있다. 재크롤은 기존 idempotency 경로를 재사용한다.

**핀이 재생성되지 않는 경우 처리:** 원본 페이지가 더 이상 유효한 미디어를 제공하지 않는다는 의미. 본 변경에서는 정리하지 않고 메트릭으로 노출만 한다 (Non-Goals 참조).

### D7. backfill 임계 시점 이전 Pin은 새 invariant의 예외다

**선택:** "Pin은 항상 유효한 primary media를 참조한다"는 invariant는 **본 변경 배포 이후 새로 생성/갱신되는 Pin**에만 적용된다. 배포 이전 누적분은 backfill 진행 중 일시적으로 invariant를 위반할 수 있으며, 그 잔여 분포는 메트릭으로 추적된다.

**이유:**
- 즉시 데이터 정리는 Pin idempotency를 위협하므로 D6과 일관되게 점진 정상화한다.
- 메트릭이 잔여 비율의 신호 역할을 하여 후속 정리 정책 결정의 입력이 된다.

## Risks / Trade-offs

- **[Risk] 임계값 오탐: 정상이지만 작은 미디어가 탈락한다 (예: 정상 32x32 아이콘이 경계에 걸침)** → 임계값을 보수적으로 설정 (1x1, 1KB는 명확한 placeholder 신호). 메트릭으로 탈락률을 관찰하여 점진 조정.
- **[Risk] 검증 추가로 harvest 처리 시간 증가** → 이미지 검증은 in-process 디코더로 호출당 ms 단위. 비디오/오디오는 ffprobe subprocess로 호출당 수십~수백 ms 가능. harvest 단위 평균 +O(10ms) 수준 예상이지만 비디오 후보가 다수인 페이지는 더 클 수 있음.
- **[Risk] backfill로 인한 scheduler 큐 폭증** → backfill을 배치/rate-limited하여 점진 재투입. 일회성 운영 task로 격리.
- **[Risk] placeholder가 1x1 GIF 외 다른 형태로 존재할 수 있다 (예: "image not available" 텍스트 PNG)** → 본 변경은 "디코딩 가능 + 최소 크기" 기준만 다룬다 (Non-Goals 참조). 메트릭으로 추가 패턴 발견 시 후속 과제로.
- **[Trade-off] reason enum을 확장하지 않으므로 메트릭 차원에서 "비어있어서 탈락"과 "무효라서 탈락"이 reason 레벨에서는 구분되지 않는다.** `og_data`의 검증 기록 필드로 구분 가능하나 집계가 한 단계 더 필요. 운영 단순성을 우선했다.

## Migration Plan

1. **변경 배포**: 새 검증 로직이 켜진 harvester를 배포한다. 신규 harvest는 자동으로 새 행위를 따른다.
2. **메트릭 관찰**: 무효 미디어 탈락률, classifier `no_primary_media` 비율을 1주일 관찰한다. 임계값 오탐이 의심되면 D4 임계값을 조정한다.
3. **Backfill 실행**: placeholder media를 가진 기존 Pin을 식별하여 scheduler에 재투입한다. 처리는 배치로 수행한다.
4. **Rollback**: 새 검증 로직은 코드 revert 한 번으로 비활성화 가능하다 — 검증 호출을 우회하면 기존 행위로 즉시 복귀한다 (단, 그 동안 backfill로 재harvest된 Pin은 유지).

## Open Questions

(현재 Open Questions 없음. 후속 과제 후보는 Non-Goals 참조.)
