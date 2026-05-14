# TODOS

## 사이트별 CSS Selector 기반 필터 규칙

**Priority:** Medium
**Added:** 2026-04-13 (eng review)
**Depends on:** 링크 필터 체인 PR 완료

현재 범용 필터만 있는데, 사이트마다 DOM 구조가 다르므로 사이트별 selector 규칙을 `bot_sites` 테이블에 저장하면 필터링 정확도 향상 가능. Link 구조체의 `[]Selector` 데이터를 활용하여 사이트별 특정 영역의 링크만 추출하거나 제외하는 규칙 적용. `bot_sites`에 `content_selector` 또는 `exclude_selectors` 컬럼 추가 필요.

## robots.txt / rel=nofollow 준수 레이어

**Priority:** Medium
**Added:** 2026-04-13 (eng review)
**Depends on:** 링크 필터 체인 PR 완료

크롤링 예절과 법적 리스크 감소를 위해 robots.txt 파싱 + rel=nofollow 링크 제외. 링크 필터 체인에 `RobotsFilter`를 추가하면 자연스러움. Go용 robots.txt 파서 라이브러리 필요 (e.g., `github.com/temoto/robotstxt`). 매 사이트 첫 크롤 시 robots.txt를 한 번 fetch하여 캐시.

## Graph Visualization: D3 CDN 오프라인 미지원

**Priority:** Medium
**Added:** 2026-04-14 (QA ISSUE-005, deferred)
**File:** `apps/api/cmd/bot-visualize/template.html:7`

D3.js를 CDN(`d3js.org`)에서만 로딩. 오프라인이나 프록시 환경에서 graph.html이 동작하지 않음. 인라인 번들링 또는 onerror fallback 검토 필요.

## Graph Visualization: 노드 라벨에 URL 패턴 미표시

**Priority:** Low
**Added:** 2026-04-14 (QA ISSUE-006, deferred)
**File:** `apps/api/cmd/bot-visualize/template.html:333`

노드 라벨이 `node_type`만 표시 (listing, detail 등). 333개 노드 중 80%+가 같은 텍스트. URL 패턴(`/ranking.php?mode=daily`)을 표시하면 가독성 향상.

## Graph Visualization: 키보드/스크린 리더 미지원

**Priority:** Low
**Added:** 2026-04-14 (QA ISSUE-007, deferred)
**File:** `apps/api/cmd/bot-visualize/template.html`

SVG 노드에 `tabindex`, `role`, `aria-label` 없음. 마우스 전용 인터랙션. 개발자 도구이므로 우선도 낮음.

## DB 마이그레이션 24번 실패 (bot_pioneer_runs 테이블 미존재)

**Priority:** High
**Added:** 2026-04-14 (QA 중 발견)
**File:** `apps/api/db/migrations/000024_add_bot_run_indexes.up.sql`

마이그레이션 24번이 `bot_pioneer_runs` 테이블 인덱스를 생성하려 하나, 해당 테이블 생성 마이그레이션이 없음. 24번 이후 마이그레이션(25번 `sample_url` 추가 등)이 전부 블로킹됨.

## EKS + CI/CD 학습 트랙 — Phase 0: 로컬 k8s 위에서 raw YAML 작성

**Priority:** Medium
**Added:** 2026-05-14 (personal learning track)
**Depends on:** —

kind 또는 k3d 로 로컬 클러스터 1개 띄우고, `docker-compose.yml` 의 web/api/postgres/redis 를 helm 없이 raw `*.yaml` 5~6장으로 옮기기. 학습 목표: `Pod / Deployment / Service / StatefulSet / ConfigMap / Secret / PVC` 가 각각 왜 있는지 손으로 익히기.

산출물: `web-deployment.yaml` + `web-service.yaml`(ClusterIP), `api-deployment.yaml` + `api-service.yaml`, `postgres-statefulset.yaml` + headless Service + PVC, `redis-deployment.yaml` + `redis-service.yaml`, `secret.yaml`(DB 비밀번호) + `configmap.yaml`(env). 검증: `kubectl port-forward` 로 브라우저 접속, Pod 강제 종료 시 자동 복구, `kubectl exec` 로 Postgres 접속. 비용: 0원.

## EKS + CI/CD 학습 트랙 — Phase 1: helm/fugue 차트 완성

**Priority:** Medium
**Added:** 2026-05-14 (personal learning track)
**Depends on:** Phase 0
**File:** `helm/fugue/`

현재 `helm/fugue/templates/cronjob-bot.yaml` 만 있는 깡통 차트를 완성. 학습 목표: helm 의 `values`, 템플릿 헬퍼, `helm upgrade --install` 의 롤링/롤백 흐름.

산출물: `Chart.yaml`(metadata), `values.yaml`(image tag, replica, resource), `templates/_helpers.tpl`(`cronjob-bot.yaml` 이 이미 참조 중인 `fugue.fullname` / `fugue.labels` / `fugue.selectorLabels`), Phase 0 의 raw YAML 들을 `templates/` 로 이전 + 하드코딩을 `{{ .Values.* }}` 로 치환. Postgres·Redis 는 차트에 직접 포함(학습 목적, 나중에 RDS 분리 가능). 검증: `helm install fugue ./helm/fugue` 성공, `values.yaml` image tag 변경 후 `helm upgrade` 로 API Pod 만 롤링 업데이트, `helm rollback fugue 1` 로 이전 리비전 복귀. 비용: 0원(로컬).

