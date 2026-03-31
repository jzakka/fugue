# API 엔드포인트

## Auth

```
POST   /api/auth/google/callback
POST   /api/auth/twitter/callback
POST   /api/auth/discord/callback
POST   /api/auth/logout
```

## Creator

```
GET    /api/creators/:id
PUT    /api/creators/me
GET    /api/creators?roles=&page=&limit=
```

## Work

```
POST   /api/works
GET    /api/works/:id
DELETE /api/works/:id
GET    /api/works?field=&tags=&page=&limit=
```

## Recommendation

```
POST   /api/recommend
       body: { work_id, project_type }
```

## OG Metadata

```
POST   /api/og/fetch
       body: { url }
```
