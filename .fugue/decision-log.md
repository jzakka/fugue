# Decision Log

루프가 머지에 성공한 변경, 사용자가 명시적으로 내린 결정, 사용자가 반려한 변경의 이유를 누적하는 곳. 루프는 매 사이클 시작 시 마지막 10개를 읽고, 같은 결정을 다시 침범하지 않는다.

## 작성 규칙

- 한 항목 = 1~3줄.
- 형식:
  ```
  ## YYYY-MM-DD — [track] 짧은 제목
  결정/변경: 무엇을. (필요하면 PR/OpenSpec change id)
  이유: 왜. (디자인/스펙/사용자 결정 인용)
  영향 범위: 어디까지 적용되며 어디는 적용되지 않는가.
  ```
- track: `design` | `system` | `user`(사용자 직접 결정) | `reject`(반려된 변경)
- 시간순 누적. 위가 최신.

## 항목

## 2026-07-03 — [system] cycle 2150 Processing — 에러처리: fetchHTMLShared Body.Close 경고의 fmt.Printf(stdout) 채널을 log.Printf 로 교정

- **결정**: Processing (실결함 fix, confidence 3). bot/helpers.go:54 fetchHTMLShared 의 resp.Body.Close() 실패 경고가 `fmt.Printf("Warning: ...")` 로 **stdout에 무타임스탬프·무프리픽스로 출력**되어 bot 패키지 전체의 `log.Printf`(stderr·타임스탬프) 컨벤션과 어긋남 → `log.Printf("fetchHTMLShared: failed to close response body: %v", closeErr)` 로 교정 + `log` import 추가. 에러 자체는 로깅되고 있었으나(삼킴 아님) 잘못된 채널로 보고되는 에러처리 결함.
- **축 선택**: 6-area rotation 정합성→에러처리 (93주기째). cycle 2148 forward-pointer 후보 4건 스크리닝: (1) bot-visualize generateGraphviz Graphviz 미설치/DOT 실패 에러 경로 — cmd/bot-visualize/main.go:131-151 READ 로 견고 확인(미설치→:134 명시 에러+brew 설치 힌트·GenerateDOT/ExportGraphviz 에러 wrap·graphviz.go:96-99 CombinedOutput 포함 wrap·main.go:32 log.Fatalf 비정상 종료) → 결함 아님. (2) fetchHTMLShared Body.Close 경고 fmt.Printf 채널 — **실결함 채택**. (3) bot/spec.md:562 HarvestStats(census 0건)·(4) helm _helpers.tpl 부재(census 0건)는 에러처리 축이 아니라 이월.
- **검증**: (a) `grep -rn 'fmt.Print' internal/bot --include='*.go' | grep -v _test | grep -v cmd/` → helpers.go:54 가 bot 프로덕션 코드 **유일한 fmt.Print** (다른 로그 site 전부 log.Printf: harvest_pipeline.go:140/:198/:210·snapshot_first_fetch.go:99/:107/:112·harvester_consumer.go 14곳 등) → 설계 선택이 아닌 누락 확정. (b) fix 후 go build ./... OK·`go test ./internal/bot/ -run FetchHTML` 1 passed·fmt.Print 잔존 0건·구 경고 문자열("Warning: failed to close") 참조 0건(테스트 assert 없음).
- **비중첩(census)**: L350 은 resp.Body.Close 의 **lifecycle**(전 return 경로에서 닫힘·:52-56 defer 클로저 인용)·L136 은 9 site close 경로 전수+"로깅 동반" 사실만 인정 — 둘 다 close 가 *수행되는지* 축이고 경고가 *어느 채널로* 나가는지(stdout fmt.Printf vs stderr log.Printf 컨벤션)는 미커버. L92(SSRF)·L446(client 재사용 동시성)도 별축.
- **MANDATORY 체크**: census grep 수행 — `fmt.Printf`/`log 채널`/`로그 채널` 0건·`fetchHTMLShared` 매치 L92/L136/L350 전문 READ 로 비중첩 확인.
- **QA**: 로깅 채널 1-line 변경으로 동작 불변(에러 분기·반환값 무변). 단위 테스트 통과가 사전조건이며 실제 close-실패 재현은 fault-injection 필요로 생략 — 채널 교정은 코드 READ 로 검증 완결.
- **차기**: area = 동시성 (6-area rotation 에러처리→동시성, 94주기째) → cycle 2152. 후보: harvester_consumer 워커 풀 공유 상태(HarvestStats 집계 시 mutex 여부)·image_cache.go 용량 카운터/맵 동기화·pioneer_consumer 배치 내 visited map 공유·bot snapshot store 동시 쓰기. 이월(타 축): bot/spec.md:562 HarvestStats 계약(OpenSpec갭)·helm _helpers.tpl 부재 렌더 실패(정합성·helm 미설치라 미검증).
