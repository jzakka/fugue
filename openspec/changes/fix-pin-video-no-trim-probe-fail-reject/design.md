## Context

`openspec/specs/pin/spec.md`의 Requirement `서버에서 비디오를 트리밍한다`는 7개 Scenario에 걸쳐 서버 측 비디오 길이 정책을 만든다(L192-217). 그중 두 형제 Scenario가 핵심이다.

- Scenario "구간 정보 없이 15초 이하 비디오 업로드"(L199-201): 길이 ≤ 15초 → 전체 비디오를 사용한다.
- Scenario "구간 정보 없이 15초 초과 비디오 업로드"(L203-205): 길이 > 15초 → 거부한다.

두 Scenario를 함께 enforce하려면 "no-trim 비디오의 길이"를 결정적으로 측정해 분기해야 한다. 현재 구현은 `ffprobe`로 측정하며, ffprobe가 실패하면 두 Scenario 중 어느 분기에도 진입하지 않는 제3의 silent passthrough 경로를 만든다. 이 경로는 길이 > 15초 비디오를 그대로 저장해 두 번째 Scenario의 SHALL을 침묵 우회한다.

본 change는 길이 측정 실패라는 우연으로 거부 SHALL이 무력화되는 것을 막는다. spec text가 "거부"만 명시하므로 fail-closed가 가장 단순한 충실 구현이다.

## Goals / Non-Goals

### Goals

- `pin.Create`의 no-trim 비디오 경로에서 `probeDuration` 실패 시 거부한다. 거부 응답은 기존 길이 초과 거부와 동일한 400을 유지하되, 메시지는 "길이 확인 불가"라는 사실을 명시한다.
- 길이가 결정적으로 측정되고 ≤ 15초인 비디오는 기존 동작 그대로 통과한다. 길이가 결정적으로 측정되고 > 15초인 비디오는 기존 메시지("15초 초과 비디오는 트리밍이 필요합니다")로 거부된다.
- 트림 경로(`trim_start`/`trim_end` 동시 제공)의 동작은 변하지 않는다.

### Non-Goals

- `probeDuration` 자체의 강건성 개선(retry, 다른 도구로의 폴백 등). 본 결함의 최소 해결은 fail-closed로 충분하다. retry/fallback은 별도 결함으로 다룰 수 있다.
- 클라이언트 측 검증 경로(`apps/web/`) 변경. 본 change는 백엔드 SHALL의 enforce에 한정.
- 트림 경로의 `probeDuration` 실패 핸들링(handler.go:182). 트림 경로는 클라이언트 trim_end 가드로 결과물이 ≤ 15초임이 보장되므로 probe 결과는 trim_end 범위 검증의 보조 입력이다. probe 실패는 trim_end > duration+1.0 검증을 우회할 뿐 결과물 길이 SHALL을 깨지 않는다(별도 카테고리 결함).
- ffprobe 부재 자체에 대한 운영 보강(부재 감지·헬스 체크). 본 change는 부재 상황에서의 안전한 거부에 한정.
- 본 change는 응답 코드를 422(Unprocessable Entity)로 바꾸지 않는다. 기존 길이 초과 거부가 400이며 클라이언트가 두 거부 경로를 구분해야 할 spec 요구가 없다.

## Decisions

### Decision 1: probe 실패 시 fail-closed (옵션 A 채택)

세 가지 옵션을 검토했다.

- **(A) `probeErr != nil` 또는 `duration > 15` → 거부** ← 채택.
  - 장점: spec SHALL("거부")의 가장 단순한 충실 구현. 길이 확인 실패가 silent passthrough를 만들 수 없다. 변경 폭은 가드 조건 1줄과 새 거부 분기 1개로 최소.
  - 단점: ffprobe가 일시적으로 실패해 유효한 ≤ 15초 비디오도 거부될 수 있다. 그러나 이는 spec SHALL 보존을 위한 의도적 trade-off이며, 클라이언트가 재시도하면 정상 경로로 복구된다.

