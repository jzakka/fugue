---
name: auto-gap-fix
description: OpenSpec 스펙과 코드 사이의 갭을 발견하여 "코드 누락" 유형만 자동으로 별도 브랜치/PR로 채웁니다. 사용자가 자리를 비운 동안 무인으로 도는 갭 채우기 루프입니다.
license: MIT
---

스펙 갭을 자동으로 채우는 무인 루프 스킬입니다. `/openspec-features`로 갭을 발견하고, "코드 누락" 유형만 선별해 `/openspec-loop`에 위임하여 단일 갭 단위 PR을 생성합니다. "의도된 제거"와 "스펙 충돌" 유형은 자동 수정하지 않고 보고서에만 기록합니다.

---

## 선행 조건

작업을 시작하기 전에 모두 만족해야 합니다. 하나라도 어긋나면 즉시 중단하고 보고서에 사유를 적습니다.

- 현재 브랜치가 main이고 working tree가 clean하다.
- openspec CLI, gh CLI가 설치되어 있고 인증되어 있다.
- baseline 빌드가 통과한다.
  - `cd apps/api && go build ./... && go test ./...`
  - `cd apps/web && npm run build && npm test`
- 저장소 루트에 `auto-gap-fix.STOP` 파일이 없다.

---

## 인자

- `--max-iterations N` (기본 5): 처리할 갭의 최대 개수.
- `--domain <name>` (선택): 지정한 도메인의 갭만 처리한다. 예: `pin`, `board`, `interaction`, `feed`. 값은 `openspec/specs/<name>/` 디렉토리 이름과 일치해야 한다.

인자가 없으면 기본값으로 시작하며 모든 도메인을 대상으로 한다.

---

## 루프 한 사이클

### 1단계 - 갭 발견

`/openspec-features`를 실행하고 출력을 받는다. 체크리스트에서 `[ ]`(미구현), `[~]`(부분 구현)으로 마크된 시나리오를 모두 추출한다. 각 갭에 대해 다음을 구조화하여 기록한다.

- 도메인 (pin/board/interaction/feed/...)
- Requirement 제목
- Scenario 제목
- 마크 (`[ ]` 또는 `[~]`)

### 2단계 - 분류

각 갭을 세 유형 중 하나로 분류한다.

**코드 누락** (자동 진행 대상)

다음 중 하나에 해당하면 코드 누락으로 본다.

- 시나리오가 요구하는 핸들러/함수/엔드포인트가 코드베이스에 존재하지 않는다.
- 함수는 정의되어 있으나 어디서도 호출되지 않는다. 예: `recordInteraction(pinId, "pin")` 호출처가 0건.
- 시나리오 동작이 코드에 전혀 나타나지 않는다. (스펙이 요구한 것이 부정 형태로도 존재하지 않는다)

**의도된 제거** (자동 스킵 + 보고)

다음 중 하나라도 해당하면 스킵한다.

- 갭 위치 파일 또는 인접 파일에 "removed", "deprecated", "no longer", "previously" 등 명시적 제거 주석.
- `openspec/changes/archive/` 안에 해당 기능을 제거한 change가 있다.
- git log에 해당 기능의 명시적 revert/remove 커밋이 있다.

**스펙 충돌** (자동 스킵 + 보고)

- 핸들러는 존재하지만 응답·동작이 스펙과 다르다. 예: board AddPin이 `ON CONFLICT DO NOTHING`(silent idempotent)으로 동작하나 스펙은 "중복 추가 오류 반환"을 요구.

분류 근거(주석 인용, archive change 경로, 호출처 0건 grep 결과 등)를 갭 메타데이터에 함께 기록한다.

### 3단계 - 후보 선정

코드 누락 갭만 남긴 뒤 다음 순서로 처리한다.

1. `--domain` 인자가 주어졌다면, 해당 도메인에 속하지 않는 갭은 모두 제외한다. 필터링 결과 코드 누락 갭이 0건이면 5단계로 점프한다.
2. 도메인 우선순위로 정렬: **pin > board > interaction > feed > 그 외**. `--domain`이 지정된 경우 정렬은 사실상 무의미하지만 동일 도메인 내 순서는 유지한다.
3. 동일 도메인 안에서 의존성 검사. 다음 중 하나라도 해당하면 후보에서 제외:
   - 같은 갭 리스트의 다른 코드 누락 갭에 종속된다.
   - 신규 외부 라이브러리·의존성·인프라 도입이 선행되어야 한다. (예: 오디오 WAV/FLAC 압축 변환은 ffmpeg.wasm 또는 서버 ffmpeg 파이프라인 도입 필요)
   - DB 마이그레이션이 선행되어야 한다.

상위 1건만 선택한다. 후보가 0건이면 5단계로 점프한다.

### 4단계 - 단일 갭 처리

선정된 갭 하나에 대해 다음을 순서대로 수행한다. 각 하위 단계가 실패하면 해당 갭을 폐기하고 보고서에 실패 사유를 적은 뒤 다음 사이클로 넘어간다.

**a. change 이름 결정**

