## ADDED Requirements

### Requirement: JavaScript 파싱 스크립트를 실행하여 콘텐츠 항목을 추출한다
스크립트 실행기는 DB에 저장된 JavaScript 스크립트를 실행하여 HTML에서 콘텐츠 항목 배열을 반환해야 한다(SHALL). 스크립트 실행 중 에러가 발생하면 빈 배열과 에러를 반환해야 한다(SHALL).

#### Scenario: 정상적인 스크립트 실행
- **WHEN** 유효한 JavaScript 스크립트와 HTML이 주어질 때
- **THEN** 스크립트가 실행되어 추출된 콘텐츠 항목 배열이 반환된다

#### Scenario: 스크립트 구문 에러
- **WHEN** 문법 오류가 있는 JavaScript 스크립트가 주어질 때
- **THEN** 빈 배열과 구문 에러를 포함한 error가 반환된다

#### Scenario: 스크립트 런타임 에러
- **WHEN** 실행 중 예외가 발생하는 스크립트가 주어질 때
- **THEN** 빈 배열과 런타임 에러를 포함한 error가 반환된다

#### Scenario: 빈 HTML 입력
- **WHEN** 빈 문자열의 HTML이 주어질 때
- **THEN** 스크립트 실행이 정상 시도된다. 스크립트가 빈 배열을 반환하면 정상 처리되고, 예외를 throw하면 런타임 에러로 처리된다

#### Scenario: 스크립트 실행 타임아웃
- **WHEN** 스크립트가 지정된 타임아웃을 초과하여 실행될 때
- **THEN** 실행이 중단되고 빈 배열과 타임아웃 에러를 포함한 error가 반환된다

---

### Requirement: DOM 헬퍼 함수를 스크립트 런타임에 주입한다
실행 런타임은 스크립트가 HTML을 탐색할 수 있도록 DOM 유사 API를 제공해야 한다(SHALL). 최소한 querySelectorAll, querySelector, textContent, getAttribute 접근을 지원해야 한다(SHALL).

#### Scenario: querySelectorAll로 요소 목록 조회
- **WHEN** 스크립트가 CSS selector를 인자로 querySelectorAll을 호출할 때
- **THEN** 매칭되는 모든 요소의 배열이 반환된다

#### Scenario: querySelector로 단일 요소 조회
- **WHEN** 스크립트가 CSS selector를 인자로 querySelector를 호출할 때
- **THEN** 첫 번째 매칭 요소가 반환되거나, 없으면 null이 반환된다

#### Scenario: 요소의 textContent 접근
- **WHEN** 스크립트가 조회된 요소의 textContent를 읽을 때
- **THEN** 해당 요소의 텍스트 내용이 반환된다

#### Scenario: 요소의 getAttribute 접근
- **WHEN** 스크립트가 조회된 요소의 getAttribute를 호출할 때
- **THEN** 해당 속성 값이 반환되거나, 없으면 null이 반환된다

---

### Requirement: 스크립트 실행 결과를 콘텐츠 항목 배열로 변환한다
스크립트의 반환값을 콘텐츠 항목 배열로 변환해야 한다(SHALL). 필수 필드(title, mediaURL, mediaType)가 누락된 항목은 건너뛰어야 한다(SHALL). 선택 필드(description, sourceURL)가 빈 문자열이거나 누락된 경우에는 항목을 건너뛰지 않고 정상 처리해야 한다(SHALL).

#### Scenario: 정상적인 결과 변환
- **WHEN** 스크립트가 title, mediaURL, mediaType 필드를 포함한 객체 배열을 반환할 때
- **THEN** 각 객체가 콘텐츠 항목으로 변환되어 반환된다

#### Scenario: 필수 필드 누락 항목 스킵
- **WHEN** 스크립트 반환값 중 title, mediaURL, 또는 mediaType이 빈 문자열이거나 누락(undefined/null)인 항목이 있을 때
- **THEN** 해당 항목은 결과에서 제외된다

#### Scenario: sourceURL 누락 시 기본값 사용
- **WHEN** 추출된 항목에 sourceURL 필드가 없거나 빈 문자열일 때
- **THEN** 스크립트 실행 시 제공된 URL이 sourceURL로 사용된다

#### Scenario: 비배열 반환값 처리
- **WHEN** 스크립트가 배열이 아닌 값을 반환할 때 (undefined, null, 문자열, 숫자, 단일 객체 등)
- **THEN** 빈 배열과 에러가 반환된다

---

### Requirement: 콘텐츠 항목 중복 체크
처리 파이프라인은 봇이 이미 저장한 콘텐츠와 동일한 sourceURL을 가진 항목을 중복으로 판단하여 건너뛰어야 한다(SHALL). 중복 판단은 봇 계정이 생성한 Pin만을 대상으로 해야 한다(SHALL). 중복 건수를 집계하여 반환해야 한다(SHALL).

#### Scenario: 신규 콘텐츠 통과
- **WHEN** sourceURL에 해당하는 봇 Pin이 DB에 존재하지 않는 항목이 입력될 때
- **THEN** 해당 항목은 중복이 아닌 것으로 판단되어 다음 단계로 진행된다

#### Scenario: 봇이 이미 수집한 콘텐츠 중복 스킵
- **WHEN** sourceURL에 해당하는 봇 Pin이 이미 DB에 존재하는 항목이 입력될 때
- **THEN** 해당 항목은 건너뛰고 중복 카운트가 증가한다

