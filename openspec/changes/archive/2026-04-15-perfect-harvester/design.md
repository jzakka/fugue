## Context

Harvester는 Pioneer가 구축한 사이트 그래프를 BFS로 순회하며, 각 노드에 할당된 파싱 스크립트를 실행해 콘텐츠(RawItem)를 추출하고 Pin으로 저장하는 역할이다. 현재 `fuguebot harvester` CLI는 MockScriptExecutor(더미 RawItem 반환)와 MockPipeline(통계만 기록)을 사용하고 있어, 실제 콘텐츠 추출이 이뤄지지 않는다.

Pioneer가 pixiv 사이트를 크롤하면서 AI로 생성한 JavaScript 파싱 스크립트가 `bot_scripts` 테이블에 저장되어 있다. 이 스크립트를 실제 실행하여 HTML에서 title/description/mediaURL 등을 추출하고, 미디어를 S3에 저장하고, Pin을 DB에 생성하는 end-to-end 파이프라인이 필요하다.

## Goals / Non-Goals

**Goals:**
- Go 프로세스 내에서 JavaScript 파싱 스크립트를 실행하는 ScriptExecutor 구현
- RawItem → Pin 변환 파이프라인 구현 (dedup, media download, pin creation)
- `fuguebot harvester` CLI에서 실제 구현체를 사용하도록 연결
- 실행 결과 통계 리포트 (추출/중복/실패 건수)

**Non-Goals:**
- Python/DSL 스크립트 실행 지원 (현재 AI가 생성하는 스크립트는 JavaScript만)
- Harvester 스케줄링/자동화 (수동 CLI 실행 유지)
- 스크립트 자동 재생성/업데이트 (Pioneer 영역)
- 태그 추출 고도화 (기본 태그만, 향후 별도 변경으로)

## Decisions

### D1: JS 실행 엔진 — goja (pure Go)

**선택**: goja (Go-native JavaScript 엔진)
**대안**: 
- V8 (via cgo): 성능 우수하나 CGO 의존성으로 빌드 복잡도 증가, Docker 이미지 크기 증가
- Deno/Node subprocess: 프로세스 생성 오버헤드, 파이프 통신 복잡도

**근거**: Pioneer가 생성하는 스크립트는 DOM 파싱 로직 수준이라 ES5 호환이면 충분하다. goja는 pure Go라 빌드가 단순하고 크로스 컴파일 문제가 없다. 단, DOM API는 제공하지 않으므로 HTML을 미리 파싱하여 헬퍼 함수로 주입한다.

### D2: DOM 헬퍼 주입 방식

스크립트가 DOM API를 사용할 수 있도록, Go 측에서 goquery로 HTML을 파싱한 뒤 `querySelectorAll`, `querySelector`, `textContent`, `getAttribute`, `innerHTML` 헬퍼 함수를 goja 런타임에 주입한다. 스크립트 실행에는 타임아웃(기본 10초)을 적용하여 무한 루프를 방지한다. goja는 context 취소를 자동 감지하지 않으므로, 별도 goroutine에서 context Done 채널을 감시하여 goja `Interrupt()`를 호출하는 방식으로 구현한다. 타임아웃은 GojaExecutor 생성자의 파라미터로 설정한다. CLI에서 환경변수 또는 플래그로 값을 읽어 GojaExecutor에 직접 전달한다. 값이 0 이하이면 기본값 10000ms를 사용한다.

빈 HTML 입력 시에도 goquery 파싱은 정상 진행되며 빈 Document에 대해 스크립트를 실행한다. 스크립트가 빈 배열을 반환하면 정상, 에러를 throw하면 일반 런타임 에러로 처리한다.

스크립트 반환값은 반드시 배열이어야 한다. 단일 객체 반환은 에러로 처리한다(자동 래핑하지 않음). 스크립트가 배열을 반환하도록 AI가 생성하므로 단일 객체 반환은 스크립트 오류로 간주한다. sourceURL이 빈 문자열이거나 누락된 항목은 GojaExecutor의 결과 변환 단계에서 현재 노드 URL을 기본값으로 채운다.

### D3: Pipeline 구현 — 단일 순차 파이프라인

미디어 다운로드 시 기존 `storage.Client.Upload`를 사용한다. Content-Length가 있으면 size로 전달하여 Upload 내부의 미디어 타입별 크기 검증(image 10MB, audio 50MB, video 100MB)에 위임한다. Content-Length가 없으면 size에 -1을 전달한다. 이 경우 Upload의 사전 크기 검증은 우회되지만, 대부분의 CDN/미디어 서버가 Content-Length를 제공하므로 실제 문제 가능성은 낮다. HTTP 응답의 `resp.Body`를 `io.Reader`로 직접 Upload에 전달한다. 파일명은 UUID 기반으로 생성하되(충돌 방지), 확장자는 mediaURL 경로에서 추출한다.

봇이 생성하는 Pin의 `og_image`와 `og_data`는 NULL로 저장한다(RawItem에 해당 필드가 없으므로). 기존 `CreatePin` 쿼리가 `og_image`와 `og_data`를 필수 파라미터로 받으므로, 봇 전용 Pin INSERT 쿼리를 별도로 작성하거나 기존 쿼리에 NULL을 전달한다.

