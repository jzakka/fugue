# tasks

- [x] 1. `apps/web/src/app/globals.css` `@theme inline` 블록 끝에 `--shadow-card-hover: 0 8px 32px rgba(0, 0, 0, 0.3);` 한 줄 추가
- [x] 2. `apps/web/src/components/feed/PinCard.tsx:146` 클래스명에서 `hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)]` → `hover:shadow-card-hover`, `focus-visible:shadow-[0_8px_32px_rgba(0,0,0,0.3)]` → `focus-visible:shadow-card-hover`
- [x] 3. `apps/web/src/app/search/SearchClient.tsx:313` 동일 치환 2건
- [x] 4. `apps/web/src/app/search/SearchClient.tsx:351` 동일 치환 2건
- [x] 5. `apps/web/src/components/board/BoardCover.tsx:6` `group-hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)]` → `group-hover:shadow-card-hover`, `group-focus-visible:shadow-[0_8px_32px_rgba(0,0,0,0.3)]` → `group-focus-visible:shadow-card-hover`
- [x] 6. `apps/web/src/components/board/BoardCover.tsx:30` 동일 치환 2건
- [x] 7. 사후 grep `shadow-\[0_8px_32px_rgba\(0,0,0,0\.3\)\]` 0건, `shadow-card-hover` 10건 확인
