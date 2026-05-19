## Tasks

- [x] 1. `apps/api/internal/pin/handler.go`의 no-trim 비디오 경로 가드 분리
  - [x] 1.1 L241-248 else 블록의 단일 가드 `if probeErr == nil && duration > 15`를 두 갈래로 분리한다.
  - [x] 1.2 분기 1: `probeErr != nil` → `log.Printf("pin.Create: probe duration failed: %v", probeErr)` + `writeError(w, http.StatusBadRequest, "비디오 길이를 확인할 수 없습니다")` + return.
  - [x] 1.3 분기 2: `duration > float64(maxVideoDurationSeconds)` → 기존 `writeError(w, http.StatusBadRequest, "15초 초과 비디오는 트리밍이 필요합니다")` + return.
  - [x] 1.4 트림 경로(L172-240)와 `probeDuration` 정의(L701-713)는 변경하지 않는다.

- [x] 2. `apps/api/internal/pin/handler_test.go`에 회귀 방지 테스트 추가
  - [x] 2.1 `TestCreate_RejectsNoTrimVideoWhenProbeDurationFails`: `video/mp4` Content-Type 의 plain-text 본문(ffprobe 가 미디어 컨테이너로 읽지 못해 결정적으로 실패)을 multipart 로 전송, trim_start/trim_end 없음으로 no-trim 경로 진입. 응답 400, body 가 "비디오 길이를 확인할 수 없습니다" 포함, "15초 초과" 메시지로 오분류되지 않음.
  - [x] 2.2 (보류) `TestCreate_RejectsVideoLongerThan15s_PreservesMessage`: 기존 `TestDurationValidation_ThresholdLogic` 이 `duration > maxVideoDurationSeconds` 비교 로직을 임계값별로 이미 회귀 방지한다(15.0 통과, 15.001 거부 등). 메시지 문자열은 handler.go 변경에서 그대로 유지되었음을 review 단계 코드 확인으로 검증. 16 초 비디오 실파일을 결정적으로 만드는 ffmpeg 의존을 도입할 가치는 본 사이클 범위 밖.
  - [x] 2.3 테스트는 기존 `handler_test.go`의 setup(`mockQuerier`·`NewHandlerWithQuerier`·`auth.WithCreatorID`)을 재사용한다. 신규 헬퍼나 인터페이스 surface는 도입하지 않는다.

- [x] 3. `openspec/changes/fix-pin-video-no-trim-probe-fail-reject/specs/pin/spec.md`에 ADDED Scenario 작성
  - [x] 3.1 기존 Requirement `서버에서 비디오를 트리밍한다`에 1개 Scenario를 ADDED.
  - [x] 3.2 Scenario 제목: "구간 정보 없이 비디오 길이를 확인할 수 없는 경우".
  - [x] 3.3 본 Scenario는 응답 코드·문구를 명시하지 않으며("거부한다"는 surface 행위 계약만 명시), 길이 측정 도구를 명시하지 않는다(`ffprobe`는 구현 디테일).

- [x] 4. 검증·통합
  - [x] 4.1 `cd apps/api && go vet ./...` 통과.
  - [x] 4.2 `cd apps/api && go build ./...` 통과.
  - [x] 4.3 `cd apps/api && go test ./internal/pin/...` 통과(신규 테스트 포함, pin 패키지 20개).
  - [x] 4.4 `cd apps/api && go test ./...` 통과(전체 471개 회귀 없음).
  - [ ] 4.5 `openspec validate --specs --strict`로 pin capability 검증. 본 change 외 사전 드리프트는 본 change 범위 밖.
  - [ ] 4.6 archive: capability main spec(`openspec/specs/pin/spec.md`)의 Requirement `서버에서 비디오를 트리밍한다`에 신규 Scenario 머지. 기존 Scenario들의 본문·순서는 변경하지 않는다.
