# Bot Architecture: Pioneer & Harvester

## 개요

Fugue의 크롤링 시스템은 **Pioneer**(탐색자)와 **Harvester**(수확자) 두 개의 독립적인 프로세스로 구성됩니다.

```
┌──────────────────────────────────────────────────────────────┐
│  PIONEER (AI-powered, 주기: 매일 또는 주 1회)                │
│  - 사이트 그래프 탐색 (BFS)                                   │
│  - 파싱 스크립트 생성 (AI)                                    │
│  - 스크립트 검증 (70% threshold)                              │
└──────────────────────────────────────────────────────────────┘
                            │
                            ▼
              Site Graph + Parse Scripts
                    (Persisted in DB)
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│  HARVESTER (Rule-based, 주기: 매시간 또는 매일)              │
│  - 그래프 전체 순회                                           │
│  - 저장된 스크립트로 콘텐츠 파싱                              │
│  - Pin 생성 및 저장                                          │
└──────────────────────────────────────────────────────────────┘
```

### 핵심 원칙

1. **Pioneer**: AI 사용, 비용 발생, 낮은 빈도
2. **Harvester**: 로직만 사용, 무료, 높은 빈도
3. **전체 재순회**: 두 프로세스 모두 항상 전체 그래프를 순회
4. **스마트 재사용**: 검증된 스크립트는 재생성하지 않음

---

## 🗺️ Pioneer (탐색자)

### 역할

- 사이트 내부 구조 탐색 (BFS)
- URL 그래프 생성 및 저장
- 페이지 타입 분류 (listing, gallery, detail 등)
- 파싱 스크립트 생성 (AI)
- 기존 스크립트 검증 및 재사용

### 실행 주기

- **개발**: 수동 실행
- **프로덕션**: 매일 새벽 3시 (또는 주 1회)

```yaml
schedule: "0 3 * * *"  # 매일 새벽 3시
# schedule: "0 3 * * 0"  # 매주 일요일 3시 (대안)
```

### 탐색 전략

#### 1. 우선순위 기반 BFS

링크를 우선순위별로 분류하여 효율적인 탐색:

| 우선순위 | 타입 | 키워드 예시 |
|---------|------|------------|
| 100 | Listing | trending, popular, hot, featured, recent, explore, shots |
| 80 | Gallery | gallery, collection, album, showcase |
| 60 | Category | category, tag, genre, style |
| 40 | Profile | user, artist, creator, profile |
| 10 | Detail | 개별 작품 상세 페이지 (숫자 ID 많음) |
| 0 | Skip | ad, popup, login, signup, cart |

**이유**: 리스팅 페이지는 한 번에 여러 아이템을 파싱할 수 있어 효율적. 상세 페이지는 depth를 낭비하므로 우선순위를 낮춤.

#### 2. 도메인 제한 (엄격)

```go
// ✅ 허용
dribbble.com → dribbble.com
www.dribbble.com → dribbble.com

// ❌ 차단
dribbble.com → ads.dribbble.com  // 서브도메인 다름
dribbble.com → twitter.com       // 외부 도메인
```

**이유**: 광고, 팝업, 외부 링크를 완전 차단하여 순수한 사이트 그래프 구축.

#### 3. 파일 타입 제외

- 이미지: `.jpg`, `.png`, `.gif`, `.webp`, `.svg`
- 미디어: `.mp3`, `.mp4`, `.wav`, `.webm`
- 문서: `.pdf`, `.zip`, `.exe`
- 정적 자산: `.css`, `.js`, `.json`, `.xml`

### 스크립트 생성 로직

```
각 URL 방문 시:
  ↓
기존 스크립트 존재?
  ├─ YES → 검증 (70% threshold)
  │   ├─ PASS → 스크립트 재사용, 생성 스킵 ✅
  │   └─ FAIL → AI 재생성
  └─ NO → AI 스크립트 생성
```

#### 70% 검증 기준

1. 페이지의 예상 콘텐츠 개수 추정 (휴리스틱)
   - `<img>` 태그 개수
   - `.card`, `.item`, `.post`, `<article>` 클래스 개수
   - 둘 중 큰 값 사용

2. 스크립트 실행하여 실제 파싱된 개수 확인

3. 성공률 = 파싱된 개수 / 예상 개수

4. 성공률 ≥ 70% → 스크립트 유효 ✅

