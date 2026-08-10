# WBY Web

Astro SSR frontend for fixed Finnish city weather pages. The server runtime signs requests to the Go API; browser code never receives API signing secrets.

## Commands

```bash
npm install
npm run dev
npm test
npm run check
npm run build
```

## Environment

Copy `.env.example` to `.env` for local dev — `astro.config.mjs` loads it into
`process.env`. Real environment variables take precedence over the file, so
Docker and production are unaffected.

Required:

- `WBY_API_CLIENT_ID`
- `WBY_API_CLIENT_SECRET`

Optional:

- `WBY_API_BASE_URL` defaults to `http://localhost:8080`
- `WBY_CACHE_TTL_SECONDS` defaults to `900`
- `WBY_CACHE_STALE_SECONDS` defaults to `3600`

The web client ID and secret must match an entry in the backend `CLIENT_SECRETS` value.