#### Scenario: 일반 사용자 Pin과는 중복 판정하지 않음
- **WHEN** 일반 사용자가 이미 동일 sourceURL로 Pin을 생성했지만 봇 Pin은 없을 때
- **THEN** 해당 항목은 중복이 아닌 것으로 판단되어 정상 처리된다

#### Scenario: 같은 배치 내 중복 처리
- **WHEN** 동일 sourceURL을 가진 항목이 같은 배치에 2개 이상 포함될 때
- **THEN** 첫 번째만 처리하고 나머지는 중복으로 처리된다

---

### Requirement: 미디어 파일을 스토리지에 다운로드하여 저장한다
처리 파이프라인은 항목의 mediaURL에서 미디어 파일을 다운로드하여 스토리지에 업로드해야 한다(SHALL). 업로드된 파일의 경로를 Pin 생성에 사용해야 한다(SHALL).

#### Scenario: 미디어 다운로드 및 업로드
- **WHEN** 지원되는 mediaType(image, audio, video)의 항목이 처리될 때
- **THEN** mediaURL에서 미디어를 다운로드하여 스토리지에 업로드하고 저장 경로를 반환한다

#### Scenario: 미디어 다운로드 또는 업로드 실패 시 해당 항목 스킵
- **WHEN** 미디어 다운로드 또는 스토리지 저장 과정에서 에러가 발생할 때 (404, timeout, 크기 초과, MIME 미지원, 네트워크 오류 등)
- **THEN** 해당 항목은 건너뛰고 에러가 로그에 기록되며 실패 카운트가 증가한다

---

### Requirement: Pin을 DB에 생성한다
처리 파이프라인은 중복 체크와 미디어 저장을 통과한 항목에 대해 Pin을 생성해야 한다(SHALL). 생성된 Pin은 시스템 봇 계정 소유여야 한다(SHALL). 생성된 Pin 수를 반환해야 한다(SHALL).

#### Scenario: 정상적인 Pin 생성
- **WHEN** 중복이 아니고 미디어 다운로드가 성공한 항목이 있을 때
- **THEN** Pin이 시스템 봇 계정 소유로 DB에 생성되고 생성 카운트가 증가한다

#### Scenario: Pin 생성 실패 시 해당 항목 스킵
- **WHEN** DB 에러로 Pin 생성이 실패할 때
- **THEN** 해당 항목은 건너뛰고 에러가 로그에 기록되며 실패 카운트가 증가하고 나머지 항목 처리는 계속된다

---

### Requirement: 처리 파이프라인이 배치 처리 통계를 반환한다
처리 파이프라인은 한 노드에서 추출된 항목 배열을 받아 처리한 뒤, 한 번의 호출 결과로 생성 건수, 중복 건수, 실패 건수를 구분하여 반환해야 한다(SHALL).

#### Scenario: 혼합 결과 통계
- **WHEN** 5개 항목 중 2개가 중복이고 1개가 다운로드 실패이고 2개가 신규일 때
- **THEN** 생성=2, 중복=2, 실패=1이 반환된다

#### Scenario: 전체 중복 시 통계
- **WHEN** 모든 항목이 중복일 때
- **THEN** 생성=0, 실패=0이고 중복이 전체 건수와 같다

---

### Requirement: Harvester 실행 완료 시 전체 통계를 집계한다
Harvester는 모든 노드의 처리가 끝난 후, 노드별 파이프라인 결과를 누적하여 전체 통계(총 처리 노드 수, 총 Pin 생성 수, 총 중복 수, 총 실패 수)를 반환해야 한다(SHALL).

#### Scenario: 다수 노드 처리 후 전체 통계
- **WHEN** 3개 노드를 처리하여 각각 생성=1/중복=2/실패=0, 생성=3/중복=0/실패=1, 생성=0/중복=5/실패=0일 때
- **THEN** 전체 통계는 노드수=3, 생성=4, 중복=7, 실패=1이 반환된다

---

### Requirement: Harvester CLI가 실제 모드를 지원한다
Harvester CLI는 설정에 따라 실제 스크립트 실행기와 처리 파이프라인을 사용하는 실제 모드를 지원해야 한다(SHALL). 실제 모드란 mock 대신 프로덕션 스크립트 실행기와 처리 파이프라인을 사용하여 실제 콘텐츠를 추출하고 Pin을 생성하는 모드를 의미한다. 기본 동작은 mock을 유지하여 기존 워크플로우에 영향을 주지 않아야 한다(SHALL). 실행 완료 후 통계를 로그로 출력해야 한다(SHALL).

#### Scenario: 실제 모드로 Harvester 실행
- **WHEN** 실제 모드가 활성화된 상태에서 Harvester를 실행할 때
- **THEN** 실제 스크립트 실행기와 처리 파이프라인이 사용되어 스크립트가 실행되고 Pin이 생성된다

#### Scenario: 기본 mock 모드 유지
- **WHEN** 실제 모드가 명시적으로 활성화되지 않은 상태에서 Harvester를 실행할 때
- **THEN** 기존과 동일하게 mock이 사용된다

#### Scenario: 인식되지 않는 설정 값은 mock으로 동작
- **WHEN** 실제 모드 설정에 인식할 수 없는 값이 주어질 때
- **THEN** mock 모드로 동작한다

#### Scenario: 실행 결과 통계 출력
- **WHEN** Harvester 실행이 완료될 때
- **THEN** 총 처리 노드 수, 생성된 Pin 수, 중복 스킵 수, 실패 수가 로그에 출력된다