**예시**:
```
페이지에 30개 이미지 → 예상 30개
스크립트가 25개 파싱 → 성공률 83% → PASS ✅

페이지에 30개 이미지 → 예상 30개
스크립트가 15개 파싱 → 성공률 50% → FAIL ❌ (재생성)
```

### AI 프롬프트

```
You are a web scraping expert for {domain}.

Site: {url}
Node Type: {listing/gallery/detail}

Analyze this HTML and generate a Cheerio-based parsing script.

Requirements:
1. Function signature: function parse($, url) { return items; }
2. Return array of objects:
   {
     title: string,
     description: string,
     media_url: string,      // Direct download URL
     source_url: string,     // Original page URL
     media_type: "image|audio|video"
   }
3. Handle missing fields gracefully
4. Convert relative URLs to absolute
5. Extract as many items as possible

Return valid JavaScript only, no markdown.
```

### 리소스

- **CPU**: 500m - 2000m
- **Memory**: 1Gi - 4Gi
- **Timeout**: 2시간
- **AI Model**: Claude 3.5 Haiku
- **AI Cost**: ~$0.01-0.10 per site

---

## 🌾 Harvester (수확자)

### 역할

- Pioneer가 생성한 그래프 전체 순회
- 저장된 스크립트로 콘텐츠 파싱
- Dedup → Download → Tag → Pin 파이프라인 실행

### 실행 주기

- **개발**: 수동 실행
- **프로덕션**: 매시간 정각 (또는 매일)

```yaml
schedule: "0 * * * *"  # 매시간
# schedule: "0 0 * * *"  # 매일 자정 (대안)
```

### 순회 전략

#### 1. 전체 재순회

모든 그래프 노드를 방문 (부분 순회 없음)

```sql
-- 모든 노드 가져오기
SELECT * FROM bot_graph_nodes WHERE site_id = $1
```

#### 2. 노드 타입별 정렬

효율성을 위해 리스팅 우선 처리:

```go
sort.Slice(nodes, func(i, j int) bool {
    return nodeTypePriority(nodes[i].NodeType) > nodeTypePriority(nodes[j].NodeType)
})

// listing (100) > gallery (80) > category (60) > detail (10)
```

**이유**: 리스팅 페이지는 한 번에 여러 아이템을 추출하므로 먼저 처리하면 중복 제거가 빨라짐.

#### 3. 스크립트 실행

```
각 노드 방문 시:
  ↓
HTML 가져오기
  ↓
저장된 스크립트 실행
  ↓
RawItem[] 추출
  ↓
Pipeline 처리
  ├─ Dedup (URL 중복 체크)
  ├─ Download (미디어 다운로드)
  ├─ Tag (태그 매칭)
  └─ Create Pin
```

### Pipeline

기존 파이프라인 재사용 (`internal/bot/engine.go`):

1. **Dedup**: `pins.url` 중복 체크
2. **Download**: 미디어 다운로드 → S3 업로드
3. **Tag**: 제목/설명으로 태그 매칭 (최대 10개)
4. **Create Pin**: `creator_id = fuguebot`

### 리소스

- **CPU**: 200m - 1000m
- **Memory**: 512Mi - 2Gi
- **Timeout**: 1시간
- **AI Cost**: $0 (AI 미사용)

---

## 📊 데이터 모델

### bot_sites

사이트 목록 및 상태 추적

```sql
CREATE TABLE bot_sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain TEXT NOT NULL UNIQUE,
    root_url TEXT NOT NULL,
    
    pioneer_status TEXT DEFAULT 'pending',  -- pending, in_progress, completed, failed
    pioneer_started_at TIMESTAMPTZ,
    pioneer_completed_at TIMESTAMPTZ,
    
    last_harvest_at TIMESTAMPTZ,
    active BOOL DEFAULT true,
    
    metadata JSONB,  -- {description, media_types, tags}
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### bot_graph_nodes

URL 그래프 노드

```sql
CREATE TABLE bot_graph_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    url TEXT NOT NULL,
    url_hash TEXT NOT NULL,  -- MD5(url) for faster lookup
    depth INT NOT NULL,      -- BFS depth from root
    node_type TEXT,          -- 'listing', 'gallery', 'detail', etc.
    parent_url TEXT,
    
    -- Script reference
    script_id UUID REFERENCES bot_scripts(id),
    
    -- Statistics
    visit_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    fail_count INT DEFAULT 0,
    last_visited_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(site_id, url_hash)
);