## EKS + CI/CD 학습 트랙 — Phase 2: AWS 에 처음 EKS 올리기

**Priority:** Medium
**Added:** 2026-05-14 (personal learning track)
**Depends on:** Phase 1

실제로 EKS 클러스터 띄우고 차트 배포. 학습 목표: ECR 푸시, EKS 클러스터 생성, IAM/IRSA, kubeconfig. 사전: AWS 계정, `aws` CLI 로그인, `eksctl`/`kubectl`/`helm` 설치.

순서: (1) ECR repo 3개 콘솔에서 생성 — `fugue-api`, `fugue-web`, `fugue-bot`, (2) 로컬에서 `docker build` → `docker tag` → `aws ecr get-login-password | docker login` → `docker push`, (3) `eksctl create cluster --name fugue --nodes 1 --node-type t3.small --managed`(~15분), (4) `aws eks update-kubeconfig --name fugue` → `kubectl get nodes` 확인, (5) `values.yaml` 의 image repo 를 ECR 주소로 변경 후 `helm install`, (6) IRSA — 봇 ServiceAccount 단위로 S3 권한 부여(`eksctl create iamserviceaccount …`). 외부 접근: 처음은 `kubectl port-forward`, 익숙해지면 Ingress + AWS Load Balancer Controller 로 ALB. 검증: ALB DNS 로 브라우저 접속 시 API 가 in-cluster Postgres 와 통신.

⚠️ 비용 관리: 세션 끝마다 `eksctl delete cluster fugue` 필수. 안 그러면 컨트롤 플레인 ~$73/월 + NAT Gateway ~$32/월 고정 지출. 재생성 15분 소요.

## EKS + CI/CD 학습 트랙 — Phase 3: GitHub Actions CI/CD 자동화

**Priority:** Medium
**Added:** 2026-05-14 (personal learning track)
**Depends on:** Phase 2

main 브랜치 push → 이미지 빌드 → ECR 푸시 → EKS 자동 배포. 학습 목표: OIDC 기반 키리스 인증, 이미지 태그 = git SHA, 자동 `helm upgrade`.

순서: (1) AWS IAM 에 GitHub OIDC provider 등록(`token.actions.githubusercontent.com`). 장기 access key 없이 임시 자격증명으로 푸시·배포. (2) `.github/workflows/deploy.yml` 작성 — `actions/checkout` → `aws-actions/configure-aws-credentials`(`role-to-assume`) → `docker buildx` 멀티 아키 빌드 → ECR 푸시(태그 = `${{ github.sha }}`) → `aws eks update-kubeconfig` → `helm upgrade --install fugue ./helm/fugue --set image.tag=${{ github.sha }}`. (3) main 트리거로 시작, 익숙해지면 staging/prod 분기. 검증: 코드 한 줄 변경 후 push 했을 때 사람 손 없이 ECR 새 이미지 업로드 + EKS Deployment 가 새 SHA 로 롤링.

## EKS + CI/CD 학습 트랙 — Phase 4: Terraform 으로 클러스터 IaC 화

**Priority:** Medium
**Added:** 2026-05-14 (personal learning track)
**Depends on:** Phase 3

`eksctl` 의 마법을 vpc/subnet/iam/eks 모듈로 풀어 코드화. 학습 목표: 클러스터의 재현성.

산출물: `terraform/` 디렉터리 신규 생성(현재 레포에 없음), 모듈은 `terraform-aws-modules/vpc/aws` + `terraform-aws-modules/eks/aws` 두 개로 시작(VPC 직접 작성 비추천), 백엔드는 S3 + DynamoDB lock(로컬 state 금지 — 한 번 날리면 복구 불가). 검증: `terraform destroy` 로 전부 삭제 후 `terraform apply` 로 동일한 클러스터 재생성, Phase 3 GitHub Actions 가 그대로 동작. 참고: `docs/architecture.md:402` 의 "Terraform + EKS + ArgoCD" 목표와 정렬됨.

## EKS + CI/CD 학습 트랙 — Phase 5: (선택) ArgoCD GitOps 분리

**Priority:** Low
**Added:** 2026-05-14 (personal learning track)
**Depends on:** Phase 4

이미지 빌드와 배포의 책임 분리. CD 가 클러스터 안에서 git 을 폴링하는 모델. 학습 목표: GitOps 패턴.

순서: (1) 클러스터에 ArgoCD 설치(helm chart 한 줄), (2) 별도 repo 또는 같은 repo 의 `deploy/` 폴더에 환경별 `values.yaml` 만 관리, (3) GitHub Actions 의 마지막 단계가 `helm upgrade` 가 아니라 deploy repo 의 `image.tag` 만 커밋하는 형태로 변경, (4) ArgoCD 가 git 변경 감지 후 자동 sync. 검증: ArgoCD UI 에서 drift 시각화, `kubectl edit` 로 manifest 직접 변경했을 때 ArgoCD 가 원상복구. 전제: Phase 0~4 가 안정적으로 동작한 이후에만 진입 권장.
