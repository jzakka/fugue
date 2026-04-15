## 1. 인터페이스 변경 및 의존성

- [x] 1.1 `harvester.go`의 `Pipeline` 인터페이스 정의와 기존 3개 반환값 `(pinsCreated int, deduped int, error error)`에 `failed int`를 추가하여 `(pinsCreated int, deduped int, failed int, err error)` 4개 반환값으로 확장. MockPipeline의 `ProcessFunc` 타입, `Process` 메서드, struct 필드(`TotalFailed` 추가)를 모두 업데이트. MockPipeline의 기본 ProcessFunc에서 failed=0을 반환하도록 설정. 기존 `harvester.go`의 `h.pipeline.Process` 호출부도 4개 반환값으로 수정(4번째 반환값은 `_`로 무시, 통계 누적은 task 4.1에서 수행). 기존 테스트(`harvester_test.go`)의 MockPipeline 사용부(시그니처 변경에 따른 호출부) 업데이트. `README.md`의 Pipeline 인터페이스 시그니처도 4개 반환값으로 업데이트. 참고: task 4.1에서 같은 파일을 다시 수정하므로 전체 테스트 컴파일 확인은 task 4.1 완료 후 수행
- [x] 1.2 미디어 업로드를 위한 Storage 인터페이스 정의 + mock Storage 구현체 작성. 기존 `storage.Client`가 인터페이스를 만족하는지 확인하고 필요하면 어댑터 작성
- [x] 1.3 goja 의존성 추가 (`go get github.com/dop251/goja`)
- [x] 1.4 GojaExecutor 생성자에 타임아웃 파라미터 추가. CLI에서 환경변수 또는 플래그로 값을 읽어 전달 (0 이하이면 기본값 10000ms)

## 2. GojaExecutor 구현

- [x] 2.1 GojaExecutor 구조체 생성 (ScriptExecutor 인터페이스 구현, `apps/api/internal/bot/goja_executor.go`)
- [x] 2.2 DOM 헬퍼 주입 구현 — goquery로 HTML 파싱 후 `document.querySelectorAll`, `querySelector`, `textContent`, `getAttribute` 를 goja 런타임에 등록. goroutine + context Done 채널 감시 + goja `Interrupt()` 방식으로 타임아웃(기본 10초) 적용
- [x] 2.3 스크립트 반환값을 RawItem 배열로 변환하는 로직 구현 (필수 필드 title/mediaURL/mediaType 검증, 선택 필드 description/sourceURL 누락 시 정상 처리, sourceURL 빈 문자열 시 현재 노드 URL로 채우기, undefined/null 반환값 처리 포함)
- [x] 2.4 GojaExecutor 단위 테스트 — 정상 실행, 구문 에러, 런타임 에러, 빈 HTML, 필수 필드 누락, 타임아웃 케이스

## 3. HarvestPipeline 구현

- [x] 3.1 기존 `seed.sql`의 봇 계정(`00000000-0000-0000-0000-00000000f096`, `fuguebot`)이 존재하는지 확인. HarvestPipeline에서 참조할 봇 계정 UUID 상수 정의
- [x] 3.2 sqlc 쿼리 파일 작성 — 봇 중복 체크용 SELECT (`WHERE url = $1 AND creator_id = $2`), Pin INSERT 쿼리. 기존 `PinURLExists` 쿼리(`WHERE url = $1`, creator_id 조건 없음)의 사용처를 확인하고 유지/수정/삭제 결정. 현재 `idx_pins_url` 단일 컬럼 인덱스는 봇 Pin만의 중복 체크 용도로 충분(url로 필터링 후 소수의 행에서 creator_id 비교). 데이터 증가 시 복합 인덱스는 별도 변경으로 검토
- [x] 3.3 sqlc generate 실행하여 Go 코드 생성
- [x] 3.4 HarvestPipeline 구조체 생성 (Pipeline 인터페이스 구현, `apps/api/internal/bot/harvest_pipeline.go`) — Storage 인터페이스, DB queries 의존성 주입
- [x] 3.5 sourceURL + 봇 creator_id 기반 중복 체크 구현 + 배치 내 중복 체크
- [x] 3.6 미디어 다운로드 및 S3 업로드 구현 — mediaURL에서 다운로드 후 Storage 인터페이스로 업로드
- [x] 3.7 Pin 생성 구현 — sqlc 쿼리를 사용하여 pins 레코드 생성 (봇 계정 고정 UUID 상수 사용, og_image/og_data는 NULL로 설정)
- [x] 3.8 HarvestPipeline 단위 테스트 — 신규/중복(봇 Pin만)/다운로드·업로드 실패/DB 에러 케이스 (mock Storage 사용)

## 4. Harvester 통계 수집 및 CLI 연결

- [x] 4.1 Harvester.Run 반환값을 통계 struct + error로 확장 — harvester.go의 Pipeline.Process 호출부에 노드별 통계 누적 로직 추가. 기존 `harvester_test.go`의 `Run` 호출부를 `(stats, err)` 2개 반환값으로 업데이트 (task 1.1이 Process 시그니처 변경, 이 task는 Run 반환값 변경). 참고: task 1.1과 이 task가 같은 테스트 파일을 수정하므로, 전체 테스트 컴파일 확인은 이 task 완료 후 수행
- [x] 4.2 `cmd/bot/main.go`의 harvesterCmd에서 `HARVESTER_MODE=real` 분기 추가 — GojaExecutor 생성 시 타임아웃 파라미터 전달 + HarvestPipeline 생성 (기존 infra.Storage를 Storage 어댑터로 연결, DB queries 포함)
- [x] 4.3 Harvester.Run 반환 통계를 로그로 출력 (노드 수, Pin 생성 수, 중복 수, 실패 수)

## 5. 통합 검증

- [ ] 5.1 로컬 환경에서 end-to-end 동작 확인 — 사전 조건: docker-compose 실행(PostgreSQL + MinIO), DB 마이그레이션 적용, Pioneer로 pixiv 데이터 수집 완료. `HARVESTER_MODE=real go run ./cmd/bot/main.go harvester pixiv` 실행
- [ ] 5.2 생성된 Pin 데이터와 S3 미디어 파일 검증
