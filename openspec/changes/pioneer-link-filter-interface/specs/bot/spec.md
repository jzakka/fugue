## MODIFIED Requirements

### Requirement: Link 타입은 URL과 DOM 구조 정보를 포함해야 한다

Link 타입은 URL 문자열과 DOM 조상 경로 정보를 함께 보유해야 한다(SHALL). Selector 구조체는 HTML 요소의 태그명, ID, Class 속성을 표현해야 한다(SHALL). 이 타입들은 crawler 패키지 내에 정의되어야 한다(MUST).

#### Scenario: Selector 구조체 정의

- **WHEN** Selector가 정의될 때
- **THEN** HTML 요소의 태그명, ID, Class 속성을 각각 표현하는 세 개의 공개 필드를 가져야 한다(MUST)

#### Scenario: Link 구조체 정의

- **WHEN** Link가 정의될 때
- **THEN** URL을 나타내는 필드와 DOM 조상 경로를 나타내는 Selector 목록 필드를 가져야 한다(MUST)
- **THEN** DOM 조상 경로는 루트에서 `<a>` 태그까지의 순서여야 한다(SHALL)

#### Scenario: Link 타입의 제로값 유효성

- **WHEN** Link의 제로값이 생성될 때
- **THEN** URL은 빈 문자열이고 조상 경로는 비어 있어야 한다(MUST)
- **THEN** 패닉 없이 정상 동작해야 한다(SHALL)

### Requirement: LinkFilter는 링크 목록을 받아 필터링된 목록을 반환하는 단일 연산을 정의해야 한다

LinkFilter 인터페이스는 링크 목록을 받아 필터링된 목록을 반환하는 단일 메서드를 정의해야 한다(MUST). 이 인터페이스는 bot 패키지 내에 정의되어야 한다(MUST).

#### Scenario: LinkFilter 인터페이스의 연산 계약

- **WHEN** LinkFilter 인터페이스가 정의될 때
- **THEN** Link 목록을 입력받아 필터링된 Link 목록을 반환하는 메서드 하나만 가져야 한다(MUST)
- **THEN** crawler 패키지의 Link 타입을 사용해야 한다(SHALL)

#### Scenario: 빈 슬라이스 입력 계약

- **WHEN** 빈 링크 목록이 필터에 전달될 때
- **THEN** 구현체는 빈 목록 또는 nil을 반환해야 한다(SHALL) -- 패닉이 발생해서는 안 된다(MUST NOT)

### Requirement: FilterChain은 여러 필터를 순차 적용하는 복합 구조를 제공해야 한다

FilterChain은 내부에 LinkFilter 목록을 보유해야 한다(MUST). 가변 인자로 필터들을 받는 생성자와 링크 목록에 필터를 순차 적용하는 메서드를 제공해야 한다(MUST). 순차 적용 시 각 필터의 출력이 다음 필터의 입력이 되어야 한다(SHALL).

#### Scenario: FilterChain 생성

- **WHEN** 여러 필터를 인자로 FilterChain이 생성될 때
- **THEN** 전달된 필터들이 전달 순서대로 내부에 저장되어야 한다(MUST)

#### Scenario: FilterChain 순차 적용

- **WHEN** FilterChain에 링크 목록이 전달될 때
- **THEN** 첫 번째 필터의 결과가 두 번째 필터의 입력으로 전달되어야 한다(MUST)
- **THEN** 마지막 필터의 출력이 최종 결과로 반환되어야 한다(MUST)

#### Scenario: 빈 FilterChain

- **WHEN** 필터 없이 생성된 체인에 링크 목록이 전달될 때
- **THEN** 입력 링크 목록이 그대로 반환되어야 한다(MUST)

#### Scenario: nil 입력 처리

- **WHEN** nil이 링크 목록으로 전달될 때
- **THEN** 패닉 없이 nil 또는 빈 목록을 반환해야 한다(MUST)

#### Scenario: nil 필터 원소 처리

- **WHEN** FilterChain에 nil 필터 원소가 포함될 때
- **THEN** nil 필터를 건너뛰고 나머지 필터를 정상 적용해야 한다(SHALL)
