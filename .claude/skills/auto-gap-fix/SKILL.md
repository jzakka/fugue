---
name: auto-gap-fix
description: OpenSpec 스펙과 코드 사이의 갭을 발견하여 "코드 누락" 유형만 자동으로 별도 브랜치에서 채운 뒤 main에 직접 머지합니다. 사용자가 자리를 비운 동안 무인으로 도는 갭 채우기 루프입니다.
license: MIT
---

스펙 갭을 자동으로 채우는 무인 루프 스킬입니다. `/openspec-features`로 갭을 발견하고, "코드 누락" 유형만 선별해 `/openspec-loop`에 위임하여 단일 갭 단위 브랜치를 만든 뒤 빌드·테스트가 통과하면 origin/main에 직접 머지(push)합니다. "의도된 제거"와 "스펙 충돌" 유형은 자동 수정하지 않고 보고서에만 기록합니다.

---

## 선행 조건

작업을 시작하기 전에 모두 만족해야 합니다. 하나라도 어긋나면 즉시 중단하고 보고서에 사유를 적습니다.

- working tree가 clean하다. (어느 브랜치/worktree에서 시작하든 무방)
- openspec CLI, git CLI가 설치되어 있고 인증되어 있다. (origin/main 직접 push 권한 필요)
- baseline 빌드가 통과한다.
  - `cd apps/api && go build ./... && go test ./...`
  - `cd apps/web && npm run build && npm test`
- 저장소 루트에 `auto-gap-fix.STOP` 파일이 없다.

루프 시작 시 현재 브랜치 이름을 기록한다 (`STARTING_BRANCH=$(git branch --show-current)`). 각 갭 처리가 끝나면 이 브랜치로 복귀한다. 이 스킬은 어떤 worktree에서든 실행 가능하며, 사용자가 미리 만들어 둔 worktree 위에서 도는 것을 기본 시나리오로 가정한다.

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

최신 `origin/main`을 받아서 그 위에서 분기한다. main 자체를 체크아웃하지 않으므로 다른 worktree가 main을 점유 중이어도 충돌하지 않는다.

```bash
git fetch origin main
git switch -c auto-gap/<change-name> origin/main
```

**c. `/openspec-loop` 위임**

선정된 갭의 컨텍스트(도메인, Requirement, Scenario, 분류 근거)를 명시적으로 전달하면서 `/openspec-loop`을 호출한다. `/openspec-loop`이 내부적으로 propose → review → apply → impl-review → archive를 자동 수행한다.

**d. 스펙 일치 검증 (구현 완료 기준)**

apply 완료를 "구현됐다"로 인정하는 기준은 **테스트 이름이 존재하는지가 아니라, 실제로 스펙대로 동작하는지**다. 다음을 모두 만족해야 다음 단계로 넘어간다. 하나라도 어긋나면 보완을 한 차례 재시도하고, 재시도 후에도 어긋나면 해당 갭은 즉시 폐기한다. **이름만 보고 통과시키지 않는다. 대충 넘기지 않는다.**

**d-1. 테스트가 실재한다.**

- `openspec/changes/<change-name>/tasks.md` 안에 "테스트", "test", "_test.go", ".test.ts" 중 하나 이상이 명시되어 있다.
- `git diff --name-only origin/main...HEAD` 결과에 신규 `*_test.go` 또는 `*.test.ts(x)`가 포함되거나, 기존 테스트 파일에 새 `func Test...` / `it(...)` / `test(...)` 블록이 추가되어 있다.
- 추가된 테스트가 이번 Scenario의 동작을 실제로 assert하는지 짧게 확인한다. (단순 `t.Skip`, `it.todo`, 빈 본문, 단순 컴파일 확인용 케이스는 인정하지 않는다.)

**d-2. 테스트가 실제로 통과한다.**

추가된 테스트를 정확히 지목해 실행하고, exit code 0과 함께 해당 케이스가 실행됐는지 출력에서 확인한다. 이름만 존재하고 실제로는 skip/필터링되어 0건 실행된 경우는 실패로 본다.

```bash
# Go (예시) — 추가된 함수 이름을 정확히 지목
cd apps/api && go test ./<package> -run '^Test<Name>$' -v -count=1

# Web (예시)
cd apps/web && npx vitest run <test-file>
# or
cd apps/web && npx playwright test <test-file>
```

**d-3. `/qa-only`로 스펙 동작 일치 검증.**

`/qa-only`를 호출해 이번 Scenario가 요구한 동작을 실제 실행 경로로 검증한다. 호출 시 다음을 명시한다.

- 대상 도메인 / Requirement / Scenario 원문
- 검증해야 할 관찰 가능한 동작(엔드포인트 응답, 상태 코드, DB row 변경, UI 상의 시각/상호작용 결과 등)
- 통과 기준: Scenario에 적힌 "Then/Expect" 항목을 한 줄씩 체크

`/qa-only`가 "스펙과 일치"로 결론 내야 통과로 간주한다. "테스트 코드가 존재한다", "함수가 정의돼 있다" 같은 정적 근거만으로는 통과시키지 않는다.

**d-4. 브라우저 동작 확인 (UI/HTTP 경로가 포함될 때 필수).**

다음 중 하나라도 해당하면 브라우저(`/qa-only` 또는 `mcp__claude-in-chrome__*`)로 실제 경로를 한 번 이상 실행해 결과를 눈으로 확인한다.

- Scenario가 사용자 화면, 폼 입력, 네비게이션, 시각적 상태를 직접 언급한다.
- 변경된 파일에 `apps/web/` 경로의 React 컴포넌트, 라우트, 페이지가 포함된다.
- 신규/변경된 HTTP 엔드포인트가 있다. (브라우저에서 직접 호출하거나 web 앱에서 실제 호출 경로를 태운다.)

