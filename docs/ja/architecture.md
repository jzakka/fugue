# Architecture: Fugue

**Date**: 2026-04-03
**Product**: クロスメディア創作キュレーションプラットフォーム

---

## 1. システムアーキテクチャ概要

```
                              ┌──────────────────────────────────┐
                              │           クライアント             │
                              │  (Next.js / ブラウザ / モバイル)   │
                              └──────────────┬───────────────────┘
                                             │
                                             │ HTTPS
                                             ▼
                              ┌──────────────────────────────────┐
                              │        ALB (ロードバランサー)      │
                              │         AWS ALB / Ingress        │
                              └──────────────┬───────────────────┘
                                             │
                              ┌──────────────┴───────────────────┐
                              │          EKS クラスター            │
                              │                                   │
                              │  ┌───────────┐  ┌─────────────┐  │
                              │  │  API       │  │  Next.js    │  │
                              │  │  サーバー   │  │  フロント    │  │
                              │  │  (Go)      │  │  エンド      │  │
                              │  └─────┬─────┘  └─────────────┘  │
                              │        │                          │
                              │        ▼                          │
                              │  ┌───────────────────────┐       │
                              │  │    内部サービス        │       │
                              │  │  ・認証 (OAuth)       │       │
                              │  │  ・レコメンド          │       │
                              │  │  ・oEmbed/OGP取得     │       │
                              │  └───────────┬───────────┘       │
                              │              │                    │
                              └──────────────┼────────────────────┘
                                             │
                              ┌──────────────┴───────────────────┐
                              │         データストア               │
                              │                                   │
                              │  ┌────────────┐ ┌─────────────┐  │
                              │  │ PostgreSQL │ │    Redis     │  │
                              │  │ (RDS)      │ │ (ElastiCache)│  │
                              │  └────────────┘ └─────────────┘  │
                              │                                   │
                              └───────────────────────────────────┘
```

---

## 2. 技術スタック

### アプリケーション

| レイヤー | 技術 | 選定理由 |
|---------|------|----------|
| フロントエンド | **Next.js 14 (App Router)** | SSR/SSG対応、React Server Components |
| UIライブラリ | **Tailwind CSS + shadcn/ui** | 高速プロトタイピング、一貫したデザインシステム |
| APIサーバー | **Go (net/http + chi)** | 高性能、型安全、コンパイル言語 |
| ORM | **sqlc** | SQLファースト、型安全なGoコード生成 |
| 認証 | **OAuth 2.0 (Google, GitHub)** | サードパーティ認証、パスワード管理不要 |
| oEmbed/OGP | **Go自作ライブラリ** | 対応プラットフォーム制御、キャッシュ統合 |

### インフラストラクチャ

| コンポーネント | 技術 | 選定理由 |
|--------------|------|----------|
| コンテナオーケストレーション | **AWS EKS (Kubernetes)** | スケーラビリティ、運用自動化 |
| データベース | **Amazon RDS (PostgreSQL 16)** | マネージドDB、自動バックアップ |
| キャッシュ | **Amazon ElastiCache (Redis)** | oEmbedキャッシュ、セッション管理 |
| オブジェクトストレージ | **Amazon S3** | プロフィール画像、静的アセット |
| CDN | **Amazon CloudFront** | 静的アセット配信、低レイテンシ |
| DNS | **Amazon Route 53** | ドメイン管理 |
| ロードバランサー | **AWS ALB** | L7ロードバランシング、SSL終端 |

### CI/CD

| コンポーネント | 技術 | 選定理由 |
|--------------|------|----------|
| ソースコード管理 | **GitHub** | 業界標準 |
| CI | **GitHub Actions** | GitHubとの統合、無料枠 |
| コンテナレジストリ | **Amazon ECR** | AWS統合、EKSとの連携 |
| IaC | **Terraform** | インフラのコード化、状態管理 |
| CDパイプライン | **ArgoCD** | GitOps、Kubernetes-native |

### オブザーバビリティ

| コンポーネント | 技術 | 選定理由 |
|--------------|------|----------|
| ログ | **Loki + Grafana** | 軽量ログ集約、Grafana統合 |
| メトリクス | **Prometheus + Grafana** | Kubernetes標準、豊富なexporter |
| トレース | **OpenTelemetry + Tempo** | 分散トレース、ベンダー非依存 |
| アラート | **Grafana Alerting** | 統一アラート管理 |

