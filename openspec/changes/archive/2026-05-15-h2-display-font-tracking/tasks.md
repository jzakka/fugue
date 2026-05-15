# tasks

- [x] 1. `apps/web/src/app/search/SearchClient.tsx:292` className `text-lg font-semibold mb-4` → `text-lg font-semibold mb-4 font-display tracking-tight`
- [x] 2. `apps/web/src/app/search/SearchClient.tsx:306` 동일 치환
- [x] 3. `apps/web/src/app/search/SearchClient.tsx:344` 동일 치환
- [x] 4. `apps/web/src/components/pin/VideoTrimModal.tsx:128` className `text-lg font-bold text-text-primary` → `text-lg font-bold text-text-primary font-display tracking-tight`
- [x] 5. `apps/web/src/components/profile/ProfileEditForm.tsx:52` className `text-xl font-bold` → `text-xl font-bold font-display tracking-tight`
- [x] 6. 사후 grep `<h2[^>]*className=` 결과 9건 모두 `font-display`와 `tracking-tight` 보유 확인
