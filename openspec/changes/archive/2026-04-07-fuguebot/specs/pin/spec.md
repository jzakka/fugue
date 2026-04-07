## ADDED Requirements

### Requirement: fuguebot 시스템 계정으로 핀을 생성한다
크롤러가 수집한 콘텐츠로 핀을 생성할 때 fuguebot 전용 시스템 계정을 소유자로 사용해야 한다(SHALL).

#### Scenario: 봇 핀 생성
- **WHEN** fuguebot이 크롤한 콘텐츠로 핀 생성을 요청하면
- **THEN** creator_id가 fuguebot 시스템 계정을 가리키는 핀이 생성된다

#### Scenario: 시스템 계정은 DB 시드에 포함된다
- **WHEN** 데이터베이스가 초기화되면
- **THEN** fuguebot 시스템 계정이 creators 테이블에 존재한다

### Requirement: 이미 수집된 URL은 중복 생성하지 않는다
크롤러는 동일한 출처 URL의 핀이 이미 존재하면 생성을 건너뛰어야 한다(SHALL).

#### Scenario: 중복 URL skip
- **WHEN** 크롤러가 수집한 URL이 이미 pins 테이블에 존재하면
- **THEN** 해당 항목을 skip하고 다음 항목을 진행한다

#### Scenario: 다른 유저가 같은 URL을 핀한 경우
- **WHEN** 유저가 이미 핀한 URL을 크롤러도 수집하면
- **THEN** 크롤러는 해당 URL을 skip한다 (기존 유저 핀 유지)

### Requirement: 크롤러 핀에 태그를 자동 할당한다
크롤러가 생성하는 핀에는 OG 메타데이터 텍스트에서 추출한 태그가 자동으로 할당되어야 한다(SHALL).

#### Scenario: 자동 태그 추출
- **WHEN** 크롤러가 핀을 생성할 때
- **THEN** 제목과 설명 텍스트에서 사전정의 태그를 자동으로 매칭하여 할당한다

#### Scenario: 태그 추출 실패 시
- **WHEN** 텍스트에서 매칭되는 태그가 없으면
- **THEN** 해당 항목을 skip 처리한다 (태그 없는 핀은 탐색/추천 불가)
