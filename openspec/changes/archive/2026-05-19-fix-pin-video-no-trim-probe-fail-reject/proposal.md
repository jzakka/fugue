## Why

`openspec/specs/pin/spec.md`의 Requirement `서버에서 비디오를 트리밍한다` Scenario "구간 정보 없이 15초 초과 비디오 업로드"(L203-205)는 다음을 명시한다.

- **WHEN** 트리밍 구간 정보 없이 15초 초과 비디오가 업로드되면
- **THEN** 서버가 업로드를 거부하고 트리밍 필요 에러를 반환한다

SHALL은 "거부"를 명시하며 길이 측정 실패에 대한 carve-out이 없다. 같은 Requirement의 형제 Scenario "구간 정보 없이 15초 이하 비디오 업로드"(L199-201)는 "서버가 전체 비디오를 사용한다"로, 두 Scenario가 함께 "서버가 비디오 길이를 알고 분기한다"는 가정을 만든다. 길이를 알 수 없을 때의 거동은 명시되어 있지 않으므로, 두 Scenario의 SHALL을 모두 enforce하려면 길이 미확정 케이스도 거부되어야 한다(15초 초과 거부 SHALL이 길이 측정 실패라는 우연으로 침묵 우회되는 것을 막아야 한다).

그러나 `apps/api/internal/pin/handler.go`의 `Create`는 `trim_start`/`trim_end` 폼 필드가 비어 있는 no-trim 경로에서 다음과 같이 동작한다(L241-248).

```go
} else {
    // No trim params: reject if > 15s (server defense)
    duration, probeErr := probeDuration(origTmpPath)
    if probeErr == nil && duration > float64(maxVideoDurationSeconds) {
        writeError(w, http.StatusBadRequest, "15초 초과 비디오는 트리밍이 필요합니다")
        return
    }
}
```

가드 조건이 `probeErr == nil && duration > 15`이므로 ffprobe가 실패하면(`probeErr != nil`) 거부 분기에 진입하지 않고 핸들러는 else 블록을 빠져나가 `uploadPath := origTmpPath` 그대로 `storage.Upload`로 진행한다. ffprobe 실패는 운영에서 관측 가능한 경로다: 이미지에 ffmpeg 패키지가 누락된 채 배포되면 `exec.LookPath`가 `ErrNotFound`를 반환하고, 비디오 컨테이너의 메타데이터가 malformed이면 ffprobe가 non-zero exit하며, 임시 디스크 I/O 일시 오류로 입력을 읽지 못할 수도 있다. 어느 경로든 60초 (또는 임의 길이) 비디오가 그대로 저장된다 = Scenario "구간 정보 없이 15초 초과 비디오 업로드"의 SHALL 위반.

본 change는 no-trim 경로에서 `probeErr != nil`을 fail-closed로 처리해 길이를 확인할 수 없는 비디오를 거부하도록 한다. 트림 경로(`trim_start`/`trim_end`가 전달된 경로)는 클라이언트가 자체 trim_end 가드(`trimDuration > 15.5s`, L184)로 결과물이 15초 이내임을 보장하므로 본 change 범위 밖이다.

## What Changes

- `pin.Create`의 no-trim 비디오 분기는 `probeDuration` 실패 시 거부해야 한다. 응답 코드는 기존 길이 초과 거부와 동일한 400을 유지하고, 응답 메시지는 길이를 확인할 수 없다는 사실을 사용자에게 알려야 한다.
- 길이를 확인할 수 있고 ≤ 15초인 비디오는 기존 동작 그대로 통과해야 한다(no-trim 정상 경로 유지).
- 트림 경로(`trim_start`/`trim_end` 동시 제공)는 본 change에서 변경하지 않는다. 해당 경로는 trim_end 가드로 결과물이 15초 이내임을 보장하며, probe는 trim_end 범위 검증의 보조 입력이라 실패해도 결과물 길이 SHALL을 깨지 않는다.

## Capabilities

### Modified Capabilities

- `pin`: 기존 Requirement `서버에서 비디오를 트리밍한다`에 1개 Scenario를 ADDED로 추가한다 — 길이 확인 실패 시 거부 surface를 가시화한다.

### New Capabilities

<!-- 없음. pin capability는 이미 존재한다. -->

## Impact

- 영향 코드: `apps/api/internal/pin/handler.go`(no-trim else 블록 가드 조건 1개 확장 + 신규 거부 분기 1개 추가), 신규 단위 테스트 1~2건. `internal/storage/storage.go`, `cmd/server/main.go`, 트림 경로(L172-240), spec의 다른 Scenario는 변경하지 않는다.
- 운영 지표: ffprobe가 정상 동작하는 환경에서는 동작이 변하지 않는다. ffprobe가 실패하는 환경(이미지에 ffmpeg 누락, malformed 비디오, I/O 오류)에서만 응답이 200 → 400으로 바뀐다. 본 change는 이 케이스를 silent bypass에서 명시적 거부로 바꾸는 것이 목적.
- 의존성·인프라·DB 마이그레이션 없음. Redis·S3 서버 측 변경 없음.
- 롤백: 가드 조건과 거부 분기를 직전 형태로 되돌리면 즉시 복귀(이 경우 probe 실패 silent bypass가 다시 활성화되지만 정상 경로의 가용성에는 영향 없음). 변경 전후로 라우트 외부 컨트랙트가 동일하다(추가 거부 응답 코드는 기존 400과 일관).
