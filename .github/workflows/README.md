# CI/CD

Two workflows.

## `ci.yml` — always on

Runs on every pull request and on pushes to the default branch: `gofmt` check, `go vet`, and the full
`go test ./...` (the integration/e2e layers use the runner's Docker to spin up a throwaway Postgres via
testcontainers). Needs no cloud credentials — it works the moment the repo is on GitHub.

## `deploy.yml` — off until you enable it

Builds an immutable image, pushes it to **Amazon ECR**, and rolls it out on the target **EC2** host over
SSH. The server never builds or sees source; it pulls a fixed image tag, so deploys are reproducible and
a rollback is just deploying an earlier commit sha.

The branch picks the environment: push to `staging` deploys staging, push to `production` deploys
production. Each maps to a GitHub Environment of the same name.

### It is skipped until configured

The deploy job is guarded by `if: vars.DEPLOY_ENABLED == 'true'`. Until that repository variable is set,
pushing to `staging`/`production` does nothing — no failing runs while there is no AWS account yet.

### To activate (once AWS exists)

**Repository → Settings → Secrets and variables → Actions**

Variables:

| Name             | Example                    | What it is                                  |
| ---------------- | -------------------------- | ------------------------------------------- |
| `DEPLOY_ENABLED` | `true`                     | Master switch. Nothing deploys until `true` |
| `AWS_REGION`     | `us-east-1`                | Region of the ECR repo and the EC2 host     |
| `ECR_REPOSITORY` | `news-feed`                | ECR repository name (without the registry)  |

Repository secret:

| Name           | What it is                                                                    |
| -------------- | ----------------------------------------------------------------------------- |
| `AWS_ROLE_ARN` | IAM role trusted for GitHub OIDC (no static access keys). Push to ECR + read. |

Per-environment secrets (define under **Environments → staging** and **Environments → production**):

| Name          | What it is                                    |
| ------------- | --------------------------------------------- |
| `EC2_HOST`    | Public host/IP of that environment's instance |
| `EC2_USER`    | SSH user (e.g. `ubuntu`, `ec2-user`)          |
| `EC2_SSH_KEY` | Private key for that user (PEM)               |

### What the target host needs (provisioned once, not by CI)

Under `/opt/news-feed`:

- `docker-compose.yaml` + `docker-compose.<environment>.yaml` (the overlays from this repo).
- `.env.<environment>` holding the real config **and secrets** for that environment (copied from the
  `.env.<environment>.example` template). This file stays on the box; it is never committed.
- Docker + the Compose plugin installed.

The deploy step only sets `API_IMAGE` to the new tag, `pull`s it, and `up -d`. Everything else already
lives on the box.

### Notes

- **Single instance:** the discovery CRON is in-process. Production must run one API replica (or set
  `RSS_FEED_CRON_ACTIVE=false` on all but one). Do not scale the `api` service blindly.
- **RDS vs container Postgres:** the production overlay uses RDS by default (no Postgres container). The
  choice lives entirely in `.env.production`'s `DATABASE_URL` — see `docker-compose.production.yaml`.
