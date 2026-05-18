## 1. 라벨 className에 `font-medium` 추가 (8건)

- [x] 1.1 `apps/web/src/app/pin/new/PinCreateForm.tsx:326` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.2 `apps/web/src/app/pin/new/PinCreateForm.tsx:435` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.3 `apps/web/src/app/pin/new/PinCreateForm.tsx:451` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.4 `apps/web/src/app/pin/new/PinCreateForm.tsx:464` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.5 `apps/web/src/app/pin/new/PinCreateForm.tsx:508` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.6 `apps/web/src/components/pin/VideoThumbnailPicker.tsx:127` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.7 `apps/web/src/components/profile/ProfileEditForm.tsx:62` `<label>` className 끝에 `font-medium` 추가.
- [x] 1.8 `apps/web/src/components/profile/ProfileEditForm.tsx:75` `<label>` className 끝에 `font-medium` 추가.

## 2. 사후 검증

- [x] 2.1 `grep -rE '<label\b[^>]*className=' apps/web/src --include='*.tsx' | grep -vE 'font-medium|font-semibold|font-bold'` 결과 0건 확인(이전 8건 → 신규 0건).
- [x] 2.2 `grep -rn 'font-medium' apps/web/src --include='*.tsx'` 결과로 신규 적용 8건 확인.
- [x] 2.3 라인 시프트 0(각 라인 className 단어 1개 추가). 다른 후보/아카이브 항목의 라인 의존성에 영향 없음.
- [x] 2.4 변경 외 파일(globals.css, 다른 컴포넌트) 수정 0건 확인. git diff stat: 3 파일(PinCreateForm 10 라인 ±·VideoThumbnailPicker 2 라인 ±·ProfileEditForm 4 라인 ±) 정확히 일치.