---

## 3. EKSクラスター構成

### ノードグループ

| ノードグループ | インスタンスタイプ | 最小/最大ノード数 | 用途 |
|--------------|-------------------|------------------|------|
| app | t3.medium | 2 / 4 | APIサーバー、フロントエンド |
| system | t3.small | 1 / 2 | ArgoCD、モニタリング |

### Namespace構成

| Namespace | コンポーネント | 説明 |
|-----------|--------------|------|
| `fugue-app` | APIサーバー、フロントエンド | アプリケーション本体 |
| `fugue-system` | ArgoCD、Ingress Controller | システムコンポーネント |
| `fugue-monitoring` | Prometheus、Grafana、Loki、Tempo | オブザーバビリティ |

### 環境別データベース/キャッシュ

| 環境 | DB | キャッシュ | 備考 |
|------|------|----------|------|
| 開発 (dev) | RDS db.t3.micro | ElastiCache cache.t3.micro | 最小構成 |
| ステージング (stg) | RDS db.t3.small | ElastiCache cache.t3.small | 本番相当 |
| 本番 (prod) | RDS db.t3.medium | ElastiCache cache.t3.medium | Multi-AZ |

---

## 4. ネットワーク設計

### VPC構成

```
VPC: 10.0.0.0/16
│
├── パブリックサブネット
│   ├── 10.0.1.0/24 (AZ-a) ── ALB, NAT Gateway
│   └── 10.0.2.0/24 (AZ-c) ── ALB
│
├── プライベートサブネット (アプリケーション)
│   ├── 10.0.11.0/24 (AZ-a) ── EKSワーカーノード
│   └── 10.0.12.0/24 (AZ-c) ── EKSワーカーノード
│
└── プライベートサブネット (データ)
    ├── 10.0.21.0/24 (AZ-a) ── RDS, ElastiCache
    └── 10.0.22.0/24 (AZ-c) ── RDS, ElastiCache
```

### トラフィックフロー

```
インターネット
    │
    ▼
CloudFront (静的アセット)
    │
    ▼
ALB (パブリックサブネット)
    │
    ▼
EKSポッド (プライベートサブネット - アプリ)
    │
    ▼
RDS / ElastiCache (プライベートサブネット - データ)
```

### セキュリティグループ

| セキュリティグループ | インバウンド | アウトバウンド | 説明 |
|-------------------|------------|--------------|------|
| sg-alb | 0.0.0.0/0:443 | sg-eks:any | ALB → インターネットからのHTTPS |
| sg-eks | sg-alb:any | sg-rds:5432, sg-redis:6379 | EKS → ALBからのトラフィック |
| sg-rds | sg-eks:5432 | なし | RDS → EKSからのみアクセス |
| sg-redis | sg-eks:6379 | なし | Redis → EKSからのみアクセス |

---

## 5. CI/CDパイプライン

```
┌──────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────┐
│  開発者   │───→│   GitHub     │───→│   GitHub     │───→│   ECR    │
│  git push │    │  リポジトリ   │    │  Actions     │    │ イメージ  │
└──────────┘    └──────────────┘    │              │    │ プッシュ  │
                                    │ ・lint       │    └────┬─────┘
                                    │ ・test       │         │
                                    │ ・build      │         ▼
                                    │ ・イメージ    │    ┌──────────┐
                                    │   プッシュ    │    │  ArgoCD  │
                                    └──────────────┘    │ 自動同期  │
                                                        └────┬─────┘
                                                             │
                                                             ▼
                                                        ┌──────────┐
                                                        │   EKS    │
                                                        │ デプロイ  │
                                                        └──────────┘
```

### パイプラインステージ

| ステージ | ツール | 内容 |
|---------|--------|------|
| Lint | golangci-lint, ESLint | コード品質チェック |
| Test | go test, Jest/Vitest | ユニット/統合テスト |
| Build | Docker | コンテナイメージビルド |
| Push | ECR | イメージレジストリへプッシュ |
| Deploy | ArgoCD | EKSクラスターへのデプロイ (GitOps) |

### ブランチ戦略

| ブランチ | 用途 | デプロイ先 |
|---------|------|-----------|
| `main` | 本番リリース | prod |
| `develop` | 開発統合 | stg |
| `feature/*` | 機能開発 | dev (手動) |
| `hotfix/*` | 緊急修正 | prod (main経由) |

---

## 6. モノレポ構成