CREATE INDEX idx_graph_nodes_site ON bot_graph_nodes(site_id);
CREATE INDEX idx_graph_nodes_hash ON bot_graph_nodes(site_id, url_hash);
CREATE INDEX idx_graph_nodes_type ON bot_graph_nodes(site_id, node_type);
```

### bot_graph_edges

그래프 엣지 (링크 관계)

```sql
CREATE TABLE bot_graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    from_node_id UUID NOT NULL REFERENCES bot_graph_nodes(id) ON DELETE CASCADE,
    to_node_id UUID NOT NULL REFERENCES bot_graph_nodes(id) ON DELETE CASCADE,
    link_text TEXT,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(from_node_id, to_node_id)
);

CREATE INDEX idx_graph_edges_from ON bot_graph_edges(from_node_id);
```

### bot_scripts

파싱 스크립트

```sql
CREATE TABLE bot_scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    node_type TEXT NOT NULL,  -- 'listing', 'detail', etc.
    
    script_lang TEXT DEFAULT 'js',
    script_code TEXT NOT NULL,
    
    -- AI metadata
    ai_model TEXT DEFAULT 'claude-3-5-haiku',
    generation_cost_usd DECIMAL(10, 6),
    
    -- Validation statistics
    validation_success_count INT DEFAULT 0,
    validation_fail_count INT DEFAULT 0,
    last_validated_at TIMESTAMPTZ,
    
    -- Execution statistics
    success_count INT DEFAULT 0,
    fail_count INT DEFAULT 0,
    avg_execution_ms INT,
    avg_items_extracted FLOAT,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(site_id, node_type)
);

CREATE INDEX idx_scripts_site ON bot_scripts(site_id);
```

### bot_pioneer_runs

Pioneer 실행 이력

```sql
CREATE TABLE bot_pioneer_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running',  -- running, completed, failed
    
    -- Statistics
    nodes_discovered INT DEFAULT 0,
    nodes_updated INT DEFAULT 0,
    scripts_generated INT DEFAULT 0,
    scripts_reused INT DEFAULT 0,
    
    ai_api_calls INT DEFAULT 0,
    ai_cost_usd DECIMAL(10, 6) DEFAULT 0,
    
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_pioneer_runs_site ON bot_pioneer_runs(site_id);
```

### bot_harvest_runs

Harvester 실행 이력

```sql
CREATE TABLE bot_harvest_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES bot_sites(id) ON DELETE CASCADE,
    
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running',
    
    -- Statistics
    nodes_visited INT DEFAULT 0,
    nodes_succeeded INT DEFAULT 0,
    nodes_failed INT DEFAULT 0,
    items_extracted INT DEFAULT 0,
    items_deduplicated INT DEFAULT 0,
    pins_created INT DEFAULT 0,
    
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_harvest_runs_site ON bot_harvest_runs(site_id);
```

---

## ⚙️ 설정

### Pioneer 설정

```yaml
# config/pioneer.yaml
pioneer:
  schedule: "0 3 * * *"  # 매일 새벽 3시
  
  crawler:
    max_depth: 5
    max_nodes_per_site: 500
    rate_limit_ms: 2000
    
  validation:
    success_threshold: 0.7  # 70%
    
  ai:
    model: "claude-3-5-haiku"
    max_tokens: 2000
    temperature: 0.2
```

### Harvester 설정

```yaml
# config/harvester.yaml
harvester:
  schedule: "0 * * * *"  # 매시간
  
  crawler:
    rate_limit_ms: 2000
    retry_failed_nodes: true
    max_retries: 3
    
  pipeline:
    dedup_enabled: true
    download_timeout_sec: 30
    tag_min_count: 1
    tag_max_count: 10
```

---

## 🚀 CLI 커맨드

### Pioneer

```bash
# 사이트 추가
fuguebot pioneer add dribbble.com --root "https://dribbble.com/shots"

# Pioneer 실행
fuguebot pioneer run dribbble.com
fuguebot pioneer run --all

# 상태 확인
fuguebot pioneer status dribbble.com
fuguebot pioneer status --all
```

### Harvester

```bash
# Harvester 실행
fuguebot harvest dribbble.com
fuguebot harvest --all

