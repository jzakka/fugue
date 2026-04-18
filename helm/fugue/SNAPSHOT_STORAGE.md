# Pioneer Snapshot Storage — Infra Notes

Source of truth for the object-storage side of
`openspec/changes/pioneer-snapshot-storage` (now archived under
`openspec/specs/bot`). Consumed by a follow-up change
`harvester-snapshot-first-fetch`.

## Decision: single shared bucket + `snapshots/` prefix

The Pioneer raw-HTML snapshots live in the **same bucket as other media
uploads** (`S3_BUCKET`), segregated by the `snapshots/` key prefix. We chose
this over a dedicated bucket to reuse existing IAM/credentials and avoid a
new bucket to provision in every environment. The prefix gives us enough
isolation for a lifecycle rule and IAM scope.

If future traffic volume or retention requirements diverge sharply from
media assets, splitting into a dedicated bucket is a non-breaking change
(the key scheme `snapshots/<sha256>/<yyyymmdd>.html.gz` is identical).

## Required bucket configuration

1. **Lifecycle rule** — 365-day expiration, scoped to objects under the
   `snapshots/` prefix. Example (AWS S3 CLI):

   ```
   {
     "Rules": [
       {
         "ID": "pioneer-snapshots-ttl-365d",
         "Status": "Enabled",
         "Filter": { "Prefix": "snapshots/" },
         "Expiration": { "Days": 365 }
       }
     ]
   }
   ```

2. **Bucket encryption** — server-side encryption at rest (default bucket
   encryption). No application-level keys.

3. **Access** — bucket is private (no public ACLs). Pioneer writes via
   AWS SDK with the CronJob's IAM credentials.

## Required IAM policy

Grant the Pioneer service principal (the Kubernetes ServiceAccount that
runs `cronjob-bot`) the following, scoped to the prefix:

```
{
  "Effect": "Allow",
  "Action": ["s3:PutObject"],
  "Resource": "arn:aws:s3:::${S3_BUCKET}/snapshots/*"
}
```

Read permissions (`s3:GetObject`) are NOT required for this change —
Harvester's reuse of snapshots is the scope of
`harvester-snapshot-first-fetch` and will extend the policy there.

## Feature flag

`PIONEER_SNAPSHOT_ENABLED` (env var, default `false`). Set to `true` after
the bucket lifecycle + IAM policy are in place. Rolling back is a single
configmap/env change; stored objects age out naturally via the TTL rule.

## Key scheme (contract shared with Harvester)

```
snapshots/<sha256_hex_64>/<yyyymmdd>.html.gz
```

- `<sha256_hex_64>`: lowercase hex SHA-256 of the **normalized** URL.
- `<yyyymmdd>`: UTC date of the fetch.
- Content is a complete gzip stream (`Content-Encoding: gzip`,
  `Content-Type: text/html`).
- Same-day overwrites for the same URL follow S3 last-write-wins.
