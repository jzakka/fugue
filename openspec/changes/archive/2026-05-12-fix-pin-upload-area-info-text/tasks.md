## 1. UI 안내 텍스트 추가

- [x] 1.1 `apps/web/src/app/pin/new/PinCreateForm.tsx`의 파일 업로드 dropzone 안내 영역에 "허용 미디어 타입과 자동 최적화가 적용됨"을 안내하는 텍스트를 추가한다.

## 2. 검증

- [x] 2.1 `apps/web/src/app/pin/new/__tests__/PinCreateForm.test.tsx` 파일을 신규 생성하여, 파일이 선택되지 않은 dropzone 상태에서 안내 텍스트가 렌더링되는지 확인하는 단위 테스트를 추가한다. (디렉터리도 함께 생성. 테스트 인프라는 기존 `apps/web/src/components/feed/__tests__/*` 컨벤션을 따른다.)
- [x] 2.2 `npm test`로 통과 확인.
- [x] 2.3 `npm run build`로 빌드 통과 확인.
