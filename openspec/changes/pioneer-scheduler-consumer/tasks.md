## 1. 선행 의존성 확인

- [ ] 1.1 `scheduler-frontier-table` change가 반영되어 `bot_frontier` 테이블과 Pioneer claim용 partial index가 존재하는지 확인
- [ ] 1.2 `scheduler-claim-api` change가 반영되어 `URLScheduler` 인터페이스(`Enqueue`/`Dequeue`/`SetStatus`)와 status 문자열 계약이 확정되었는지 확인
- [ ] 1.3 `apps/api/fuguebot_pseudo.go` Pioneer.Run(라인 33-68)을 정식 구현의 참조 모델로 재확인

## 2. Pioneer 신규 구현 (feature flag 하)

- [ ] 2.1 `apps/api/internal/bot/` 하위에 새 Pioneer 경로를 추가: `Dequeue → fetch → parse → Enqueue` 루프 본문을 구현
- [ ] 2.2 fetch 성공/실패 시 `scheduler.SetStatus(url, ...)`를 호출하도록 구현 (status 문자열은 `scheduler-claim-api` 계약을 그대로 사용)
- [ ] 2.3 링크 추출은 기존 `ExtractLinksWithSelectors`/필터 체인을 재사용하고, 결과 URL을 `scheduler.Enqueue(urls...)`로 투입
- [ ] 2.4 Pioneer 생성자에서 `URLScheduler`를 주입받도록 시그니처 변경(컴포지션 포인트)
- [ ] 2.5 feature flag(예: `BOT_PIONEER_SCHEDULER=true`)로 신규 경로를 on/off 전환 가능하게 구성

## 3. 인메모리 상태 제거 검증

- [ ] 3.1 Pioneer 신규 구현에 URL 큐/스택/슬라이스/채널 상태가 없음을 코드 리뷰 체크리스트에 반영
- [ ] 3.2 Pioneer 신규 구현에 visited 맵/사이트 카운터/세션 변수가 없음을 확인
- [ ] 3.3 Pioneer가 `bot_frontier` 테이블 컬럼을 직접 UPDATE하지 않음을 `grep`/정적 검사로 확인
- [ ] 3.4 Pioneer 코드에 분산 락/advisory lock/mutex 기반 조율 코드가 없음을 확인

## 4. 다중 워커 동작 검증

- [ ] 4.1 단일 프로세스 테스트: 신규 Pioneer가 seed URL 하나로 시작하여 Dequeue/fetch/Enqueue 사이클을 수 회 정상 반복하는지 확인
- [ ] 4.2 복수 인스턴스 테스트: 동일 scheduler에 2개 이상의 Pioneer를 띄웠을 때 동일 URL이 한 번만 처리되는지 검증 (`FOR UPDATE SKIP LOCKED` 기반 claim 재현)
- [ ] 4.3 재시작 복구 테스트: 크롤 도중 Pioneer를 kill했다가 다시 띄우면 frontier 현재 상태에서 즉시 이어받는지 확인
- [ ] 4.4 교차 사이트 Enqueue 확인: 외부 도메인 링크가 필터를 통과하면 Pioneer가 거르지 않고 `Enqueue`하는지 검증

## 5. 레거시 코드 제거

- [ ] 5.1 feature flag를 기본 on으로 전환하고 스테이징에서 안정화
- [ ] 5.2 `apps/api/internal/bot/priority_queue.go` 제거
- [ ] 5.3 `apps/api/internal/bot/bfs_queue.go` 제거
- [ ] 5.4 기존 `pioneer.go`의 BFS 본문/visited 맵/세션 카운터 제거 (생성자·공개 API는 신규 구현으로 일원화)
- [ ] 5.5 제거된 코드를 참조하던 테스트·진단 도구(`show-map` 등) 동작 재확인

## 6. 스펙/문서 정리

- [ ] 6.1 `bot` spec의 "BFS로 사이트를 탐색한다 (Pioneer)" requirement가 아카이브 시 제거되는지 검증(openspec validate)
- [ ] 6.2 `docs/architecture.md`의 Pioneer-Scheduler 관계 다이어그램/서술 갱신 (Pioneer가 scheduler consumer임을 명시)
- [ ] 6.3 AGENTS.md에 Pioneer 동작 모델 한 줄 요약 업데이트
- [ ] 6.4 아카이브: 배포 완료 후 change를 `/Users/ivanchung/fugue/openspec/changes/archive/2026-04-16-pioneer-crawl-refactor/` 하위로 이동하여 보관
