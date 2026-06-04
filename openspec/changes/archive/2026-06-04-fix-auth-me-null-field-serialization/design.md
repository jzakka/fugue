## Context

`GET /api/auth/me`(`auth.Handler.Me`)와 `GET /api/creators/me`(`creator.Handler.GetMe`)는 둘 다 JWT로 보호되며 인증된 동일 Creator를 반환한다. 두 핸들러는 `db.Creator`(`avatar_url`·`email`이 `sql.NullString`)를 읽지만 직렬화 방식이 다르다.

- `creator.GetMe` → `toPrivateDTO`: `if c.AvatarUrl.Valid { avatarURL = &c.AvatarUrl.String }` 후 `*string` 필드 → NULL은 JSON `null`.
- `auth.Me`: `map[string]interface{}{ "avatar_url": creator.AvatarUrl.String, "email": creator.Email.String }` → NULL은 zero value `""`.

코드베이스 전반(`creator`/`pin`/`boards`/`feed` DTO 30+ site)이 전자(포인터→null) 패턴을 정전으로 채택하며, `auth.Me`만 후자다.

## Goals / Non-Goals

**Goals:**
- `auth.Me`의 `avatar_url`·`email`을 미설정 시 JSON `null`로 직렬화해 `creators/me` 및 코드베이스 정전 패턴과 수렴한다.
- 값이 설정된 경우의 응답(문자열 노출)과 다른 키(`id`·`nickname`)는 불변으로 유지한다.

**Non-Goals:**
- 다른 핸들러의 직렬화 변경(이미 정전 패턴이라 변경 불필요).
- 응답에 새 필드 추가 / 라우트·인증 방식 변경.
- DB 스키마·마이그레이션 변경.

## Decisions

- **포인터 패턴으로 정렬**: `auth.Me`에서 `avatar_url`·`email`을 각각 `if .Valid`로 가드해, valid이면 문자열 값을, 아니면 `nil`(→JSON `null`)을 map에 넣는다. `map[string]interface{}`에 `nil`을 넣으면 `encoding/json`이 `null`로 직렬화하므로 별도 DTO 타입 도입 없이 최소 변경으로 달성한다(보수적 선택: 외부 계약 표면 최소 변경).
- **baseline 인용**: `creator.GetMe`→`toPrivateDTO`(`creator/dto.go:76`)의 null 표현을 정전 기준으로 삼는다. 자의적 선택이 아니라 동일 엔티티를 반환하는 자매 endpoint + 코드베이스 전역 관용에 정렬하는 것이다.

## Risks / Trade-offs

- **응답 형태 변경**: NULL `avatar_url`·`email`이 `""` → `null`로 바뀐다. 이를 소비하는 클라이언트가 빈 문자열을 기대했다면 영향이 있으나, 자매 endpoint(`creators/me`)가 이미 `null`을 반환하므로 클라이언트는 두 표현을 모두 다뤄야 했고, `null` 수렴은 오히려 계약을 단순화한다. 롤백은 단일 함수 되돌리기.
- **위험 낮음**: 변경 범위가 한 함수의 두 키 직렬화에 국한되며, 값이 있는 경로·다른 키·인증 동작은 불변.
