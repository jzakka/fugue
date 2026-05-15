# Anti-Patterns

루프가 false positive로 분류된 제안 패턴을 누적하는 곳. 발견 모드에서 매 사이클 시작 시 전체를 읽고, 매칭되는 후보는 백로그에 올리지 않는다.

## 작성 규칙

- 한 항목 = 한 줄. 길게 쓰지 않는다.
- 형식: `- [track] 패턴: 왜 false positive인가 (출처 사이클 id 또는 날짜)`
- track: `design` | `system` | `both`
- 같은 패턴이 두 번 이상 누적되면 굵게 표시(**) 해 우선순위를 높인다.
- 패턴은 추상화해서 적는다. "이 파일 이 라인" 같은 구체 좌표는 안 된다 — 다른 파일에서도 재발하므로.

## 항목

- [system] 배포 매니페스트(helm/terraform/docker-compose)의 완성도·args·wiring 결함은 결함으로 다루지 않는다: 이전 worker-budget archive notes(2026-04-20 §4.2, 2026-04-23 §3.1)가 "실제 배포 매체가 추가되는 시점에 그 배포 change의 책임"으로 명시적 보류. 인프라 deployment change가 사용자에 의해 명시적으로 propose되기 전까지는 사이클 후보로 올리지 않는다 (cycle 29 rejected_self).
- [design] `@theme inline`에 미정의된 타이포·여백 스케일 발견에서 "토큰 추가"와 "기존 Tailwind 기본 클래스(text-sm/base/3xl 등) 의미 덮어쓰기"를 단일 항목으로 묶으면 후자가 광범위 시각 회귀를 트리거하므로 effort 추정이 무너진다. 두 작업은 별도 후보로 분리한다. (2026-05-15)
- [design] DESIGN.md radius scale(sm/md/lg/full) 외 단일 매직값(예: rounded-xl, rounded-[12px]) 발견 시, 해당 요소가 어느 등급에 속하는지 DESIGN.md가 직접 명시하지 않는 컴포넌트(로고/마스코트 박스, 작은 아이콘 컨테이너 등)는 단일 라인 교체가 자의적 등급 매핑이 되어 자체 리뷰 #2(자의적 해석)에 걸린다. DESIGN.md가 등급 매핑을 명시하거나 사용자 결정이 선행된 항목만 처리 후보로 등록한다. (2026-05-15)