```
fugue/
├── apps/
│   ├── web/                    # Next.js フロントエンド
│   │   ├── app/                # App Router
│   │   ├── components/         # UIコンポーネント
│   │   ├── lib/                # ユーティリティ
│   │   ├── public/             # 静的アセット
│   │   ├── Dockerfile
│   │   ├── next.config.js
│   │   ├── package.json
│   │   └── tsconfig.json
│   │
│   └── api/                    # Go APIサーバー
│       ├── cmd/
│       │   └── server/         # エントリーポイント
│       │       └── main.go
│       ├── internal/
│       │   ├── handler/        # HTTPハンドラー
│       │   ├── service/        # ビジネスロジック
│       │   ├── repository/     # データアクセス
│       │   ├── model/          # ドメインモデル
│       │   └── oembed/         # oEmbed/OGP取得
│       ├── db/
│       │   ├── migrations/     # DBマイグレーション
│       │   ├── queries/        # sqlcクエリ
│       │   └── sqlc.yaml
│       ├── Dockerfile
│       └── go.mod
│
├── infra/
│   ├── terraform/
│   │   ├── modules/
│   │   │   ├── vpc/            # VPC・サブネット
│   │   │   ├── eks/            # EKSクラスター
│   │   │   ├── rds/            # PostgreSQL
│   │   │   ├── elasticache/    # Redis
│   │   │   ├── s3/             # S3バケット
│   │   │   ├── cloudfront/     # CDN
│   │   │   └── ecr/            # コンテナレジストリ
│   │   ├── environments/
│   │   │   ├── dev/
│   │   │   ├── stg/
│   │   │   └── prod/
│   │   └── backend.tf
│   │
│   └── k8s/
│       ├── base/               # 共通マニフェスト
│       │   ├── api/
│       │   ├── web/
│       │   └── monitoring/
│       ├── overlays/           # 環境別オーバーレイ
│       │   ├── dev/
│       │   ├── stg/
│       │   └── prod/
│       └── argocd/             # ArgoCD設定
│
├── .github/
│   └── workflows/
│       ├── ci-api.yml          # API CI
│       ├── ci-web.yml          # Web CI
│       └── deploy.yml          # CDトリガー
│
├── docs/
│   ├── ja/                     # 日本語ドキュメント
│   ├── ko/                     # 韓国語ドキュメント
│   ├── en/                     # 英語ドキュメント
│   └── zh/                     # 中国語ドキュメント
│
├── .gitignore
├── README.md
└── Makefile
```

---

## 7. 月額コスト見積もり

> MVP構成 (開発/小規模本番) の月額概算

| サービス | 構成 | 月額コスト (USD) |
|---------|------|-----------------|
| EKS | クラスター料金 | $73 |
| EC2 (EKSノード) | t3.medium × 2 + t3.small × 1 | $0 (※) |
| RDS | db.t3.micro, Single-AZ | $15 |
| ElastiCache | cache.t3.micro | $12 |
| S3 | 10GB以下 | $1 |
| CloudFront | 100GB以下転送 | $9 |
| Route 53 | ホストゾーン1つ | $1 |
| ECR | イメージ10GB以下 | $1 |
| ALB | 1台 | $18 |
| NAT Gateway | 1台 | $32 |
| その他 (データ転送等) | - | $5 |
| **合計** | | **約 $167/月** |

> ※ EC2コストはEKSクラスター料金に含まないが、AWS無料利用枠またはスポットインスタンスの活用を想定。スポットインスタンス利用時は追加で約$30〜50/月。

### コスト最適化戦略

| 戦略 | 削減見込み | 説明 |
|------|-----------|------|
| スポットインスタンス | 60-70% | appノードグループにSpotを使用 |
| リザーブドインスタンス | 30-40% | RDS、ElastiCacheに1年RI適用 |
| 開発環境のスケールダウン | - | 夜間・週末にdevクラスターを停止 |
| Graviton (ARM) | 20% | t4g系インスタンスへの移行 |

---

## 8. データベーススキーマ (概要)

