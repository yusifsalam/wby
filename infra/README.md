# Shared VPS infrastructure

Host-wide services shared by every app on the VPS:

- **Caddy** — the single HTTPS ingress. Only this stack publishes ports 80/443.
- **Observability** (profile `logging`) — **Alloy** discovers every container via
  the Docker socket and ships logs to **Loki**; **Grafana** visualizes them
  (host-wide "Logs Overview" dashboard + Loki datasource auto-provisioned).

App stacks (e.g. `../server` for Weather) keep their own databases, secrets, and
volumes. Apps and Caddy meet on a shared external Docker network, `edge`.

## Layout

```
infra/
  docker-compose.yml          # caddy + loki/alloy/grafana (logging profile)
  .env.example                # copy to .env; Grafana admin password etc.
  conf/
    caddy/
      Caddyfile               # base: accesslog snippet + import sites/*.caddy
      sites/weather.caddy     # per-app routing fragments live here
    alloy/config.alloy
    loki/loki-config.yaml
    grafana/provisioning/...
```

## Bring up

```bash
# One-time: create the shared ingress network.
docker network create edge

cp .env.example .env            # set GRAFANA_ADMIN_PASSWORD

# Ingress only:
docker compose -p infra up -d caddy

# Ingress + logging:
docker compose -p infra --profile logging up -d
```

Grafana is at http://localhost:3000 directly, and behind Caddy at
`https://logs.yourweatherapp.fi` in production (set `GRAFANA_ROOT_URL`).

## Onboarding a new app

1. In the app's Compose, attach its public-facing service(s) to the external
   `edge` network with a **globally unique alias** (e.g. `myapp-web`). Keep
   internal services (db, workers) off `edge` on the app's private network.
2. The app publishes **no** public host ports — Caddy is the only ingress.
3. Add a routing fragment `conf/caddy/sites/<app>.caddy` pointing at the app's
   aliases.
4. Validate and reload Caddy without touching app stacks:

   ```bash
   docker compose -p infra exec caddy caddy validate --config /etc/caddy/Caddyfile
   docker compose -p infra exec caddy caddy reload --config /etc/caddy/Caddyfile
   ```

Alloy discovers new containers automatically; filter their logs in Grafana with
the dashboard's **Project** selector (the `compose_project` label).
