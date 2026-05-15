## ADDED Requirements

### Requirement: 시스템은 핀 조회·핀 생성·보드 추가 핸들러 진입 시 인증된 호출자에 한해 interaction 행동을 best-effort로 기록한다

시스템은 `GET /api/pins/{id}`·`POST /api/pins`·`POST /api/boards/{id}/pins` 세 핸들러가 원래 요청을 성공적으로 처리한 후 응답을 보내기 직전에, 호출자가 인증되어 있으면 `interactions` 테이블에 대응하는 `type`(`view`·`pin`·`board_add`)의 row를 INSERT해야 한다(SHALL). 이 INSERT는 best-effort이며 실패해도 원래 요청의 응답에는 영향을 주지 않는다(SHALL NOT alter the original response).

본 Requirement는 기존 Requirement `유저 행동을 기록한다`의 Scenario "작품 조회 기록"·"핀 생성 기록"·"보드 추가 기록"·"미인증 유저는 기록하지 않는다"·"기록 실패가 유저 경험에 영향을 주지 않는다"가 production에서 enforce되도록 보장하는 wiring 계약이다. 기존 Requirement의 의미와 별도 endpoint `POST /api/interactions`의 동작은 변경하지 않는다.

#### Scenario: 인증된 호출자의 핀 조회·핀 생성·보드 추가에 interaction row가 piggyback된다

- **WHEN** 인증된 호출자가 `GET /api/pins/{id}`(존재하는 핀)·`POST /api/pins`·`POST /api/boards/{id}/pins`를 호출해 원래 요청이 성공하면
- **THEN** 응답이 반환된 후(또는 응답 직전에 best-effort로) `interactions` 테이블에 `(user_id=호출자, pin_id=대상 핀, type='view'|'pin'|'board_add')` row가 하나 INSERT된다

#### Scenario: 미인증 호출자의 핀 조회에는 interaction이 기록되지 않는다

- **WHEN** 미인증 호출자가 `GET /api/pins/{id}`를 호출하면
- **THEN** 응답은 정상적으로 반환되지만 `interactions` 테이블에는 row가 INSERT되지 않는다. 라우트는 `OptionalJWTMiddleware`로 보호되어 미인증 호출이 401을 받지 않으며 동시에 piggyback 기록 분기에도 진입하지 않는다

#### Scenario: interaction 기록 INSERT가 실패해도 원래 요청 응답은 정상이다

- **WHEN** 원래 요청은 성공했지만 `CreateInteraction` INSERT가 DB 에러로 실패하면
- **THEN** 원래 요청의 응답 status·body는 영향을 받지 않는다(`GET /api/pins/{id}`는 200·핀 데이터, `POST /api/pins`는 201·새 핀, `POST /api/boards/{id}/pins`는 정상 응답). DB 에러는 서버 로그로만 남는다