- (B) spec text를 수정해 "probe 실패 시 거부"를 명시 SHALL로 추가하고 옵션 A 구현.
  - 장점: spec과 코드의 일치가 더 직접적.
  - 단점: spec 본문 수정은 동일 결함의 최소 해결에서 분리 가능하다. ADDED Scenario로 같은 행위 계약을 만들 수 있으며 본 change의 specs/pin/spec.md는 정확히 그 형태를 취한다(MODIFIED Requirement에 ADDED Scenario 1개 추가).

- (C) probe 실패 시 ffprobe retry 후 그래도 실패면 거부.
  - 장점: 일시적 실패에 대한 회복성.
  - 단점: retry 정책(횟수·간격·backoff)·timeout 결정이 spec에 명시되어 있지 않아 자의적 surface가 된다. 본 change의 최소 해결 범위 밖이며 별도 백로그 후보로 분리.

(A)는 본 결함의 영향 경로에 정확히 fail-closed를 부착하고, retry/fallback 정책은 미래 결정으로 분리한다.

### Decision 2: 거부 응답 코드·문구

ffprobe 실패에 대한 응답은 다음 형태로 둔다.

- 응답 코드: 400 (기존 길이 초과 거부와 동일)
- 응답 문구: "비디오 길이를 확인할 수 없습니다"

세 가지 옵션을 검토했다.

- **(A) 400 + "비디오 길이를 확인할 수 없습니다"** ← 채택.
  - 기존 길이 초과 거부 응답(handler.go:245)과 같은 400을 유지해 클라이언트 에러 분기 로직을 단순하게 둔다. 문구는 두 거부 경로를 구분 가능하게 별도로 표현해 사용자(또는 운영자)가 거부 원인을 파악할 수 있게 한다.

- (B) 500 + 일반 서버 오류 문구.
  - probe 실패는 운영 환경(ffmpeg 부재·malformed 입력)에 따른 결과로 일관된 분류가 아니다. 5xx는 클라이언트의 재시도/회피 로직을 잘못 안내한다(예: 동일 파일 재시도가 같은 결과를 낸다).

- (C) 415 Unsupported Media Type.
  - 형식이 unsupported인지 일시적 도구 부재인지를 응답 시점에서 결정하기 어렵다. 또한 클라이언트 측 Scenario "비디오 duration 감지 실패"(L160-162)와의 명명 일관성은 본 change 범위 밖.

(A)는 기존 거부 경로의 응답 코드와 일관되며, 응답 문구로 두 경로를 명확히 구분한다.

### Decision 3: 트림 경로(`trim_start`/`trim_end` 제공)는 본 change 범위 밖

`handler.go:182`의 트림 경로도 `duration, _ := probeDuration(origTmpPath)`로 같은 함수를 사용하지만 본 change에서 다루지 않는다.

- 트림 경로의 결과물 길이는 `trimDuration = trimEnd - trimStart` ≤ 15.5s 가드(L184)로 보장된다(클라이언트가 trim_end를 정직하게 전송한다는 전제 아래).
- probe는 `trimEnd > duration+1.0` 검증의 보조 입력일 뿐이며 probe 실패 시 그 검증은 우회되지만 결과물 길이 SHALL은 깨지지 않는다.
- 트림 경로의 probe 실패 핸들링은 별도 카테고리(트림 범위 검증 강건성)이며 본 결함과 분리하는 것이 변경 폭과 회귀 위험을 줄인다.

### Decision 4: 가드 조건 표현

기존 가드 `if probeErr == nil && duration > 15`을 두 갈래로 분리한다.

```go
duration, probeErr := probeDuration(origTmpPath)
if probeErr != nil {
    log.Printf("pin.Create: probe duration failed: %v", probeErr)
    writeError(w, http.StatusBadRequest, "비디오 길이를 확인할 수 없습니다")
    return
}
if duration > float64(maxVideoDurationSeconds) {
    writeError(w, http.StatusBadRequest, "15초 초과 비디오는 트리밍이 필요합니다")
    return
}
```

- 두 거부 분기를 분리하면 응답 문구가 자연스럽게 분기되며, 가드 의도가 명확해진다.
- probe 실패 로그를 한 줄 남겨 운영 측에서 ffmpeg 부재·malformed 입력을 진단할 수 있게 한다(기존 핸들러 본문이 다른 실패 경로에 `log.Printf`를 일관 사용하므로 동일 패턴).

