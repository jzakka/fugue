## ADDED Requirements

### Requirement: Light mode 색상 토큰의 SSoT 정렬

Light mode가 활성화된 상태에서 본문/보조 텍스트·카드 경계·태그 배경 등 디자인 가이드(DESIGN.md)에 명시된 색상 카테고리는 디자인 가이드가 지정한 명도/투명도 단계로 렌더링되어, 코드의 토큰 정의값과 디자인 가이드 명세 사이에 어긋남이 없어야 한다.

#### Scenario: Light mode 토글 시 본문·보조 텍스트의 명도가 디자인 가이드 명세를 따른다

- **WHEN** 사용자가 light mode를 활성화하여 페이지를 본다
- **THEN** 본문 보조 텍스트(text muted 카테고리)와 부차 보조 텍스트(text dim 카테고리)는 디자인 가이드가 지정한 light mode 명도로 렌더링된다

#### Scenario: Light mode 토글 시 카드 경계와 태그 배경이 디자인 가이드 명세를 따른다

- **WHEN** 사용자가 light mode를 활성화하여 카드/태그가 포함된 페이지를 본다
- **THEN** 카드/입력의 경계(border 카테고리)는 디자인 가이드가 지정한 light mode 명도로 렌더링되고, 태그 배경·선택 상태(accent subtle 카테고리)는 디자인 가이드가 지정한 단일 명세값(dark/light 분리 없는 값)으로 렌더링된다

#### Scenario: 토큰 정의값과 디자인 가이드 명세 사이에 어긋남이 누적되지 않는다

- **WHEN** 디자인 가이드가 light/dark 두 모드 색상 카테고리의 명세값을 지정한다
- **THEN** 코드 SSoT의 토큰 정의는 dark mode와 light mode 모두 디자인 가이드 명세값과 직접 일치하여, 어느 한 쪽 모드만 잔여 어긋남이 누적되지 않는다
