# Design: creator `avatar_url` 입력 길이 검증

## D1. 검증 위치 — handler 진입부 인라인

**선택**: handler 진입부 인라인. `nickname` 검증이 이미 같은 핸들러 L178-185에 인라인으로 존재 — 같은 패턴으로 정렬. cycle 8(pin), cycle 9(boards)도 같은 인라인 패턴을 채택. validator 라이브러리 도입은 별개 design decision으로 보류.

## D2. 정책 — 400 reject

**선택**: 400 reject. user-facing 경로이므로 사용자가 입력을 수정할 수 있다. bot 경로(harvester `truncateRunes`)의 best-effort truncate와의 비대칭은 cycle 8 decision-log entry에 명시된 의도된 정책. cycle 9 boards description 검증과도 동일.

## D3. cap 단위 — rune

**선택**: `utf8.RuneCountInString`. PostgreSQL 16의 `character varying(N)`은 rune cap. 같은 핸들러 nickname 검증(L182), cycle 8/9 작업 모두 동일. 한국어 167 rune × 3 byte = 501 byte 입력은 PostgreSQL이 정상 수용(rune 167 < cap 500)하므로 byte-count로 거부하면 false positive 발생.

## D4. 에러 메시지 — "아바타 URL은 500자 이내여야 합니다"

**선택**: 도메인 친화 한국어. 같은 핸들러 nickname의 `"닉네임은 50자를 초과할 수 없습니다"`(L183)와 cycle 8/9의 `"X는 N자 이내여야 합니다"` 형식 사이의 결합. 필드명은 "아바타 URL"(사용자 친화 한국어) — `avatar_url`(snake_case 식별자)을 그대로 노출하지 않는다.

cycle 9 `"보드 설명은 500자 이내여야 합니다"`, cycle 8 `"제목은 200자 이내여야 합니다"` / `"설명은 500자 이내여야 합니다"` / `"URL은 1000자 이내여야 합니다"` / `"og_image URL은 1000자 이내여야 합니다"` 모두 동일 패턴(`<도메인 필드> <cap>자 이내여야 합니다`).

## D5. 빈 문자열(`""`) clear semantics 보존

**선택**: 빈 문자열은 cap 검증을 건너뛰고 기존 clear 동작(L189-190 `avatarURL = sql.NullString{}`)을 유지한다.

근거:
- 사용자가 `{"avatar_url": ""}`로 PUT 하면 현재 코드는 avatar_url을 NULL로 설정 — 의도된 "아바타 제거" 기능.
- 빈 문자열을 cap 검증에 통과시킨 뒤 NullString.Valid=false로 분기 — `""`도 0 rune이므로 `0 > 500`은 false. 따라서 검증 코드를 단순히 cap 비교만으로 작성해도 빈 문자열은 정상적으로 NULL로 떨어진다.
- 다만 명시성을 위해 cap 비교는 `else`(`*req.AvatarURL != ""`) 분기 내부에 배치해 의도 표명한다.

## D6. 단위 테스트 — CreatorQuerier mock 활용

**현황**: creator 핸들러는 `CreatorQuerier` interface로 추상화되어 있고 `NewHandlerWithQuerier(q)` 생성자 + `mockQuerier`가 이미 `handler_test.go`에 완비.

**선택**: 5 신규 subtest 모두 같은 mock으로 unit test. UpdateMe는 GetCreator → 검증 → UpdateCreator 흐름이므로 mock이 GetCreator 결과를 제공하면 cap 검증 분기까지 도달 가능.

테스트 5건:
1. ASCII 501 reject — `mock.lastUpdate`가 비어있음을 확인(UpdateCreator 미호출 입증).
2. 한국어 501 reject — rune cap 확인.
3. ASCII 500 boundary — UpdateCreator 호출되고 `lastUpdate.AvatarUrl.String` 길이 500 확인.
4. `""` clear — UpdateCreator 호출되고 `lastUpdate.AvatarUrl.Valid == false` 확인.
5. avatar_url 누락 — UpdateCreator 호출되고 `lastUpdate.AvatarUrl == current` 확인(기존 값 보존).

cycle 9 boards에서는 querier interface 부재로 Update 경로를 real-env QA에만 의존했지만, creator는 interface가 있어 단위 테스트로 전체 경로 cover 가능.

## D7. spec ADDED 배치

**선택**: `openspec/specs/profile/spec.md`의 "본인 프로필을 수정한다" Requirement 부근에 신규 Requirement "avatar_url 입력은 creators 컬럼 cap에 맞춰 사전 길이 검증된다"를 추가. clear semantics(빈 문자열은 NULL clear)와 omitted semantics(기존 값 보존)도 함께 명시.

## D8. confidence·risk

**confidence = 5**: 같은 패턴이 codebase에 3건 이미 enforce 중(pin/4 fields, boards name+description, creator nickname). PostgreSQL VARCHAR rune-count 거동은 공식 문서로 검증.

**risk = 1**: 검증 추가만 — 기존 정상 입력(cap 이하 또는 빈 문자열) 경로는 무영향. cap 초과 입력은 이전엔 500, 이후엔 400으로 응답 코드만 변화. DB나 데이터 변환 없음.
