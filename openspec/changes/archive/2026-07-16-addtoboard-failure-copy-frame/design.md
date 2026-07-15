# Design: addtoboard-failure-copy-frame

## Context

`AddToBoardButton.tsx`의 `handleSelectBoard` catch 절(:196-201):

```tsx
} catch (err) {
  const message =
    err instanceof Error && err.message.includes("409")
      ? "이미 이 보드에 추가된 핀입니다"
      : "보드에 추가하지 못했습니다. 다시 시도해주세요";
  setFeedback({ type: "error", message });
}
```

뮤테이션 실패 카피 census(2026-07-16, cycle 3743):

| 사이트 | 카피 | 문형 |
|---|---|---|
| PinCreateForm.tsx:142 | 파일 처리에 실패했습니다 | 지배 |
| PinCreateForm.tsx:289 | 핀 등록에 실패했습니다 | 지배 |
| BoardActions.tsx:41 | 보드 수정에 실패했습니다 | 지배 |
| BoardActions.tsx:56 | 보드 삭제에 실패했습니다 | 지배 |
| AddToBoardButton.tsx:223 | 보드 생성에 실패했습니다 | 지배 |
| MyPageClient.tsx:49 | 보드 생성에 실패했습니다 | 지배 |
| ProfileEditForm.tsx:40 | 프로필 업데이트에 실패했습니다 | 지배 |
| login/page.tsx:8 | 인증에 실패했습니다. 다시 시도해주세요 | 지배 |
| login/page.tsx:29 | 로그인에 실패했습니다. 다시 시도해주세요 | 지배 |
| **AddToBoardButton.tsx:200** | **보드에 추가하지 못했습니다. 다시 시도해주세요** | **이탈** |

## Goals / Non-Goals

- Goal: :200 문자열을 지배 문형으로 정렬. 명사구는 같은 컴포넌트 :223 "보드 생성"과 대구를 이루는 "보드 추가".
- Non-Goal: 409 카피, 성공 카피, 다른 파일의 실패 카피, 에러 표시 UI(색/위치/aria) 변경 없음.

## Decisions

### Decision 1: 목표 카피 = "보드 추가에 실패했습니다. 다시 시도해주세요"

- "〈명사구〉에 실패했습니다" 문형 + "다시 시도해주세요" 접미(login 선례 동형) 보존.
- 대안 "보드에 추가하는 데 실패했습니다"는 지배 문형(명사구+에 실패했습니다)과 형태가 다르고 장황 → 기각.
- 대안 "핀 추가에 실패했습니다"는 같은 컴포넌트 :223 "보드 생성에 실패했습니다"와의 명사구 대구(보드 생성/보드 추가)를 깨뜨림 → 기각.

```tsx
: "보드 추가에 실패했습니다. 다시 시도해주세요";
```

### Decision 2: 테스트 무수정 확인

`rg "못했습니다" apps/web` 결과 소스 1곳(:200)뿐이며 테스트 파일에 해당 문자열 단언 없음. 기존 테스트 수정 불필요. 신규 문자열 단언 테스트도 추가하지 않는다(카피 단건 변경, c3737 선례에서는 기존 테스트가 문자열을 단언했기 때문에 수정한 것).

## Risks / Trade-offs

- 리스크 최소: 렌더 경로·상태 로직 비변경, 문자열 1건. 롤백은 원복 1행.
