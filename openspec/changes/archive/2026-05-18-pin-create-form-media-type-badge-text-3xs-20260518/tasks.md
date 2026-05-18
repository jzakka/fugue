## 1. 코드 정렬

- [x] 1.1 `apps/web/src/app/pin/new/PinCreateForm.tsx:388` 미디어 타입 배지 `<span>` className 끝에 `text-3xs` 1단어 추가

## 2. 코드 검증

- [x] 2.1 `grep -n 'text-3xs' apps/web/src/app/pin/new/PinCreateForm.tsx` 결과 L388 1건 매칭 확인
- [x] 2.2 `grep -n 'bg-accent-subtle text-accent rounded-full' apps/web/src/app/pin/new/PinCreateForm.tsx` 결과 L388 1건이 다른 utility(font-mono / px-2 / py-0.5) 유지 확인
- [x] 2.3 `git diff apps/web/src/app/pin/new/PinCreateForm.tsx` L388 한 줄만 변경 확인 (사전 작업분 hunk는 별도 archive 변경)
- [x] 2.4 부모 L386 div utility(`text-xs text-text-muted flex items-center gap-2`) 미수정 확인
- [x] 2.5 같은 부모 div 안 다른 자식(L387 파일명 / L391-395 trim / L396-402 사이즈) 미수정 확인