`fix-<domain>-<short-slug>` 형식. 예: `fix-interaction-record-pin-action`.

**b. 새 브랜치 생성**

```bash
git switch -c auto-gap/<change-name> main
```

**c. `/openspec-loop` 위임**

선정된 갭의 컨텍스트(도메인, Requirement, Scenario, 분류 근거)를 명시적으로 전달하면서 `/openspec-loop`을 호출한다. `/openspec-loop`이 내부적으로 propose → review → apply → impl-review → archive를 자동 수행한다.

**d. 테스트 코드 강제 검증**

apply 완료 후 다음을 모두 확인한다.

- `openspec/changes/<change-name>/tasks.md` 안에 "테스트", "test", "_test.go", ".test.ts" 중 하나 이상이 명시되어 있다.
- 실제 커밋된 변경에 신규 테스트 파일 또는 신규 테스트 케이스가 포함되어 있다.
  - `git diff --name-only main...HEAD` 결과에 `*_test.go` 또는 `*.test.ts(x)` 가 존재한다.
  - 혹은 기존 테스트 파일에 새 `func Test...` 또는 `it(...)` / `test(...)` 블록이 추가되었다.

누락 시 `/openspec-loop`을 한 번 더 호출하며 다음 후속 지시를 덧붙인다.

> "테스트가 누락되었다. Scenario '<제목>'을 검증하는 테스트를 추가하라. 단위 테스트로 충분하다."

재검증해도 누락이면 해당 갭은 폐기한다.

**e. 빌드·테스트 검증**

```bash
cd apps/api && go build ./... && go test ./...
cd apps/web && npm run build && npm test
```

실패 시 브랜치를 폐기하고 보고서에 실패 사유를 적는다.

**f. PR 생성**

```bash
gh pr create --base main \
  --title "fix(<domain>): <scenario summary>" \
  --body "<...>"
```

PR 본문에 다음을 포함한다.

- 처리한 갭의 도메인 + Requirement + Scenario
- 변경된 파일 목록
- 추가된 테스트 목록
- "auto-gap-fix 스킬이 자동 생성한 PR입니다. 머지 전 사람 검토 필수." 문구

**g. main으로 복귀**

```bash
git switch main
```

### 5단계 - 종료 조건

다음 중 하나가 충족되면 루프를 종료하고 6단계로 간다.

- 코드 누락 후보가 0건이다. (`--domain`이 지정된 경우 해당 도메인 안에서 0건)
- 누적 반복 수가 `--max-iterations` 도달.
- 빌드·테스트 실패가 2회 연속 (환경 이슈 가능성).
- 저장소 루트에 `auto-gap-fix.STOP` 파일이 새로 생겼다. (원격 중단 신호)

### 6단계 - 보고서

`tmp/auto-gap-fix-report.md`에 다음 형식으로 누적 기록한다. 파일이 이미 있으면 새 실행 섹션을 위에 추가한다.

```markdown
# auto-gap-fix 실행 보고서

실행 시각: <ISO 8601>
대상 도메인: <전체 | 지정된 도메인 이름>
반복 횟수: N / max
종료 사유: <조건>

## 처리 완료
- [PR #123] fix(interaction): pin 인터랙션 기록 누락
- [PR #124] fix(board): 중복 핀 추가 시 오류 반환

## 스킵 - 의도된 제거
- pin / 오디오 WAV/FLAC 압축 변환
  - 근거: `apps/web/src/lib/media/audio.ts` 주석 "FFmpeg.wasm has been removed"
  - 사람 결정 필요: 오디오 정규화 기능을 복구할지 스펙에서 제거할지.

## 스킵 - 스펙 충돌
- board / 같은 핀 중복 추가 방지
  - 코드: `ON CONFLICT DO NOTHING` (silent idempotent, 201 응답)
  - 스펙: "중복 추가 오류 반환"
  - 사람 결정 필요: 스펙 또는 코드 중 어느 쪽이 옳은지.

## 실패
- fix-pin-something-else
  - 단계: 빌드 검증
  - 사유: `go test ./apps/api/internal/pin` 실패. 자세한 로그는 PR 브랜치 `auto-gap/fix-pin-something-else` 참조.
```

---

## 안전장치

- main에 직접 커밋하지 않는다. 모든 변경은 `auto-gap/<change-name>` 브랜치에서만 일어난다.
- PR을 자동 머지하지 않는다. 머지는 항상 사람이 한다.
- 각 PR은 단일 갭만 다룬다. 여러 갭을 묶지 않는다.
- 빌드·테스트가 baseline에서 통과하지 않으면 시작 자체를 거부한다.
- `auto-gap-fix.STOP` 파일이 생기면 진행 중인 갭의 PR 생성까지만 마무리하고 종료한다. (중단 중에 working tree가 더러워지지 않도록)
- openspec CLI나 gh CLI 호출이 비정상 종료하면 즉시 중단하고 보고한다. 절대 강제로 우회하지 않는다.

---

## 호출

```
/auto-gap-fix
/auto-gap-fix --max-iterations 3
/auto-gap-fix --domain pin
/auto-gap-fix --domain interaction --max-iterations 2
```
