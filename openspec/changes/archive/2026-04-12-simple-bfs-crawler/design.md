## Context

현재 Pioneer 크롤러(`apps/api/internal/bot/pioneer.go`)는 BFS 탐색, HTTP 조회, 페이지 타입 분류, 스크립트 생성 등이 강하게 결합되어 있어 각 로직을 독립적으로 테스트하기 어렵다. 

**핵심 문제**: BFS 알고리즘과 HTTP 페이지 조회가 결합되어 있어, BFS 로직만 테스트하려면 실제 HTTP 서버가 필요하거나 복잡한 mocking이 필요하다.

**해결 방안**: Fetcher 인터페이스를 도입하여 페이지 조회 방법을 추상화한다.
- BFS 알고리즘: Crawler 컴포넌트로 독립
- 페이지 조회: Fetcher 인터페이스로 추상화
- 테스트: FileFetcher로 fixture 파일 사용
- 프로덕션: HTTPFetcher로 실제 HTTP 요청

## Goals / Non-Goals

**Goals:**
- BFS 순회 로직을 독립적인 Crawler 컴포넌트로 추출
- Fetcher 인터페이스를 통한 페이지 조회 추상화
- 테스트용 FileFetcher 구현 (파일 시스템 기반)
- 프로덕션용 HTTPFetcher 구현 (HTTP 클라이언트)
- 테스트 HTML fixture를 사용한 BFS 알고리즘 검증
- 기존 Pioneer에 Crawler 컴포넌트 통합

**Non-Goals:**
- 페이지 타입 분류 로직 변경 - 기존 Pioneer 코드 유지
- AI 스크립트 생성 로직 변경 - 기존 Pioneer 로직 유지
- DB 저장 방식 변경 - 기존 방식 유지
- 병렬 처리 추가 - 순차 BFS 유지

## Decisions

### 1. 패키지 구조: `apps/api/internal/bot/crawler/`

**결정**: 새로운 `crawler` 하위 패키지로 분리

**이유**:
- 기존 Pioneer와 독립적으로 개발 및 테스트 가능
- 향후 다른 크롤러 전략(DFS, 병렬 등)을 추가할 수 있는 확장성
- 단일 책임 원칙: BFS 탐색 로직만 담당

**대안**:
- 기존 `bot` 패키지에 직접 추가 → 기존 코드와 섞여 복잡도 증가
- 별도 top-level 패키지 → 불필요한 추상화

### 2. 인터페이스: `Crawler`와 `Fetcher`

**결정**:
```go
// Fetcher abstracts page retrieval
type Fetcher interface {
    Fetch(ctx context.Context, url string) (*FetchResult, error)
}

type FetchResult struct {
    Body        io.ReadCloser
    ContentType string
}

// Crawler performs BFS traversal
type Crawler interface {
    Crawl(ctx context.Context, rootURL string, maxDepth int) (*Result, error)
}

type Result struct {
    URLs []VisitedURL
}

type VisitedURL struct {
    URL       string
    Depth     int
    ParentURL string
    Error     error
}
```

**이유**:
- **Fetcher 인터페이스**: 페이지 조회 방법을 추상화하여 BFS 로직과 분리
- **의존성 주입**: Crawler는 Fetcher를 받아서 사용
- **테스트 용이성**: 테스트 시 FileFetcher, 프로덕션 시 HTTPFetcher 주입
- **ContentType**: HTML 여부 판단을 위해 포함

**대안**:
- Fetcher 없이 직접 HTTP 호출 → 단위 테스트 불가능
- io.Reader만 반환 → ContentType 검증 불가

### 3. HTML 파서: `golang.org/x/net/html`

**결정**: 표준 라이브러리의 `golang.org/x/net/html` 사용

**이유**:
- Go 공식 패키지로 안정성 보장
- 링크 추출에 충분한 기능 제공
- 추가 의존성 없음

**대안**:
- `goquery` → 편리하지만 불필요한 무거운 의존성
- 정규식 파싱 → HTML 구조 변화에 취약

### 4. Fetcher 구현: FileFetcher와 HTTPFetcher

**결정**: 2가지 Fetcher 구현 제공

