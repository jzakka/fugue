---
name: openspec-impl-review
description: Review code implementation against an OpenSpec change. Verifies that code changes match the design, tasks are completed correctly, and tests pass. Use after implementing an OpenSpec change and before committing.
license: MIT
compatibility: Requires openspec directory structure and git.
metadata:
  author: openspec
  version: "1.0"
---

OpenSpec change의 설계 대비 구현 코드를 리뷰한다.

**Input**: change 이름. 없으면 질문한다.

---

## Steps

### 1. OpenSpec 문서 읽기

`openspec/changes/<name>/` 디렉토리의 모든 문서를 읽는다:
- `design.md` — Decisions, 코드 예시, 구현 패턴
- `tasks.md` — 체크리스트
- `proposal.md` — 변경 범위
- `specs/**/spec.md` — 요구사항 시나리오

### 2. 코드 변경 확인

```bash
git diff --stat
git diff
git status -s
```

미커밋 변경과 새 파일을 모두 확인한다. 커밋 완료된 변경이면 최근 커밋의 diff를 확인한다.

### 3. 변경된 파일 전체 읽기

diff만으로는 맥락이 부족하다. 변경된 파일과 새로 생성된 파일을 모두 전체 읽어서 코드의 완전한 상태를 파악한다.

### 4. Tasks 체크리스트 대비 검증

tasks.md의 각 항목을 하나씩 대조:

| task | 구현 파일:행 | 상태 |
|------|------------|------|
| 1.1 SQL 쿼리 추가 | db/queries/pins.sql:XX | 일치/불일치/누락 |
| ... | ... | ... |

### 5. Design Decisions 대비 검증

design.md의 각 Decision이 코드에 정확히 반영되었는지 확인:

- SQL 쿼리와 실제 구현이 일치하는가
- 대안으로 기각된 방식이 구현에 사용되지 않았는가
- Go 인터페이스 시그니처, 핸들러 구조, 라우트 경로가 설계와 일치하는가
- 프론트엔드 컴포넌트 구조, URL 파라미터 패턴이 설계와 일치하는가
- 에러 처리 방식이 설계와 일치하는가

### 6. Spec 시나리오 대비 검증

spec.md의 각 시나리오가 코드로 충족되는지 확인:

- WHEN 조건이 코드에서 처리되는가
- THEN 결과가 코드의 실제 동작과 일치하는가
- 누락된 시나리오가 없는가

### 7. 코드 품질 검증

설계 문서와 무관하게 코드 자체의 품질을 확인:

- 고루틴/동시성 안전성
- 리소스 정리 (DB rows.Close, HTTP body close)
- 에러 처리 누락 (Go의 err 체크)
- 네이밍 일관성
- 불필요한 코드 (import, 변수, 함수)

### 8. 테스트 확인

- 새로 추가된 테스트 파일 읽기
- 테스트가 design.md의 동작을 검증하는가
- edge case가 커버되는가
- 기존 테스트가 새 구현에 맞게 업데이트되었는가
- `go test ./...` (백엔드) 또는 `npm run test` (프론트엔드) 실행하여 통과 확인

### 9. 결과 보고

```
## Tasks 체크리스트

| task | 상태 |
|------|------|
| 1.1 | 반영 |
| ... | ... |

## Design 대비 검증

| Decision | 상태 |
|----------|------|
| Decision 1 | 일치 |
| ... | ... |

## 이슈

| 우선순위 | # | 항목 |
|---------|---|------|
| ... | ... | ... |
```

이슈가 없으면 "지적 사항 없습니다. 커밋해도 좋습니다." 출력.

---

## Guardrails

- **읽기 전용** — 코드를 수정하지 않는다. 리뷰 결과만 보고한다. (테스트 실행은 예외)
- **전체 파일 읽기** — diff만 보지 않는다. 변경된 파일의 전체 맥락을 확인한다.
- **설계 문서가 기준** — 코드가 "좋은 코드"인지가 아니라 "설계대로 구현되었는지"가 핵심이다. 설계에 없는 개선 제안은 하지 않는다.
- **테스트 반드시 실행** — 테스트를 직접 실행하여 통과 여부를 확인한다. "통과할 것으로 보인다"가 아니라 실제 결과를 보고한다.
- **거짓 긍정 방지** — 확실하지 않은 이슈는 "확인 필요"로 표시한다.