bot 패키지에 Storage 인터페이스를 정의하고, `Upload(ctx, filename, contentType string, size int64, body io.Reader) (storagePath string, err error)` 메서드를 포함한다. 기존 `storage.Client`는 `*UploadResult`를 반환하므로 인터페이스를 직접 만족하지 않는다. 간단한 어댑터를 작성하여 `UploadResult.URL`을 `storagePath`로 변환한다.

RawItem을 순차적으로 처리: dedup check(sourceURL 기반) → media download(S3) → pin creation(DB). 배치 처리나 병렬 다운로드는 첫 구현에서는 하지 않는다. 현재 Pipeline.Process 시그니처는 `(pinsCreated int, deduped int, error error)` 3개 반환값이다. 여기에 `failed int`를 추가하여 `(pinsCreated int, deduped int, failed int, err error)` 4개 반환값으로 확장한다. 기존 MockPipeline도 동일하게 업데이트한다: 현재 3개 반환값인 `ProcessFunc` 타입에 `failed int`를 추가하여 4개 반환값으로 변경하고, `TotalFailed` 필드를 추가하며, 기본 `ProcessFunc`는 `failed=0`을 반환하도록 설정한다.

### D4: 중복 체크 — sourceURL 기반

`pins.url` 필드에 sourceURL을 저장한다. ERD 설계 결정에 따라 url에는 유니크 제약이 없으므로(여러 사용자가 같은 URL을 핀할 수 있음), 봇 Pin끼리의 중복만 방지하기 위해 `WHERE url = $1 AND creator_id = $botCreatorId` 조건으로 봇 전용 중복 체크 쿼리를 별도 작성한다. 같은 배치 내 중복은 in-memory set(`map[string]bool`)으로 sourceURL을 추적하여, DB 체크와 배치 내 체크를 모두 수행한다. 기존 `PinURLExists` 쿼리(`WHERE url = $1`, creator_id 조건 없음)는 다른 코드에서 사용될 수 있으므로 유지한다. 현재 `idx_pins_url ON pins(url) WHERE url IS NOT NULL` 단일 컬럼 부분 인덱스가 존재하여 url 조건은 커버되나, 데이터 증가 시 `(url, creator_id)` 복합 인덱스 추가를 검토한다.

### D5: CLI 연결 — 환경변수로 모드 전환

`HARVESTER_MODE=real` 환경변수가 설정되면 실제 executor/pipeline 사용. `real`이 아닌 모든 경우(미설정, 빈 문자열, 기타 값)는 mock으로 동작. mock 모드에서도 DB 연결은 필요하다(그래프 순회/스크립트 조회를 위해). mock이 대체하는 것은 executor와 pipeline뿐이다.

### D6: 봇 Pin의 creator_id — 시스템 봇 계정

`pins` 테이블의 `creator_id`는 NOT NULL이므로, Harvester가 자동 생성하는 Pin에 사용할 시스템 봇 계정이 필요하다. `seed.sql`에 이미 `00000000-0000-0000-0000-00000000f096` UUID, 닉네임 `fuguebot`, email NULL인 시스템 봇 계정이 존재한다. 마이그레이션으로 새로 생성하지 않고 이 기존 계정을 재사용한다. HarvestPipeline이 해당 고정 UUID를 상수로 참조한다.

### D7: Harvester 통계 수집 — Run 반환값 확장

현재 `Harvester.Run`은 `error`만 반환한다. 노드별 Pipeline.Process 결과를 누적하여 전체 통계(총 처리 노드 수, 총 Pin 생성 수, 총 중복 수, 총 실패 수)를 수집하고, `Run`의 반환값을 통계 struct + error로 확장한다. CLI에서 이 struct를 로그로 출력한다.

## Risks / Trade-offs

- **[Risk] goja의 ES6 부분 지원**: AI가 ES6+ 문법으로 스크립트를 생성할 수 있음 → goja가 ES6 주요 문법(arrow functions, let/const, template literals, destructuring)을 지원하므로 실제 문제 가능성 낮음. async/await 등은 미지원이나 파싱 스크립트에서 사용 가능성 낮음. 실행 실패 시 에러 로그로 확인 가능
- **[Risk] DOM 헬퍼 불완전**: 모든 DOM API를 구현할 수 없음 → AI 스크립트가 사용하는 API만 우선 구현. 누락 시 에러 로그로 확인하고 점진적 추가
- **[Risk] pixiv 접근 제한**: pixiv가 로그인 필요하거나 rate limit을 걸 수 있음 → HarvesterConfig.RateLimitMs로 조절, 쿠키/헤더 지원은 향후 변경
- **[Trade-off] 순차 처리 성능**: 미디어 다운로드가 병목이 될 수 있으나, 첫 구현의 단순성을 우선
- **[Risk] 봇 계정 삭제 시 Pin CASCADE 삭제**: seed 데이터 리셋이나 봇 계정 수동 삭제 시 FK CASCADE로 봇이 생성한 모든 Pin과 관련 데이터(pin_tags, board_pins)가 삭제됨 → 삭제 전 영향 범위 확인 필요
- **[Trade-off] goja Interrupt의 DOM 헬퍼 실행 중 지연**: goja `Interrupt()`는 JavaScript 연산 사이에서만 체크되므로, DOM 헬퍼(Go native 함수) 실행 중에는 interrupt가 지연될 수 있음 → 대량 DOM에서 타임아웃 정확도가 떨어질 수 있으나, 파싱 스크립트의 DOM 조회는 일반적으로 빠르므로 실제 문제 가능성 낮음