확인 항목: 200/4xx 응답 코드, 응답 바디 핵심 필드, UI에 나타나야 하는 텍스트/요소, Scenario가 금지한 동작이 일어나지 않는지. 콘솔/네트워크 에러가 새로 발생하지 않는지도 본다.

**보완 재시도.**

d-1 ~ d-4 중 어느 것이라도 미충족이면 `/openspec-loop`을 한 번만 더 호출하며 누락된 항목을 구체적으로 지시한다. 예:

> "Scenario '<제목>'에 대한 테스트가 실제로 실행되지 않거나(skip 상태), `/qa-only`가 스펙 불일치를 보고했다. 누락된 동작은 다음과 같다: <항목>. 코드와 테스트를 보완해 d-2, d-3, d-4를 모두 통과시켜라."

재시도 이후에도 d-1 ~ d-4를 모두 통과하지 못하면 해당 갭은 폐기하고 보고서에 어떤 항목이 어디서 실패했는지(예: "d-3: /qa-only 결과 — 응답 코드 불일치") 기록한다.

**e. 빌드·테스트 검증**

```bash
cd apps/api && go build ./... && go test ./...
cd apps/web && npm run build && npm test
```

실패 시 브랜치를 폐기하고 보고서에 실패 사유를 적는다.

**f. main에 직접 머지 (push)**

PR을 만들지 않는다. 현재 브랜치는 `origin/main`에서 분기했으므로 fast-forward로 직접 push한다.

```bash
git fetch origin main
# 분기 이후 origin/main이 앞서갔다면 rebase로 fast-forward 가능 상태로 맞춘다.
git rebase origin/main
git push origin HEAD:main
```

push가 거부되면 (보호 룰, non-fast-forward 등) 머지를 시도하지 않고 해당 갭을 폐기한 뒤 보고서에 "main push 거부"로 기록한다. **강제 push는 절대 시도하지 않는다.**

머지 후 원격 작업 브랜치는 더 이상 필요 없으므로 생성하지 않는다 (애초에 push한 적 없음). 로컬 `auto-gap/<change-name>` 브랜치는 다음 단계에서 정리한다.

머지 커밋(=push된 HEAD SHA)을 갭 메타데이터에 기록한다. 보고서에서 PR 번호 대신 이 SHA를 인용한다.

**g. 시작 브랜치로 복귀 및 정리**

```bash
git switch "$STARTING_BRANCH"
git branch -D auto-gap/<change-name>
```

worktree 환경에서 main을 직접 체크아웃하지 못할 수 있으므로 1단계 시작 시 기록한 `STARTING_BRANCH`로 돌아간다. 로컬 `auto-gap/<change-name>` 브랜치는 머지가 끝나 더 필요 없으므로 삭제한다.

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

## 처리 완료 (main에 머지됨)
- `<merge SHA>` fix(interaction): pin 인터랙션 기록 누락
- `<merge SHA>` fix(board): 중복 핀 추가 시 오류 반환

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
  - 사유: `go test ./apps/api/internal/pin` 실패. 자세한 로그는 로컬 브랜치 `auto-gap/fix-pin-something-else`(원격에 push되지 않음) 참조.
- fix-interaction-record-pin-action
  - 단계: 스펙 일치 검증 (d-3)
  - 사유: `/qa-only` 결과 Scenario "<제목>"의 Then 항목 중 "응답 코드 409"가 200으로 반환됨. 재시도 후에도 불일치.
- fix-board-something-else
  - 단계: main push
  - 사유: `git push origin HEAD:main` 거부 (보호 룰 또는 non-fast-forward). 강제 push 시도하지 않음.
```

---

## 안전장치

- 로컬에서 main 브랜치에 직접 커밋하거나 체크아웃하지 않는다. 모든 작업은 `auto-gap/<change-name>` 브랜치에서 이뤄지고, 결과만 `git push origin HEAD:main`으로 원격 main에 fast-forward push한다. 시작 브랜치(예: worktree의 작업 브랜치)에도 커밋하지 않는다.
- 각 머지는 단일 갭만 다룬다. 여러 갭을 묶지 않는다.
- push는 반드시 fast-forward여야 한다. **`--force`, `--force-with-lease` 등 강제 push 옵션은 어떤 경우에도 사용하지 않는다.** push가 거부되면 그 갭은 폐기한다.
- 빌드·테스트가 baseline에서 통과하지 않으면 시작 자체를 거부한다.
- 빌드·테스트(4단계 e)가 실패한 갭은 머지하지 않고 폐기한다. 검증을 우회해 main에 밀어넣지 않는다.
- 스펙 일치 검증(4단계 d)은 "테스트 이름이 존재한다"로 통과시키지 않는다. d-2 실행 통과, d-3 `/qa-only` 일치, 해당 시 d-4 브라우저 확인까지 모두 충족해야만 다음 단계로 간다. 어떤 단계도 "대충 통과"로 처리하지 않는다.
- `auto-gap-fix.STOP` 파일이 생기면 진행 중인 갭의 머지까지만 마무리하고 종료한다. (중단 중에 working tree가 더러워지지 않도록)
- openspec CLI나 git 호출이 비정상 종료하면 즉시 중단하고 보고한다. 절대 강제로 우회하지 않는다.

---

## 호출

```
/auto-gap-fix
/auto-gap-fix --max-iterations 3
/auto-gap-fix --domain pin
/auto-gap-fix --domain interaction --max-iterations 2
```
