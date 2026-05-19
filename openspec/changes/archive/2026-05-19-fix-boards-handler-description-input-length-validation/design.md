# Design: boards `description` 입력 길이 검증

## D1. 검증 위치 — handler 진입부 인라인 vs 미들웨어/라이브러리

**선택**: handler 진입부 인라인. `name` 길이 검증이 이미 같은 핸들러의 같은 위치(L110, L287)에 인라인으로 존재한다. description은 그 패턴과 정렬되어 같은 함수 내에서 추가된다. 별도 validation 미들웨어나 struct tag 기반 validator(`go-playground/validator` 등)를 도입하면 `name` 검증과의 패턴 분기가 생겨 신규 컨트리뷰터가 두 가지 패턴을 모두 학습해야 한다 — 본 change 범위에서는 의도적으로 회피한다. validator 라이브러리 도입은 별개 design decision으로 분리되어야 한다.

## D2. 정책 — 400 reject vs server-side truncate

**선택**: 400 reject. 같은 핸들러의 `name` 정책(L111 "보드 이름은 100자 이내여야 합니다" → 400)과 동일. 사용자가 입력을 수정할 수 있는 user-facing 경로이므로 best-effort truncate는 정보 손실을 사용자에게 알리지 않는다.

대비: bot/harvester 경로(cycle 7, `fix-harvest-pipeline-title-truncate`, PR #50)는 무인 best-effort truncate를 선택했다 — 사용자가 없으므로 fail 시 데이터를 잃는 대신 cap 맞춰 자르는 것이 더 합리적. 이 비대칭은 user-facing(reject) vs bot(truncate)의 의도된 정책이며 decision-log에 cycle 8 entry로 명시되어 있다.

## D3. cap 단위 — byte vs rune

**선택**: rune (`utf8.RuneCountInString`). PostgreSQL 16의 `character varying(N)`은 character(rune) cap이지 byte cap이 아니다. 같은 핸들러 L110의 `name` 검증, cycle 8의 pin 핸들러 검증, creator/handler.go nickname 검증 모두 `utf8.RuneCountInString`을 사용한다.

reproduction 시나리오에서 `가*167`(한국어 167 rune × 3 byte = 501 byte)는 PostgreSQL이 정상 수용(rune 167 < cap 500)하므로 byte-count로 거부하면 false positive가 발생한다. rune-count가 cap 단위와 일치하는 유일한 옵션.

## D4. 에러 메시지 형식

**선택**: `"보드 설명은 500자 이내여야 합니다"`.

근거: 같은 핸들러 `name`의 메시지 `"보드 이름은 100자 이내여야 합니다"`(L111, L288) 형식을 그대로 따른다 — `"<도메인 필드> <cap>자 이내여야 합니다"` 패턴. cycle 8 pin 핸들러의 `"설명은 500자 이내여야 합니다"`도 동일 구조. 사용자가 한 응답 메시지에서 도메인(보드/핀), 필드(이름/설명/URL/og_image), cap 수치를 모두 식별할 수 있다.

대안 `"description은 500 characters 이내"` (영어) 또는 `"설명이 너무 깁니다"` (cap 미명시)는 (a) 프로젝트 전반의 한국어 응답 메시지 컨벤션에 어긋나거나, (b) 사용자가 얼마나 줄여야 하는지 모름 → 채택 안 함.

## D5. Create vs Update 대칭

**선택**: 두 핸들러 모두 같은 위치(description 할당 직전, `if req.Description != nil` 분기 안)에 같은 블록을 복제한다. 헬퍼 함수로 추출하지 않는다.

근거:
- 같은 함수 안에서 `name` 검증도 인라인 복제 패턴(Create L110-113, Update L287-290) — 두 핸들러가 같은 검증 함수를 공유하지 않는다.
- 함수 추출 시 두 호출자(Create의 `*req.Description`, Update의 merge 분기) 모두에서 호출 패턴이 달라지고, 헬퍼 시그니처는 (값, cap, fieldName) 세 인자를 받게 되어 인라인 한 블록보다 가독성이 떨어진다.
- 향후 더 많은 핸들러로 패턴이 퍼지면 그 시점에 별개 design change로 헬퍼 추출 가능.

## D6. Update 경로 — description 검증 위치(GetBoard 전 vs 후)

**선택**: GetBoard 후. Update handler의 흐름은 `auth → boardID parse → GetBoard → ownership check → body parse → name 검증 → description merge` 순. description 검증을 description merge 직전(L294 앞)에 두면 GetBoard와 ownership check 이후가 된다.

근거:
- 인증/권한 검증이 사용자 친화 응답 우선순위보다 높아야 한다(존재하지 않는 board에 대한 cap 위반은 404가 더 정확). 
- GetBoard 결과를 description merge에 사용하지는 않지만, 이 흐름은 같은 핸들러의 `name` 검증(L283-290도 GetBoard 후)과 정렬된다.
- 단점: DB 호출 1회가 cap 검증 전에 발생 — 비용은 사용자 입력 cap 검증보다 클 수 있으나, board 단건 조회는 PK 인덱스 단일 호출로 microsecond 수준. 측정 가능한 부하 차이 없음.

## D7. 단위 테스트 scaffolding — DB 모킹 전략

**현황**: boards 핸들러는 `db.New(h.database)`로 sqlc generated 코드를 호출하며 `*sql.DB`를 직접 들고 있다. pin 핸들러처럼 `PinQuerier` interface 추상화가 없어 mock 주입이 곤란하다.

**선택**: Create 경로만 단위 테스트, Update 경로는 real-env QA로 검증.

근거:
- Create 핸들러의 description 검증은 `q := db.New(h.database)` 호출(L125) **전에** 발생하므로 DB가 nil이어도 400 응답을 만들 수 있다 — `*sql.DB`가 nil인 Handler를 생성해 description=`A*501`을 POST하면 검증이 트리거되어 400 응답을 반환한다.
- Update 핸들러는 description 검증이 `q.GetBoard(...)` 호출(L256) **이후**에 위치하므로 DB 모킹이 필수.
- DB mocking을 위해 sqlc generated `db.Querier` interface를 핸들러에 주입하는 리팩터링은 본 change 범위 밖이며, 별도 `boards.Handler` 추상화 design change로 분리해야 한다.
- Update 경로의 검증 로직은 Create와 동일한 한 줄 블록이므로 real-env QA 5단계(`description=B*501 → 400`, `description=나*501 → 400`, `description=B*500 boundary → 200`, description 생략 → 기존 보존)로 회귀 방지 충분.

대안 검토:
- `sqlmock`(DATA-DOG/go-sqlmock) 도입: 핸들러가 `*sql.DB`를 받기 때문에 가능하지만 프로젝트 어디서도 사용 중이 아니다 → 의존성 추가 → 별개 design decision으로 분리.
- 실 DB(testcontainers / docker-compose) 사용: 통합 테스트 카테고리 — 본 change 범위에서 도입 시 CI 시간/복잡도 증가.

## D8. spec ADDED 배치

**선택**: `openspec/specs/board/spec.md`의 "보드를 생성한다" Requirement 바로 아래에 신규 Requirement "보드 description 입력은 boards 컬럼 cap에 맞춰 사전 길이 검증된다"를 추가하고, 그 Requirement에 Create와 Update 양 경로 시나리오를 모두 포함한다.

근거:
- 검증이 Create/Update 두 endpoint에서 동작하지만 같은 도메인 규칙(description cap)이므로 단일 Requirement로 묶는 것이 의도를 가장 명확히 표현한다.
- 기존 "보드를 생성한다" / "보드를 수정한다" 각 Requirement에 Scenario를 추가하는 대안은 같은 규칙이 두 Requirement에 분산되어 spec 변경 시 동기화 비용이 발생.

## D9. confidence·risk

**confidence = 5**: 같은 패턴이 codebase에 3건 이미 enforce 중(creator/nickname, boards/name, pin/4 fields). PostgreSQL VARCHAR rune-count 거동은 공식 문서로 검증.

**risk = 1**: 검증 추가만 — 기존 정상 입력(cap 이하) 경로는 무영향. cap 초과 입력은 이전엔 500, 이후엔 400으로 응답 코드만 변화. 클라이언트가 500을 retry 했다면 fix 후 400으로 retry 무한 루프가 줄어드는 방향이지 늘어나는 방향 아님. DB나 데이터 변환 없음.
