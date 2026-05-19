## ADDED Requirements

### Requirement: Spacing 매직 리터럴 회수

코드베이스에서 DESIGN.md Spacing 스케일 카테고리에 매핑되는 매직 spacing 리터럴(예: 임의 px 값을 직접 박은 className)을 사용하지 않는다. 동일한 시각 결과를 내는 표준 spacing 토큰을 통해 SSoT 매핑을 명시화한다.

#### Scenario: 2px 간격을 표현해야 하는 컴포넌트

- **WHEN** UI 컴포넌트가 DESIGN.md Spacing 스케일의 최소 단계(2px)에 해당하는 간격을 표현해야 한다
- **THEN** 해당 컴포넌트는 매직 spacing 리터럴을 직접 박지 않고 표준 spacing 토큰을 통해 2px 간격을 만든다

#### Scenario: 보드 cover 미니어처 그리드 셀 간 간격

- **WHEN** 보드 cover의 2x2 미니어처 그리드가 렌더링된다
- **THEN** 각 셀 사이 간격은 DESIGN.md Spacing 스케일의 최소 단계(2px)에 해당하며 매직 spacing 리터럴을 사용하지 않는다

#### Scenario: 오디오 카드 waveform 바 사이 간격

- **WHEN** 오디오 핀 카드의 waveform 바 시각화가 렌더링된다
- **THEN** 각 바 사이 간격은 DESIGN.md Spacing 스케일의 최소 단계(2px)에 해당하며 매직 spacing 리터럴을 사용하지 않는다