```sql
-- ユーザー
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname    VARCHAR(50) NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    bio         TEXT,
    avatar_url  VARCHAR(500),
    provider    VARCHAR(20) NOT NULL,  -- 'google' | 'github'
    provider_id VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ユーザー分野
CREATE TABLE user_fields (
    user_id  UUID REFERENCES users(id),
    field    VARCHAR(50) NOT NULL,  -- '作曲' | 'イラスト' | '映像編集' | ...
    PRIMARY KEY (user_id, field)
);

-- ユーザーSNSリンク
CREATE TABLE user_social_links (
    user_id   UUID REFERENCES users(id),
    platform  VARCHAR(50) NOT NULL,  -- 'twitter' | 'youtube' | 'pixiv' | ...
    url       VARCHAR(500) NOT NULL,
    PRIMARY KEY (user_id, platform)
);

-- 作品
CREATE TABLE works (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID REFERENCES users(id) NOT NULL,
    title         VARCHAR(200) NOT NULL,
    description   TEXT,
    url           VARCHAR(500) NOT NULL,
    platform      VARCHAR(50) NOT NULL,   -- 'youtube' | 'soundcloud' | 'pixiv' | ...
    field         VARCHAR(50) NOT NULL,    -- '音楽' | 'イラスト' | '映像' | ...
    oembed_data   JSONB,
    project_type  VARCHAR(50),            -- 'mv' | 'album_art' | 'collab_song' | ...
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 作品タグ
CREATE TABLE work_tags (
    work_id  UUID REFERENCES works(id),
    tag      VARCHAR(50) NOT NULL,
    category VARCHAR(20) NOT NULL,  -- 'genre' | 'mood' | 'theme'
    PRIMARY KEY (work_id, tag)
);

-- いいね
CREATE TABLE likes (
    user_id    UUID REFERENCES users(id),
    work_id    UUID REFERENCES works(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, work_id)
);

-- コラボ関心
CREATE TABLE collab_interests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_user   UUID REFERENCES users(id) NOT NULL,
    to_work_id  UUID REFERENCES works(id) NOT NULL,
    message     TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (from_user, to_work_id)
);

-- 通知
CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) NOT NULL,
    type        VARCHAR(50) NOT NULL,  -- 'like' | 'collab_interest'
    payload     JSONB NOT NULL,
    is_read     BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

### インデックス

```sql
CREATE INDEX idx_works_user_id ON works(user_id);
CREATE INDEX idx_works_field ON works(field);
CREATE INDEX idx_works_created_at ON works(created_at DESC);
CREATE INDEX idx_work_tags_tag ON work_tags(tag);
CREATE INDEX idx_work_tags_category ON work_tags(category);
CREATE INDEX idx_likes_work_id ON likes(work_id);
CREATE INDEX idx_collab_interests_to_work ON collab_interests(to_work_id);
CREATE INDEX idx_notifications_user_id ON notifications(user_id, is_read, created_at DESC);
```

---

## まとめ

Fugueのアーキテクチャは、MVPとしてシンプルに始めつつ、将来的なスケールアウトに対応できるよう設計されている。

- **フロントエンド**: Next.js 14 (App Router) + Tailwind CSS + shadcn/ui
- **バックエンド**: Go (chi + sqlc) + PostgreSQL + Redis
- **インフラ**: AWS EKS + Terraform + ArgoCD (GitOps)
- **オブザーバビリティ**: Prometheus + Grafana + Loki + Tempo

月額約$167のコストで、小規模な本番環境を運用可能。スポットインスタンスやリザーブドインスタンスの活用で更にコスト削減が見込める。

---

## キュレーションモデルのインフラ影響

### 新コンポーネントと既存インフラのマッピング

| コンポーネント | インフラ位置 | 変更事項 |
|----------------|------------|----------|
| OG fetchサービス | Go API Pod内部 | アウトバウンドトラフィック増加（fck-nat経由） |
| interactionsテーブル | RDS (prod) / CNPG (dev) | 書き込みの多いテーブル。ページビューごとにINSERT |
| 推薦キャッシュ | ElastiCache Redis (prod) | ユーザー別推薦結果キャッシング（TTL 5分） |
| boards/board_pins | RDS (prod) / CNPG (dev) | 通常のCRUD。インフラ変更不要 |

### 推薦エンジンのインフラ進化ロードマップ

- **v1 (MVP)**: Go API内部でクエリ + Redisキャッシュ。インフラ追加不要
- **v2**: K8s CronJobで定期バッチ計算。Helm templateにCronJob追加
- **v3**: ML学習パイプライン。SageMakerまたはGPUノード追加

### MVP段階の変更

VPC、EKS、CI/CD、Security Group、モニタリング、コスト: **すべて変更なし**