**FileFetcher** (테스트용):
```go
type FileFetcher struct {
    basePath string // testdata 디렉토리 경로
}

func (f *FileFetcher) Fetch(ctx context.Context, url string) (*FetchResult, error) {
    // URL을 파일 경로로 변환
    // testdata/index.html 형태로 읽기
    return &FetchResult{
        Body:        file,
        ContentType: "text/html",
    }, nil
}
```

**HTTPFetcher** (프로덕션용):
```go
type HTTPFetcher struct {
    client *http.Client
}

func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (*FetchResult, error) {
    resp, err := f.client.Get(url)
    // ...
    return &FetchResult{
        Body:        resp.Body,
        ContentType: resp.Header.Get("Content-Type"),
    }, nil
}
```

**이유**:
- 동일한 Crawler 코드로 테스트와 프로덕션 모두 커버
- 테스트는 빠르고 안정적 (네트워크 불필요)
- 프로덕션은 실제 HTTP 크롤링 수행

**대안**:
- httptest.Server → 테스트 셋업 복잡하고 느림

### 5. URL 정규화: `net/url` 패키지

**결정**: `url.Parse`로 절대 URL 변환, 트레일링 슬래시 정규화

**규칙**:
- 상대 경로 → 절대 URL 변환
- fragment(#) 제거
- 트레일링 슬래시 정규화 (`/path` ≡ `/path/`)
- 쿼리 파라미터는 유지 (다른 URL로 간주)

**이유**:
- 중복 방문 방지의 핵심
- 표준 라이브러리로 안정적 처리

### 6. 도메인 검증: hostname 비교

**결정**: `url.Parse`로 hostname 추출 후 정확히 일치하는지 확인

**규칙**:
- `example.com` ≡ `example.com` (OK)
- `example.com` ≠ `www.example.com` (서브도메인 제외)
- `example.com` ≠ `other.com` (외부 도메인 제외)
- `http://example.com` ≡ `https://example.com` (프로토콜 무시)

**이유**:
- 단순하고 명확한 규칙
- 서브도메인은 별도 사이트로 간주 (기존 Pioneer 로직과 일치)

### 7. Pioneer 통합: Crawler 컴포넌트 사용

**결정**: 기존 Pioneer 코드를 Crawler + HTTPFetcher를 사용하도록 리팩토링

```go
// pioneer.go 내부
func (p *Pioneer) Run(ctx context.Context, site *Site) error {
    fetcher := crawler.NewHTTPFetcher(p.httpClient)
    c := crawler.NewBFSCrawler(fetcher)
    
    result, err := c.Crawl(ctx, site.RootURL, p.maxDepth)
    if err != nil {
        return err
    }
    
    // 기존 페이지 분류, 스크립트 생성 로직은 그대로 유지
    for _, visited := range result.URLs {
        // classifyPageType(visited.URL)
        // generateScript(...)
    }
}
```

**이유**:
- BFS 로직을 Crawler로 위임하여 책임 분리
- 기존 페이지 분류/스크립트 생성 로직은 변경 없음
- Pioneer의 외부 동작은 그대로 유지 (리팩토링)

## Risks / Trade-offs

### [Risk] 기존 Pioneer 코드 수정으로 인한 회귀 버그
**Mitigation**: 
- Crawler는 철저한 단위 테스트로 검증
- Pioneer 통합 후 기존 integration test로 회귀 검증
- 페이지 분류 로직은 변경하지 않음

### [Risk] 메모리 사용량
큰 사이트를 크롤링하면 방문 URL 맵이 메모리를 많이 차지할 수 있음.
**Mitigation**: 기존 Pioneer와 동일한 메모리 사용 패턴 유지. 최대 URL 개수 제한은 기존 방식 그대로 적용 가능.

### [Trade-off] Fetcher 추상화로 인한 간접 호출
직접 HTTP 호출 대신 인터페이스를 거치므로 약간의 오버헤드.
**Rationale**: 테스트 가능성과 유지보수성 향상이 미세한 성능 오버헤드보다 훨씬 중요.

## Open Questions

없음 - 명확한 구현 범위
