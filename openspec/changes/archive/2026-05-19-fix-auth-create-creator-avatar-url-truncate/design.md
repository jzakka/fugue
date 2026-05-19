# Design: OAuth `profile.AvatarURL` 사전 절단

## Decision 1: silent truncate vs reject

`decision-log` 2026-05-19 cycle 10이 명시한 정책 dichotomy:
- **user-facing 경로**: 400 reject(사용자가 입력 수정 가능).
- **외부 시스템 ingress(bot harvester)**: silent truncate(`truncateRunes`).

OAuth provider 응답은 외부 시스템 데이터에 해당. 또한 OAuth callback flow 중간에 400 응답을 띄울 surface가 없다 — callback은 frontend로 redirect되며, 에러 시 redirect param에 에러 코드를 실어보내는 것이 표준 패턴이라 "avatar URL이 너무 깁니다" 같은 필드별 메시지를 사용자에게 전달하기 어렵다. 그리고 사용자는 자신의 OAuth provider 프로필 사진 URL을 인지하지도 수정할 수도 없다.

→ **silent truncate**. 같은 파일의 `truncateNickname`이 이미 silent truncate를 사용 중. consistency가 자연스럽다.

## Decision 2: helper 위치

`truncateNickname`이 `service.go` 하단의 helper section(L292-308)에 정의되어 있다. `truncateAvatarURL`도 같은 위치에 추가한다.

대안(별도 파일로 분리)을 고려했으나:
- truncate 함수들이 늘어나면 별도 `truncate.go` 또는 `validation.go`로 분리할 가치가 있으나 현재 2개 함수만으로는 분리 비용이 효용을 초과.
- `service.go`가 너무 길어지는 위험: 현재 ~310 라인 + 본 변경 +10 라인. 임계점 미도달.

→ **동일 파일**.

## Decision 3: helper signature

`truncateNickname(name string) string` 패턴 매칭:

```go
func truncateAvatarURL(url string) string {
    r := []rune(url)
    if len(r) > 500 {
        r = r[:500]
    }
    return string(r)
}
```

- `utf8.RuneCountInString` 대신 `[]rune(...)` slice 사용 — `truncateNickname`의 정확한 알고리즘 일치(slice는 절단할 때 어차피 필요, RuneCountInString은 절단 안 할 때만 빠름; cap 초과 케이스는 절단이 일어나므로 slice 필요).
- 빈 문자열 입력에 특별 처리 없음 — `truncateNickname`은 빈 nickname에 `"creator-" + uuid` 대체값을 반환하지만, avatar_url은 빈 값이 의미를 가지므로(NULL 저장 의도) 빈 입력은 그대로 반환. `toNullString`이 `""`을 `sql.NullString{}`으로 변환하는 기존 흐름 보존.

## Decision 4: cap 값 500의 출처

`apps/api/db/migrations/000001_create_creators.up.sql:7` `avatar_url VARCHAR(500)` 명시. const 또는 named constant로 추출할지 검토했으나:
- 같은 cap 값이 코드베이스의 다른 곳에서 재사용되지 않음(`creator/handler.go` UpdateMe의 avatar_url cap 검증은 별개 cap 값이지만 cycle 10에서 magic number `500`을 직접 사용한 선례).
- cycle 10 패턴 일치를 위해 magic number 직접 사용.

→ **inline `500` literal**. 함수명 `truncateAvatarURL`이 의미를 명확히 전달.

## Decision 5: 호출부 wrapping 위치

`toNullString(truncateAvatarURL(profile.AvatarURL))` 형식으로 `toNullString` 바깥에 truncate를 둔다.

대안(`toNullString` 내부에서 cap 인자 받기)을 고려했으나:
- `toNullString`은 범용 `string → sql.NullString` 변환기로 cap 의식이 없는 것이 자연스럽다. cap 의식 변환기로 바꾸면 다른 호출처(`email`, `provider_id`)도 영향.
- 함수 합성이 더 가독성 좋음.

→ **명시적 합성**.

## Decision 6: 테스트 범위

`truncateAvatarURL`만 unit test로 cover한다. service-level 통합 테스트로 OAuth flow 전체를 cover하는 것은 본 change 범위 밖(이미 mock provider 인프라가 부재하며, 단순 helper 테스트로 회귀를 충분히 막을 수 있다).

`truncateNickname` 회귀 테스트도 함께 추가 — 같은 알고리즘을 적용하기 때문에 미래 변경 시 두 helper가 분기되지 않도록 정렬 테스트를 둔다(`TestTruncateNickname_AvatarURLPolicyParity`).

## Risk Assessment

- **회귀 위험**: ≈0. 정상 길이 입력은 절단 분기 미발동(`len(r) <= 500` 체크). 기존 OAuth flow는 동일.
- **데이터 변환 위험**: 없음. 이미 저장된 creators row는 cap을 어차피 만족(기존에 cap 초과 INSERT는 실패해 row 자체가 없음). 본 change는 forward-only.
- **성능**: `[]rune(url)` slice 한 번 — O(N) where N ≤ ~600 chars. 무시 가능.
- **보안**: 변경 없음. truncated URL은 여전히 같은 host prefix를 유지(URL은 일반적으로 host가 앞쪽에 있으며 절단은 query string에서 발생). 만약 host가 절단되면 URL이 무효해지지만 그 시점에 이미 URL이 500+ chars로 비정상 — 표시 시 깨진 이미지로 fallback될 뿐 보안 영향 없음.

## Migration Path

없음. 코드 변경만으로 forward-only 적용. 기존 DB row 영향 없음.