# 통계 확인
fuguebot harvest stats dribbble.com
```

### 그래프 조회

```bash
# 그래프 시각화
fuguebot graph show dribbble.com
fuguebot graph export dribbble.com --format dot  # Graphviz
fuguebot graph export dribbble.com --format json

# 노드 목록
fuguebot graph nodes dribbble.com --type listing
fuguebot graph nodes dribbble.com --depth 2
```

### 스크립트 관리

```bash
# 스크립트 목록
fuguebot scripts list dribbble.com

# 스크립트 테스트
fuguebot scripts test dribbble.com listing --url "https://..."

# 스크립트 재생성 (강제)
fuguebot scripts regenerate dribbble.com detail
```

---

## 📈 실행 예시

### Pioneer 로그

```
🗺️ Pioneer: Starting full crawl of dribbble.com

🔍 [D0 P100] https://dribbble.com/shots
   → Type: listing, Content: ~30 items
   ✅ Script valid for listing (85%), reusing

🔍 [D1 P100] https://dribbble.com/shots/popular
   → Type: listing, Content: ~30 items
   ✅ Script valid for listing (92%), reusing

🔍 [D1 P100] https://dribbble.com/shots/recent
   → Type: listing, Content: ~30 items
   ✅ Script valid for listing (88%), reusing

🔍 [D1 P80] https://dribbble.com/collections
   → Type: gallery, Content: ~20 items
   🤖 Generating new script for gallery
   💰 AI cost: $0.008
   ✅ New script created (validation: 95%)

🔍 [D2 P60] https://dribbble.com/tags/illustration
   → Type: category, Content: ~25 items
   ✅ Script valid for listing (90%), reusing

⛔ Skipping https://dribbble.com/shots/12345-detail (Priority: 10)
⛔ Domain mismatch: ads.dribbble.com != dribbble.com

📊 Graph complete:
   - Nodes discovered: 87
   - Scripts generated: 1
   - Scripts reused: 2
   - Total AI cost: $0.008
   - Duration: 3m 42s
```

### Harvester 로그

```
🌾 Harvester: Starting full crawl of dribbble.com
📊 Total nodes: 87

🌾 [1/87] https://dribbble.com/shots (listing)
   ✅ Extracted 28 items → 25 pins (3 dupes)

🌾 [2/87] https://dribbble.com/shots/popular (listing)
   ✅ Extracted 30 items → 27 pins (3 dupes)

🌾 [3/87] https://dribbble.com/shots/recent (listing)
   ✅ Extracted 30 items → 29 pins (1 dupe)

🌾 [4/87] https://dribbble.com/collections (gallery)
   ✅ Extracted 18 items → 16 pins (2 dupes)

... (83 more nodes)

✅ Harvest complete:
   - Nodes visited: 87
   - Items extracted: 2,147
   - Pins created: 1,983
   - Dedup rate: 7.6%
   - Duration: 18m 22s
```

---

## 🔍 모니터링

### 대시보드 쿼리

```sql
-- 사이트별 현황
SELECT 
    bs.domain,
    bs.pioneer_status,
    (SELECT COUNT(*) FROM bot_graph_nodes WHERE site_id = bs.id) as nodes,
    (SELECT COUNT(*) FROM bot_scripts WHERE site_id = bs.id) as scripts,
    (SELECT SUM(pins_created) FROM bot_harvest_runs WHERE site_id = bs.id) as total_pins,
    bs.last_harvest_at
FROM bot_sites bs
WHERE bs.active = true
ORDER BY bs.last_harvest_at DESC;

-- Pioneer 실행 이력
SELECT 
    bs.domain,
    bpr.started_at,
    EXTRACT(EPOCH FROM (bpr.completed_at - bpr.started_at))/60 as duration_min,
    bpr.nodes_discovered,
    bpr.scripts_generated,
    bpr.scripts_reused,
    bpr.ai_cost_usd,
    bpr.status
FROM bot_pioneer_runs bpr
JOIN bot_sites bs ON bpr.site_id = bs.id
ORDER BY bpr.started_at DESC
LIMIT 20;

-- Harvester 실행 이력
SELECT 
    bs.domain,
    bhr.started_at,
    EXTRACT(EPOCH FROM (bhr.completed_at - bhr.started_at))/60 as duration_min,
    bhr.nodes_visited,
    bhr.items_extracted,
    bhr.pins_created,
    ROUND(bhr.pins_created::FLOAT / NULLIF(bhr.items_extracted, 0) * 100, 1) as dedup_rate,
    bhr.status
