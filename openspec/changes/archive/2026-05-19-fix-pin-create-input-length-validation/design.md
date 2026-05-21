# Design: POST /api/pins 입력 길이 검증

## D1. 검증 위치: 핸들러 내부 vs 미들웨어

후보:
- (A) `Create` 핸들러 내부에서 각 `r.FormValue` 직후 한 줄씩 추가
- (B) 새 미들웨어 `validatePinCreateBody`를 라우터에 끼움
- (C) sqlc `CreatePinParams` 빌더에 검증 추가

**선택: (A)**.

근거:
- 같은 핸들러가 이미 `title == "" → 400`(L93-96), `len(tagIDs) > 10 → 400`(L106-108), `uuid.Parse 실패 → 400`(L113-117) 등 라인별 검증을 인라인으로 한다. 새 검증만 별도 위치에 두면 책임 분산.
- (B)는 multipart 파싱이 핸들러 안에서 일어나므로 미들웨어가 multipart를 재파싱해야 함 — 두 번 파싱 비용 + body 재읽기 트릭 필요. 가치 대비 비용 과다.
- (C)는 핸들러에서 sql.NullString을 빌드한 이후 단계라 거부 시 message 메타데이터가 손실됨(필드명 누락). 또한 ScriptAdapter 등 다른 호출 경로가 같은 path를 공유하면 의도치 않게 적용됨.

## D2. 거부 vs 절단(truncate) 정책

후보:
- (A) cap 초과 시 400 reject
- (B) cap에 맞춰 잘라 저장(harvester `truncateRunes` 패턴)
- (C) 워닝 헤더 + 잘라 저장

**선택: (A)**.

근거:
- harvester 경로는 외부 사이트의 HTML을 가져온 결과이므로 사용자가 길이를 통제할 수 없다 → truncate가 best-effort 정책. 본 경로는 사용자가 직접 입력하는 form 필드 → 사용자가 길이를 줄여 재시도할 수 있다.
- 같은 핸들러의 `title == ""` 거부 분기와 일관: 입력 결함은 400으로 즉시 거부하는 것이 이 핸들러의 합의된 정책.
- 같은 패키지 외 동일 패턴: `creator.UpdateMe`의 nickname > 50, `boards.Create/Update`의 name > 100 모두 400 reject. codebase consensus.
- (B)는 사용자 의도와 다른 데이터를 사일런트로 저장. URL 같은 식별자는 절단되면 잘못된 곳을 가리키게 됨(특히 og_image URL).

## D3. 길이 단위: rune vs byte

후보:
- (A) `utf8.RuneCountInString` (rune)
- (B) `len(s)` (byte)

**선택: (A)**.

근거:
- PostgreSQL 16의 `VARCHAR(N)`은 N character(≒ rune) 단위로 자른다(byte 아님). `pin.title VARCHAR(200)`은 한국어 200자(= 600 byte UTF-8)를 허용한다. byte 기준으로 검증하면 한국어 67자에서 거부됨 → spec 위반.
- 같은 codebase의 nickname/board name 검증이 이미 `utf8.RuneCountInString` 사용 — 같은 함수.
- harvester `truncateRunes`도 rune iteration(range s) — 동일 단위.

## D4. 에러 메시지 형식

같은 핸들러 내 기존 메시지:
- `"제목은 필수입니다"` (L94)
- `"태그는 최대 10개까지 가능합니다"` (L107)

같은 패키지 외 길이 검증 메시지:
- `"닉네임은 50자를 초과할 수 없습니다"` (creator)
- `"보드 이름은 100자 이내여야 합니다"` (boards)

**선택**: `"X는 N자 이내여야 합니다"` 형식 채택. boards 패턴과 동일. `og_image`만 URL이므로 `"og_image URL은 1000자 이내여야 합니다"`로 명시(필드명 그대로 노출 — multipart form key가 `og_image`이므로 사용자가 어느 필드를 줄여야 할지 즉시 매핑).

## D5. 검증 순서와 자원 누수

`Create` 핸들러의 현재 순서:
1. multipart 파싱(L82-90)
2. title 빈값 검증(L93-96)
3. tag 검증(L99-133)
4. 미디어 파일 처리(L136-268, S3 업로드 L270)
5. description/url/og_image trim(L286-299)
6. 썸네일 업로드(L302-310)
7. CreatePin INSERT(L312-325)

**title 검증 위치**: L96 직후. 이유: S3 업로드(L270) 전에 거부하면 orphan 객체가 발생하지 않는다. **이는 자원 누수 방지의 의미가 있다**.

**description/url/og_image 검증 위치**: 각 trim 직후(L288/L293/L298 직후). 이미 S3 업로드(L270) 이후 단계라 거부 시 orphan 객체가 생긴다. 단, 이는 본 change로 새로 도입되는 문제가 아니라 기존부터 cap 초과 시 INSERT 실패로 동일한 orphan이 발생하던 케이스를 단지 INSERT 시점 대신 trim 시점으로 거부 위치를 옮길 뿐이다. orphan 정리는 별도 이슈로 분리(Out of Scope 참조).

