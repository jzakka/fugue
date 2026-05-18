# Tasks

## 1. globals.css `.light` 블록 4줄 값 교체

- [x] 1.1 `apps/web/src/app/globals.css:33` `--text-muted: #666666;` → `--text-muted: #777777;` (DESIGN.md L48 'Text muted: ... / #777777')
- [x] 1.2 `apps/web/src/app/globals.css:34` `--text-dim: #999999;` → `--text-dim: #AAAAAA;` (DESIGN.md L49 'Text dim: ... / #AAAAAA')
- [x] 1.3 `apps/web/src/app/globals.css:35` `--accent-subtle: rgba(232, 90, 42, 0.08);` → `--accent-subtle: rgba(232, 90, 42, 0.12);` (DESIGN.md L41 'Accent subtle: rgba(232, 90, 42, 0.12)' — 단일 값)
- [x] 1.4 `apps/web/src/app/globals.css:36` `--border: #D0D0D0;` → `--border: #E0E0E0;` (DESIGN.md L46 'Border: ... / #E0E0E0')

## 2. 사후 검증

- [x] 2.1 `git diff apps/web/src/app/globals.css` 결과 `:root` 블록(L9-25) 무수정 확인 — dark mode 동등성 보장
- [x] 2.2 `grep -n '#666666\\|#999999\\|0.08\\|#D0D0D0' apps/web/src/app/globals.css` 결과 0건 확인 — 잔여 어긋남 없음
- [x] 2.3 DESIGN.md L41/L46/L48/L49 4건 명세값과 globals.css `.light` 4건 토큰값 직접 일치 확인
