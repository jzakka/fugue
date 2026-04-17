## 1. 인프라 및 설정

- [ ] 1.1 스냅샷 전용 bucket(또는 기존 미디어 bucket + `snapshots/` prefix) 결정 및 terraform/helm 반영
- [ ] 1.2 `snapshots/` prefix에 TTL 365일 lifecycle rule 추가 (365일 경과 객체 삭제)
- [ ] 1.3 Pioneer 서비스의 IAM/자격 증명에 `PutObject` 권한 부여 (해당 prefix 한정)
- [ ] 1.4 feature flag `PIONEER_SNAPSHOT_ENABLED` 환경변수/컨피그 도입 (기본값 off)

## 2. 스냅샷 저장 컴포넌트 구현

- [ ] 2.1 `apps/api/internal/bot/` 하위에 `snapshot` 저장 인터페이스 정의 (예: `SnapshotStore.Put(ctx, normalizedURL, body) error`)
- [ ] 2.2 normalized URL → 해시 함수 구현 (Pioneer/Harvester 공유 가능하도록 bot 공용 패키지에 배치)
- [ ] 2.3 키 빌더 구현: `snapshots/<hash>/<UTC yyyymmdd>.html.gz` 생성 함수
- [ ] 2.4 gzip 스트림 압축 래퍼 구현 (응답 바이트 → gzip → object storage Put)
- [ ] 2.5 object storage 클라이언트 어댑터 구현 (S3 호환, 기존 미디어 업로드 코드 재사용 가능 시 공유)

## 3. Pioneer 통합

- [ ] 3.1 Pioneer fetch 성공 경로에 `SaveRawContent` 훅 추가 (2xx + 본문 길이 > 0 조건 체크)
- [ ] 3.2 feature flag off인 경우 저장 단계를 스킵하도록 분기
- [ ] 3.3 업로드 실패 시 fail-open 처리: 경고 로그 + 메트릭 카운터 + Pioneer 루프 계속 진행
- [ ] 3.4 fetch 실패(4xx/5xx/타임아웃/빈 본문) 시 저장 스킵 경로 확인
- [ ] 3.5 URLScheduler 상태 업데이트가 스냅샷 결과와 분리되어 있음을 보장 (업로드 실패 ≠ fetch 실패)

## 4. 테스트

- [ ] 4.1 단위 테스트: 키 빌더(동일 URL → 동일 해시, UTC 날짜 포맷)
- [ ] 4.2 단위 테스트: gzip 압축 래퍼(원문 바이트 복원 가능)
- [ ] 4.3 단위 테스트: fetch 실패(4xx/5xx/타임아웃/빈 본문)에서 업로드 미호출
- [ ] 4.4 단위 테스트: SnapshotStore Put 실패 시 Pioneer가 링크 추출을 계속 수행
- [ ] 4.5 통합 테스트(로컬 S3 mock): 2xx 수신 → 업로드된 객체 키/본문/압축 확인
- [ ] 4.6 통합 테스트: 같은 날 같은 URL 두 번 fetch 시 동일 키로 덮어쓰기 확인

## 5. 관측성

- [ ] 5.1 메트릭: `pioneer_snapshot_uploads_total{result="success|failure"}` 카운터 추가
- [ ] 5.2 메트릭: 업로드 지연 히스토그램 (`pioneer_snapshot_upload_duration_seconds`)
- [ ] 5.3 구조화 로그: 업로드 실패 시 URL, 해시, 오류 원인 기록

## 6. 롤아웃

- [ ] 6.1 스테이징 환경에 배포하고 feature flag on → 24시간 업로드 성공률/지연/용량 증가 모니터링
- [ ] 6.2 운영 환경에 feature flag on으로 점진 롤아웃
- [ ] 6.3 후속 change `harvester-snapshot-first-fetch`에 키 규칙/TTL/압축 포맷을 문서로 넘김
- [ ] 6.4 롤백 절차 문서화: feature flag off로 업로드 중단, 저장된 스냅샷은 TTL 자연 소멸