FROM bot_harvest_runs bhr
JOIN bot_sites bs ON bhr.site_id = bs.id
ORDER BY bhr.started_at DESC
LIMIT 20;

-- 스크립트 성능
SELECT 
    bs.domain,
    bsc.node_type,
    bsc.success_count,
    bsc.fail_count,
    ROUND(bsc.success_count::FLOAT / NULLIF(bsc.success_count + bsc.fail_count, 0) * 100, 1) as success_rate,
    bsc.avg_items_extracted,
    bsc.last_validated_at
FROM bot_scripts bsc
JOIN bot_sites bs ON bsc.site_id = bs.id
ORDER BY bs.domain, bsc.node_type;
```

---

## 💰 비용 분석

### Pioneer 비용 (Claude 3.5 Haiku 기준)

| 항목 | 크기 | 비용 |
|------|------|------|
| 1 페이지 분석 (입력) | ~10KB HTML = ~2.5K tokens | - |
| 스크립트 생성 (출력) | ~500 tokens | - |
| **총 비용** | **~3K tokens** | **~$0.001/페이지** |

**1 사이트 Pioneer 실행**:
- 신규 사이트: 100 페이지 × $0.001 = **$0.10**
- 재실행 (검증만): 5-10 페이지 × $0.001 = **$0.005-0.01**

**월간 비용 (10개 사이트)**:
- 초기: $1.00
- 유지: $0.20-0.50 (주 1회 재검증)

### Harvester 비용

**$0** (AI 미사용, 로직만 실행)

---

## ⚠️ 제약사항 & 주의점

### Pioneer

1. **Rate Limiting**: 2초 간격 (서버 부하 방지)
2. **Depth 제한**: 최대 5 depth (무한 재귀 방지)
3. **Node 제한**: 사이트당 최대 500 노드
4. **Timeout**: 2시간 (hung 방지)

### Harvester

1. **Rate Limiting**: 2초 간격
2. **Retry**: 실패한 노드 최대 3회 재시도
3. **Timeout**: 1시간

### 공통

1. **도메인 엄격 체크**: 서브도메인도 차단
2. **파일 타입 제외**: 정적 자산 무시
3. **상세 페이지 우선순위 낮음**: Depth 낭비 방지

---

## 🛠️ 구현 체크리스트

### Phase 1: DB & Core (Week 1-2)

- [ ] DB 스키마 마이그레이션
  - [ ] `bot_sites`
  - [ ] `bot_graph_nodes`
  - [ ] `bot_graph_edges`
  - [ ] `bot_scripts`
  - [ ] `bot_pioneer_runs`
  - [ ] `bot_harvest_runs`

- [ ] sqlc 쿼리 생성
  - [ ] Site CRUD
  - [ ] Graph CRUD
  - [ ] Script CRUD
  - [ ] Run history

### Phase 2: Pioneer (Week 2-3)

- [ ] BFS 크롤러
  - [ ] Priority Queue
  - [ ] Link classification
  - [ ] Domain validation
- [ ] AI Client
  - [ ] Claude API integration
  - [ ] Script generation
  - [ ] Cost tracking
- [ ] Script validation
  - [ ] Content count estimation
  - [ ] 70% threshold check
- [ ] Graph persistence

### Phase 3: Harvester (Week 3-4)

- [ ] Graph traversal
- [ ] Script executor (Node.js subprocess)
- [ ] Pipeline integration
  - [ ] Dedup
  - [ ] Download
  - [ ] Tag
  - [ ] Create Pin

### Phase 4: CLI & Deployment (Week 4-5)

- [ ] Cobra CLI
  - [ ] `fuguebot pioneer`
  - [ ] `fuguebot harvest`
  - [ ] `fuguebot graph`
  - [ ] `fuguebot scripts`
- [ ] Kubernetes CronJobs
  - [ ] Pioneer schedule
  - [ ] Harvester schedule
- [ ] Monitoring & Dashboards

---

## 📚 참고

- [Architecture](./architecture.md)
- [ERD](./erd.md)
- [Tech Stack](./tech-stack.md)
- [API Endpoints](./api-endpoints.md)
