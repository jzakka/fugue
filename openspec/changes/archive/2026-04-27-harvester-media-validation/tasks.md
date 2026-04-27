## 1. 미디어 유효성 검증 모듈

- [x] 1.1 미디어 후보 검증기를 harvester 패키지 하위에 신설한다 (image/video/audio 타입별 분기)
- [x] 1.2 이미지 검증: 디코더 헤더로 width/height/format 추출 + design.md D4의 임계값 비교 (in-process 디코더)
- [x] 1.3 비디오 검증: design.md D5의 probe 도구로 duration/format 추출 + D4의 임계값 비교
- [x] 1.4 오디오 검증: design.md D5의 probe 도구로 duration 추출 + D4의 임계값 비교
- [x] 1.5 검증기 단위 테스트: 1x1 GIF 거부, 정상 이미지 통과, 손상 바이트열 거부, 임계값 미만 비디오/오디오 거부

## 2. 추출 경로에 검증 통합

- [x] 2.1 PinDocument 생성 직전, `media_candidates` 후보 배열을 검증기에 통과시켜 무효 후보 제거
- [x] 2.2 `thumbnail_url` 후보도 동일 검증기 적용
- [x] 2.3 검증 결과(탈락 수, 사유 분류 카운트)를 PinDocument의 `og_data`에 관찰 가능한 형태로 기록. 구체 필드명/포맷은 구현 결정이되, spec.md "검증 실패 사유의 og_data 기록" Requirement의 최소 관찰 항목((a) 탈락 수, (b) 사유 분류별 카운트)을 외부에서 조회 가능하도록 만족해야 한다
- [x] 2.4 통합 테스트: 1x1 GIF placeholder 후보 한 개만 가진 페이지 → PinDocument의 candidates/thumbnail이 빈 상태로 구성됨
- [x] 2.5 통합 테스트: 정상 이미지 + 무효 후보 혼재 → 정상만 채택, og_data에 탈락 사유가 관찰 가능

## 3. ObjectStorage 업로드 경로

- [x] 3.1 미디어 다운로드 → 임시 버퍼/임시 키에 보관 → 검증 → 통과 시 정본 키 영속, 탈락 시 폐기 흐름으로 변경 (design.md D3)
- [x] 3.2 정본 키에 무효 미디어가 도달하지 않음을 통합 테스트로 검증
- [x] 3.3 임시 자원이 누수되지 않도록 cleanup 보장 (성공/실패 경로). Go 런타임 GC가 in-memory 임시 버퍼의 회수를 책임지므로 panic 경로에서 별도 recover 없이도 메모리 누수가 발생하지 않는다 (네트워크 응답 본문은 defer Close로 명시적으로 정리)

## 4. classifier 경로 검증

- [x] 4.1 모든 후보가 무효한 PinDocument에 대해 기존 classifier가 `no_primary_media` reason을 반환하는지 회귀 테스트
- [x] 4.2 일부 후보만 무효한 PinDocument에 대해 정상 pinnable=true 경로가 동작하는지 테스트

## 5. 메트릭/관찰성 (운영 가이드)

스펙은 메트릭을 행위 계약으로 강제하지 않는다 (design.md Migration Plan의 관찰성 권고 차원). 본 작업군은 권장 운영 항목이다.

- [x] 5.1 무효 미디어 탈락 카운터 메트릭 추가 (sum + 사유별 분기)
- [x] 5.2 `no_primary_media` 분류 비율 메트릭 노출 (변화 추세 관찰용)

## 6. 기존 placeholder Pin backfill (일회성 운영 task)

스펙의 Pin invariant는 본 변경 배포 이후 신규/갱신 Pin에만 적용된다 (design.md D7). 이 작업군은 배포 이전 누적 Pin을 점진 정상화하는 일회성 운영 절차이며, 코드 변경의 일부가 아니다.

- [x] 6.1 placeholder 패턴 식별 SQL 작성: `media_url`이 가리키는 ObjectStorage 자원 크기가 design.md D6의 placeholder 패턴 임계값 이하인 Pin 추출
- [x] 6.2 식별된 Pin들의 canonical URL을 scheduler 큐에 재투입하는 일회성 마이그레이션 스크립트 작성 (rate-limited)
- [x] 6.3 dry-run 모드로 실행하여 영향 범위(건수) 보고, 운영 승인 후 실제 실행
- [ ] 6.4 backfill 후 placeholder 패턴이 사라졌는지 동일 SQL로 재확인 (운영 단계 — backfill 실행 후 수행)

## 7. 문서/검증

- [x] 7.1 `openspec validate harvester-media-validation` 통과 확인
- [ ] 7.2 변경 후 신규 harvest의 `og_data` 검증 기록 샘플을 운영 환경에서 1주일 관찰 (운영 단계)
- [ ] 7.3 임계값 1차 보정 (메트릭 기반): 오탐이 의심되면 design.md D4의 임계값을 조정하고 재배포 (운영 단계)
