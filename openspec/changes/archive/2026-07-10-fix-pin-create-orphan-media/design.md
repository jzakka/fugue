# fix-pin-create-orphan-media — Design

## Context

`POST /api/pins`(`apps/api/internal/pin/handler.go`의 `Create`)의 현재 순서:

1. 인증 → multipart 파싱 → 제목·태그 검증
2. 미디어 파일 추출 + 비디오 처리(트리밍/길이 검증)
3. **미디어 업로드** (`h.store.Upload`, L287)
4. description/url/og_image 길이 검증 (L303-328) — **업로드 이후**
5. 썸네일 업로드 (L331-339, 실패는 warning으로 무시)
6. `CreatePin` DB insert (L341)
7. `LinkPinTag` 루프 — 실패 시 핀 row는 `DeletePin`으로 롤백하지만 **업로드된 객체는 방치**

orphan 발생 경로: (a) 4단계 검증 실패 → 400인데 미디어 객체 잔존, (b) 6단계 insert 실패 → 미디어(+썸네일) 잔존, (c) 7단계 태그 연결 실패 → 핀 row만 롤백되고 미디어(+썸네일) 잔존.

`storage.Client`(`apps/api/internal/storage/storage.go`)에는 `Upload`만 있고 삭제 기능이 없다. 핸들러 테스트는 같은 패키지(`pin`)에서 `NewHandlerWithQuerier`(store 없이 mock querier만 주입)로 업로드 이전 경로만 검증한다.

## Goals / Non-Goals

**Goals:**
- 검증 실패(4xx) 요청이 저장소에 객체를 만들지 않도록 폼 필드 검증을 업로드 앞으로 이동
- 업로드 이후 실패(핀 insert, 태그 연결) 시 미디어·썸네일 객체 보상 삭제
- 보상 삭제 실패 시 객체 key를 포함한 로그 기록
- storage에 객체 삭제 기능 추가 및 회귀 테스트

**Non-Goals:**
- 기존에 이미 쌓인 orphan 정리(백필/배치 스캐너) — 별도 운영 과제
- 썸네일 업로드 실패를 경고로 무시하는 기존 정책 변경
- 업로드/삭제의 트랜잭셔널 보장(2단계 커밋 등) 도입 — 보상 삭제(best-effort)로 충분
- 핀 삭제(DELETE /api/pins/{id}) 시 미디어 삭제 — 기존 동작 범위 밖

## Decisions

1. **폼 필드 검증(description/url/og_image)을 미디어 업로드 앞으로 이동**
   - 제목·태그 검증과 같은 블록(비디오 처리 이전)으로 올린다. 검증은 순수 문자열 길이 검사라 부작용이 없고, 실패 시 저장소·임시파일 비용 자체를 만들지 않는다.
   - 대안(업로드 후 실패 시 보상 삭제로만 해결)은 정상적인 사용자 실수(설명 길이 초과)마다 업로드+삭제 왕복을 치러 낭비다. 기각.

2. **`storage.Client`에 `Delete(ctx, key)` 추가 (S3 `DeleteObject`)**
   - `Upload`가 반환하는 `UploadResult.Key`를 그대로 사용한다. 존재하지 않는 key 삭제는 S3 시맨틱상 성공으로 처리되므로 멱등하다.

3. **보상 삭제는 실패 경로에서 best-effort로 수행, 실패는 key 포함 로그**
   - `CreatePin` 실패 시: 미디어 key(+썸네일 key가 있으면 함께) 삭제.
   - `LinkPinTag` 실패 시: 기존 `DeletePin` 롤백에 더해 미디어(+썸네일) 삭제.
   - 삭제 실패는 기존 롤백 로그 관례(`pin.Create: rollback ...`)를 따라 key를 포함해 로그만 남기고 사용자 응답은 원래 실패 응답 유지.
   - 롤백 경로 전체(객체 보상 삭제와 `LinkPinTag` 실패 시의 `DeletePin` 핀 row 롤백 모두)의 context는 `context.WithoutCancel(r.Context())`를 사용한다(Go 1.21+ API, 본 프로젝트 go.mod는 1.26). 실패 경로가 클라이언트 취소/타임아웃에서 비롯된 경우 요청 context가 이미 canceled라 보상 삭제·핀 롤백까지 연쇄 실패하는 것을 막는다. 스펙의 "핀 데이터가 되돌려지는 것과 함께 미디어도 삭제된다" 시나리오가 취소 유발 실패에서도 성립하려면 두 롤백이 같은 취소-독립 context를 써야 한다.

4. **핸들러의 store 필드를 최소 인터페이스로 교체하여 보상 삭제를 단위 테스트**
   - `pin` 패키지에 `Upload`/`Delete` 두 메서드만 갖는 비공개 인터페이스를 정의하고 `Handler.store` 타입을 이것으로 바꾼다. `*storage.Client`가 자연 충족하므로 `NewHandler` 시그니처는 불변.
   - 테스트는 같은 패키지에서 fake store(업로드 key 기록, 삭제 key 기록, 주입식 실패)를 store 필드에 직접 주입해 (a) 검증 실패 시 Upload 미호출, (b) insert/태그 실패 시 업로드된 key 전부 Delete 호출, (c) Delete 실패에도 응답 불변을 검증한다.
   - 대안(실제 MinIO 통합 테스트만): CI 의존이 커지고 실패 주입이 어렵다. 기각.

## Risks / Trade-offs

- [보상 삭제와 재시도 사이의 경합] 삭제 직전 프로세스 크래시 시 orphan은 여전히 남을 수 있다 → best-effort 설계의 한계로 수용. 발생 빈도는 현재(모든 실패가 orphan)보다 크게 감소하며, 잔존 케이스는 로그로 식별 가능.
- [검증 순서 이동으로 인한 에러 우선순위 변화] 설명 길이 초과 + 미디어 형식 오류가 동시에 있는 요청은 이전엔 미디어 오류(업로드 단계)가 먼저 보고됐으나 이제 설명 오류가 먼저 보고될 수 있다 → 응답 코드(400)는 동일하고 어떤 오류든 하나만 보고하는 기존 계약과 충돌 없음.
- [LinkPinTag 실패 시 삭제 2종(핀 row, 객체) 중 일부만 성공] 각각 독립적으로 시도하고 각각 실패 로그를 남긴다. 부분 실패도 현재(전부 방치)보다 개선.

## Migration Plan

- DB·API 계약 변경 없음. 코드 배포만으로 적용. 롤백은 커밋 revert.
- 기존에 쌓인 orphan 객체는 이 변경으로 정리되지 않는다(Non-Goal). 필요 시 별도 과제로 스캐너 도입.

## Open Questions

- 없음
