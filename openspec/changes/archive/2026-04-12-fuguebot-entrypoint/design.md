## Context

Fuguebot는 Pioneer(탐색)와 Harvester(수확) 두 가지 크롤러 역할을 수행한다. 현재 `cmd/bot/main.go`는 단일 진입점으로 전체 크롤 실행만 지원하며, 개발 중 특정 사이트나 특정 크롤러만 테스트하기 어렵다.

Pioneer와 Harvester는 독립된 서비스로 `internal/bot/pioneer.go`, `internal/bot/harvester.go`에 구현되어 있으며, 각각 데이터베이스, 스토리지, AI 클라이언트 등의 의존성을 필요로 한다.

## Goals / Non-Goals

**Goals:**
- Cobra 기반 CLI로 `pioneer <site>`, `harvester <site>` 명령을 통해 각 크롤러를 독립 실행
- Makefile에 `make pioneer <site>`, `make harvester <site>` 타겟 추가하여 로컬 개발 편의성 향상
- 사이트 이름 검증 및 존재하지 않는 사이트에 대한 명확한 오류 메시지 제공
- 기존 bot 실행 방식과의 호환성 유지 (인수 없이 실행 시 안내 메시지 또는 기본 동작)

**Non-Goals:**
- Production 배포 환경 변경 (Kubernetes에서의 실행 방식은 변경하지 않음)
- Pioneer/Harvester의 내부 로직 수정
- 여러 사이트를 동시에 실행하는 배치 기능

## Decisions

### 1. Cobra CLI 프레임워크 사용
**결정**: `spf13/cobra`를 사용하여 서브커맨드 구조 구현

**이유**:
- Go 표준 CLI 라이브러리로 널리 사용됨 (kubectl, hugo 등)
- 서브커맨드, 플래그, 헬프 메시지 자동 생성 지원
- 향후 추가 커맨드 확장 용이

**대안**:
- flag 패키지 직접 사용: 서브커맨드 구조 구현이 번거롭고 헬프 메시지 수동 관리 필요
- urfave/cli: Cobra만큼 기능이 풍부하지 않고 생태계가 작음

### 2. 서브커맨드 구조
**결정**:
- 루트 커맨드: `fuguebot` (사용법 안내)
- 서브커맨드: `pioneer <site>`, `harvester <site>`

**이유**:
- 명확한 의미 전달 (어떤 크롤러를 어떤 사이트에 실행하는지)
- 향후 `status`, `list-sites` 등의 관리 커맨드 추가 가능
- 사이트는 필수 인자(arg)로 받아 간결한 CLI 제공

**대안**:
- 플래그로 모드 지정 (`--mode=pioneer --site=<site>`): 장황하고 직관성 떨어짐

### 3. 사이트 이름-도메인 매핑
**결정**: CLI에서 간단한 source name → domain 매핑 레지스트리 구현

**이유**:
- 사용자가 "unsplash"처럼 짧은 이름으로 입력 가능
- bot_sites 테이블은 domain 기반이므로 변환 필요
- Source 인터페이스는 Name()을 가지지만 domain 정보는 없음

**구현**:
```go
var sourceRegistry = map[string]string{
    "unsplash": "unsplash.com",
    "fma":      "freemusicarchive.org",
}

func resolveDomain(name string) (string, error) {
    if domain, ok := sourceRegistry[name]; ok {
        return domain, nil
    }
    // name이 이미 domain 형식이면 그대로 사용
    if strings.Contains(name, ".") {
        return name, nil
    }
    return "", fmt.Errorf("unknown site: %s", name)
}
```

**대안**:
- bot_sites에 name 컬럼 추가: 마이그레이션 필요, DB 스키마 변경
- Source 인터페이스에 Domain() 메서드 추가: Source는 크롤러용이고 Pioneer/Harvester와는 별개

### 4. 사이트 검증
**결정**: domain으로 변환 후 데이터베이스에서 사이트 존재 여부 확인

**이유**:
- 오타나 존재하지 않는 사이트로 인한 무의미한 실행 방지
- 빠른 피드백 제공 (크롤링 시작 전 실패)

**구현**:
- source name을 domain으로 변환
- `SiteRepository.GetByDomain(ctx, domain)` 호출
- 404 시 명확한 오류 메시지와 함께 종료

### 5. 의존성 초기화
**결정**: 각 서브커맨드에서 필요한 리포지토리, 클라이언트를 초기화

**이유**:
- Pioneer는 AI 클라이언트 필요, Harvester는 Pipeline 필요 등 요구사항이 다름
- 공통 인프라(DB, Storage)는 코드 재사용, 개별 의존성은 각 커맨드에서 초기화

**구조**:
```
main.go
├── root command (공통 초기화: DB, Storage)
├── pioneer command (Pioneer 생성 및 실행)
└── harvester command (Harvester 생성 및 실행)
```

### 6. Makefile 통합
**결정**: `make pioneer` / `make harvester` 타겟 추가

**예시**:
```makefile
pioneer:
	@if [ -z "$(SITE)" ]; then echo "Usage: make pioneer SITE=<site>"; exit 1; fi
	go run cmd/bot/main.go pioneer $(SITE)

harvester:
	@if [ -z "$(SITE)" ]; then echo "Usage: make harvester SITE=<site>"; exit 1; fi
	go run cmd/bot/main.go harvester $(SITE)
```

**이유**:
- 개발자가 긴 Go 명령어를 외울 필요 없음
- 일관된 개발 워크플로우 제공
- 환경변수 설정이나 빌드 옵션을 Makefile에서 중앙 관리 가능

## Risks / Trade-offs

**[Risk] Cobra 의존성 추가로 바이너리 크기 증가**
→ 영향 미미 (~100KB 이하), CLI 편의성이 더 중요

**[Risk] 사이트 이름 오타로 인한 실행 실패**
→ Mitigation: 존재하지 않는 사이트 입력 시 유사한 사이트 이름 제안 (추후 개선 가능)

**[Risk] Pioneer/Harvester 초기화 코드 중복**
→ Mitigation: 공통 초기화 로직은 헬퍼 함수로 추출하여 재사용

**[Trade-off] Makefile에서 SITE 인자를 환경변수로 전달**
→ `make pioneer SITE=<site>` 형태로 호출해야 함 (표준 make 패턴이지만 처음 사용자에게는 생소할 수 있음)
→ 문서화로 해결
