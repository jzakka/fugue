## 1. 인프라 및 설정

- [x] 1.1 (결정) 단일 bucket + `snapshots/` prefix 통합 여부 확정 (스냅샷 전용 bucket vs 기존 미디어 bucket 공유). 1.2/1.3은 본 결정에 의존하므로 1.1 완료 후 진행
- [x] 1.2 (적용) 1.1 결정 결과를 terraform/helm에 반영
- [x] 1.3 `snapshots/` prefix에 TTL 365일 lifecycle rule 추가 (365일 경과 객체 삭제)
- [x] 1.4 Pioneer 서비스의 IAM/자격 증명에 `PutObject` 권한 부여 (해당 prefix 한정)
- [x] 1.5 feature flag `PIONEER_SNAPSHOT_ENABLED` 환경변수/컨피그 도입 (기본값 off)

## 2. 스냅샷 저장 컴포넌트 구현

- [x] 2.1 `apps/api/internal/bot/` 하위에 `snapshot` 저장 인터페이스 정의 (예: `SnapshotStore.Put(ctx, normalizedURL, body) error`)
- [x] 2.2 normalized URL → **sha256** 해시 함수 구현 (`crypto/sha256` 표준 라이브러리 사용, hex 64자 소문자 출력). Pioneer/Harvester 공유 가능하도록 bot 공용 패키지에 배치
- [x] 2.3 키 빌더 구현: 상수 `SnapshotKeyPattern = "snapshots/%s/%s.html.gz"` 정의 및 공개 함수 `SnapshotKey(normalizedURL string, t time.Time) string` 제공 (UTC `yyyymmdd` 포맷). `harvester-snapshot-first-fetch`에서 동일 키 재구성에 사용
- [x] 2.4 gzip 스트림 압축 래퍼 구현 (응답 바이트 → gzip → object storage Put). 별도 checksum 검증 없이 gzip 자체 CRC 활용
- [x] 2.5 object storage 클라이언트 어댑터 구현 (S3 호환, 기존 미디어 업로드 코드 재사용 가능 시 공유)

## 3. Pioneer 통합

- [x] 3.1 Pioneer fetch 성공 경로에 `SaveRawContent` 훅 추가 (2xx + 본문 길이 > 0 조건 체크)
- [x] 3.2 feature flag off인 경우 저장 단계를 스킵하도록 분기
- [x] 3.3 업로드 실패 시 fail-open 처리: 경고 로그 + 메트릭 카운터 + Pioneer 루프 계속 진행
- [x] 3.4 fetch 실패(4xx/5xx/타임아웃/빈 본문) 시 저장 스킵 경로 확인
- [x] 3.5 URLScheduler 상태 업데이트가 스냅샷 결과와 분리되어 있음을 보장 (업로드 실패 ≠ fetch 실패)

## 4. 테스트

- [x] 4.1 단위 테스트: 키 빌더(동일 normalized URL → 동일 sha256 hex 64자, UTC 날짜 포맷, `SnapshotKeyPattern`과 일치)
- [x] 4.2 단위 테스트: gzip 압축 래퍼(원문 바이트 복원 가능, gzip CRC로 손상 검증)
- [x] 4.3 단위 테스트: fetch 실패(4xx/5xx/타임아웃/빈 본문)에서 업로드 미호출
- [x] 4.4 단위 테스트: feature flag off 상태에서 업로드 미호출 — fetch/링크 추출/스케줄러 업데이트는 정상 수행
- [x] 4.5 단위 테스트: SnapshotStore Put 실패 시 Pioneer가 링크 추출을 계속 수행
- [x] 4.6 통합 테스트(로컬 S3 mock): 2xx 수신 → 업로드된 객체 키/본문/압축 확인
- [x] 4.7 통합 테스트: **동시 쓰기 idempotent 확인** — 동일 URL을 같은 UTC 날짜에 두 번(또는 병렬로) 저장 시 동일 키에 덮어쓰기 수행, 최종 객체는 마지막 PUT 내용(last-write-wins)

## 5. 관측성

- [x] 5.1 메트릭: `pioneer_snapshot_uploads_total{result="success|failure"}` 카운터 추가
- [x] 5.2 메트릭: 업로드 지연 히스토그램 (`pioneer_snapshot_upload_duration_seconds`)
- [x] 5.3 구조화 로그: 업로드 실패 시 URL, 해시, 오류 원인 기록

## 6. 롤아웃

- [x] 6.1 스테이징 환경에 배포하고 feature flag on → 24시간 업로드 성공률/지연/용량 증가 모니터링
- [x] 6.2 운영 환경에 feature flag on으로 점진 롤아웃
- [x] 6.3 후속 change `harvester-snapshot-first-fetch`에 키 규칙/TTL/압축 포맷을 문서로 넘김
- [x] 6.4 롤백 절차 문서화: feature flag off로 업로드 중단, 저장된 스냅샷은 TTL 자연 소멸
- [x] 6.5 `harvester-snapshot-first-fetch` 배포 완료 후 `PIONEER_SNAPSHOT_ENABLED` 영구화 여부 결정(운영 toggle 유지 vs 제거)을 내리고 기록 (design.md Open Question 해소)