## Risks / Trade-offs

- **R1**: 일시적 ffprobe 실패(임시 디스크 I/O 오류, 프로세스 자원 일시 고갈 등)로 유효한 ≤ 15초 비디오가 거부된다.
  - 완화: 클라이언트가 재시도하면 정상 경로로 복구된다. retry/fallback 정책은 별도 결함으로 분리하며 본 change는 spec SHALL 보존을 최우선으로 한다.
- **R2**: 운영 컨테이너 이미지에 ffmpeg이 누락되면 모든 no-trim 비디오 업로드가 거부된다.
  - 완화: 본 change 이전에는 같은 환경에서 silent passthrough로 임의 길이 비디오가 저장됐다. 본 change는 환경 결함을 가시화하는 것이 목적이며, 운영 측은 응답 메시지로 ffmpeg 부재를 빠르게 진단할 수 있다.
- **R3**: `probeDuration`이 `(0, nil)`을 반환하는 케이스(ffprobe가 0초로 측정하거나 빈 출력 파싱) — `if duration > 15`는 false라 정상 경로로 진입한다.
  - 분석: ffprobe가 0을 반환하면 비디오 길이가 0이라는 것이고 이는 storage.go의 멀티미디어 일반 처리에 위임한다. 본 change의 범위는 "probe 실패 시 fail-closed"이며 "측정값 0"은 본 결함과 별개. spec text도 "0초 비디오"에 대한 별도 SHALL을 두지 않는다.
- **R4**: 클라이언트가 다른 에러 메시지를 expect하는 케이스.
  - 완화: 기존 길이 초과 메시지("15초 초과 비디오는 트리밍이 필요합니다")는 그대로 유지되며, 신규 메시지("비디오 길이를 확인할 수 없습니다")는 새 거부 경로에만 적용된다. 클라이언트가 두 메시지를 구분 처리할 spec 요구는 없으며, 기존 메시지에 대한 클라이언트 분기 로직은 영향받지 않는다.

## Alternatives Considered

- probe 실패 시 200 + 메타데이터 누락 플래그 응답. SHALL "거부"를 만족하지 않는다.
- probe 실패 시 storage 측에서 길이를 재측정해 거부 여부 결정. 이는 본 결함을 storage 책임으로 옮기는 것이며 추상화가 흐려진다. HTTP 핸들러의 진입 가드가 spec SHALL의 자연스러운 위치.
- ffprobe 대안(예: Go 네이티브 mp4 디코더로 길이 추출). 형식별 디코더가 필요해 변경 폭이 커지며 ffprobe가 이미 dependency에 명시되어 있다(`CLAUDE.md` project: "ffprobe/ffmpeg: 비디오 업로드 시 서버 사이드 duration 검증에 필요").

## Migration Plan

1. `apps/api/internal/pin/handler.go`의 no-trim else 블록(L241-248)에서 단일 가드 `if probeErr == nil && duration > 15`를 두 갈래로 분리.
2. 분기 1: `probeErr != nil` → `log.Printf` + 400 "비디오 길이를 확인할 수 없습니다" + return.
3. 분기 2: `duration > maxVideoDurationSeconds` → 기존 400 "15초 초과 비디오는 트리밍이 필요합니다" + return.
4. 단위 테스트 추가:
   - (a) `probeDuration`이 에러를 반환하면 핸들러가 400 + "비디오 길이를 확인할 수 없습니다"로 응답.
   - (b) 길이 > 15초인 비디오는 기존 메시지로 거부됨이 유지(회귀 방지).
   - 테스트는 `probeDuration`을 wrap 가능한 surface를 만들거나, 실제 ffprobe 실패를 결정적으로 트리거할 수 있는 입력(예: `os.CreateTemp`로 만든 빈 파일)을 사용한다. 후자가 변경 폭이 더 작다.
5. `openspec validate --specs --strict`로 pin capability의 신규 Scenario가 main spec과 정합한지 검증.
6. archive 시점에 새 Scenario를 main spec의 기존 Requirement `서버에서 비디오를 트리밍한다` 안에 머지.

## Open Questions

없음. fail-closed 정책, 응답 코드·문구, 가드 분리 형태, 범위 한정 모두 본 design에서 확정.