선택지로 (B) "description/url/og_image도 L96 직후로 모음" 도 가능하지만, 그러면 multipart form의 모든 텍스트 필드를 한 곳에서 일괄 검증해야 함 → 현재 핸들러 구조와 어긋남(trim과 NullString 빌드가 같은 흐름에 묶여 있음). 본 change의 self-contained scope를 유지하기 위해 각 trim 직후에 인라인으로 둔다.

## D6. spec ADDED Requirement 위치

`openspec/specs/pin/spec.md:289-318` "외부 URL을 핀으로 저장한다" Requirement가 핀 생성 입력에 대한 정책을 모은 섹션이다. 본 Requirement의 Scenario들(L292-318)이 모두 이 핸들러의 동작을 기술한다.

본 change는 새 Requirement를 ADDED로 추가하며, 이 Requirement 인접 위치에 배치한다(spec.md를 직접 편집할 때). 본 deltas는 `specs/pin/spec.md`에 ADDED Requirements 섹션으로만 표기.

## D7. 회귀 위험

- **회귀 1**: 200/500/1000/1000 rune 이하 정상 입력. `utf8.RuneCountInString(s) > N`이 false → if 블록 진입 안 함 → 변경 없음.
- **회귀 2**: 빈 필드. description/url/og_image는 trim 후 빈 문자열 가드(`if d != ""`)가 이미 있어 길이 검증 블록에 진입조차 안 함. title은 L93-96에서 빈값 거부 이후이므로 비어 있지 않음.
- **회귀 3**: 정확히 cap 길이(200/500/1000/1000). `>` 비교라 통과. boundary 정확.
- **회귀 4**: 멀티바이트(한국어/이모지). `utf8.RuneCountInString`이 rune 단위로 세므로 PostgreSQL VARCHAR(N) cap과 정렬됨.

위 4 케이스를 모두 단위 테스트로 회귀 방지.

## D8. 백워드 호환성

- 기존 DB row는 모두 cap 이내(VARCHAR(N) 제약이 INSERT를 거부했을 것). 본 변경은 신규 INSERT만 영향.
- 클라이언트(Next.js)는 폼 검증을 자체적으로 수행 중일 가능성이 높음 → 본 서버 검증은 이중 방어선. 클라이언트 검증 우회 케이스에서만 동작.
- API contract(응답 스키마)는 동일: `{"error": "..."}` 형식 그대로.

## D9. 테스트 케이스 설계

`apps/api/internal/pin/handler_test.go`의 기존 패턴(`TestCreate_RejectsBodyOverCapBeforeDiskSpool` L571-608)을 참조해 multipart body를 직접 구성한다.

각 필드별 케이스 셋:
- cap+1 ASCII reject (4 케이스)
- cap+1 멀티바이트 reject (4 케이스)
- 정확히 cap 길이 통과(boundary) (4 케이스)

각 케이스는 (a) 응답 코드 400/201, (b) 응답 body의 error 메시지 또는 success 여부, (c) `mockQuerier.CreatePin` 호출 여부(reject 케이스는 호출되지 않아야 함)를 단언한다.

`mockQuerier`는 description/url/og_image 검증 케이스에서 (a) 정상 미디어 업로드를 위해 `mockStore`가 필요한가? 라는 문제가 있다. 현재 `Handler.store`는 `*storage.Client` 구체 타입이므로 mock이 직접 들어가지 않는다. 대안:
- (A) reject 케이스에서 storage가 실제로 호출되지 않도록 검증 위치를 이동(L286 직후를 L96 직후로 모음) — D5와 충돌.
- (B) `store == nil`이면 미디어 업로드를 우회하도록 핸들러를 분기 — 프로덕션 경로에 테스트용 분기 추가, 안티패턴.
- (C) 단위 테스트는 title 길이 검증(L96 직후, S3 호출 전)만 커버하고, description/url/og_image는 실 환경 QA에서만 검증. 단위 테스트는 검증 함수 자체(utf8.RuneCountInString)의 정합성을 신뢰.
- (D) 테스트용 사전 trim된 multipart body를 만들어 실제로 multipart 파싱과 S3 업로드까지 통과하게 한 뒤 description/url/og_image 검증에서 거부되는지 확인 — 통합 테스트 성격.

**선택: (C) 부분 + 기존 multipart body 패턴 활용**. title 검증은 multipart 파싱 이후 S3 업로드 전이므로 storage 의존 없이 단위 테스트 가능. description/url/og_image는 실 환경 QA에서 검증한다(QA Plan §4-§5에 명시). 단위 테스트는 4건(title cap+1 ASCII reject / title cap+1 멀티바이트 reject / title cap 통과 / 일반 입력 통과)으로 핸들러의 분기 한 줄을 직접 검증.

이는 `TestCreate_RejectsBodyOverCapBeforeDiskSpool`이 이미 사용하는 패턴(S3 호출 전 거부)과 동일 — 같은 핸들러의 검증 라인 추가에 같은 테스트 구조를 적용.

description/url/og_image 단위 테스트는 추후 핸들러 리팩토링(storage 인터페이스 도입) 시 한꺼번에 도입하는 것이 자연스러우며, 본 change의 self-contained scope를 벗어나지 않기 위해 미룬다. 실 환경 QA에서 4 필드 전부 검증되므로 사용자 관점에서 누락 없음.
