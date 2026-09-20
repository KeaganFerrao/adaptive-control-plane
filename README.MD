# Adaptive Control Plane

A general-purpose, policy-driven control plane framework written in Go.

At its core it is a **signal → condition → action** loop: it reads metrics from a source, checks them against declarative YAML policies, runs actions when a policy fires, and reverts them once the system recovers. Every part of that loop is behind an interface, so the framework isn't tied to any one system. Any signal you can scrape or query (from Prometheus or any other tool) can drive any reaction you can write in code.

The first implementation targets **OpenTelemetry Collectors**. It reads collector metrics from Prometheus and reconfigures the collectors at runtime over [OpAMP](https://opentelemetry.io/docs/specs/opamp/). For example, it can switch on tail sampling or tighten a memory limit when a collector is under memory pressure, then revert the change once things recover. The same pattern applies to other systems: scaling a database pool, adjusting rate limits on a proxy, toggling a feature flag, or calling a webhook. See [Extending the framework](#extending-the-framework).

```
            ┌───────────────┐   PromQL    ┌──────────────────────┐
            │  Prometheus   │◄────────────│   Metric Collector   │
            └───────▲───────┘             └──────────┬───────────┘
                    │ scrape                         │ snapshots
                    │                                ▼
┌───────────────────┴──────┐              ┌──────────────────────┐   YAML   ┌────────────┐
│  OpenTelemetry           │              │  Control Plane loop  │◄─────────│  policies/ │
│  Collectors (OpAMP agent)│              │  (one goroutine per  │          └────────────┘
└───────────────────▲──────┘              │   policy, every 10s) │
                    │                     └──────────┬───────────┘
                    │ remote config                  │ evaluate → run actions
                    │ (wss :4320/v1/opamp)           ▼
            ┌───────┴───────┐             ┌──────────────────────┐
            │  OpAMP Server │◄────────────│   Action Registry    │
            └───────────────┘             └──────────────────────┘
                    ▲
                    │ http :4321
              Admin UI (agents, config, certs)
```

## How it works

1. **Load policies.** At startup, every `.yaml`/`.yml` file in `POLICY_FILE_PATH` is parsed into a `PolicyDefinition`. Each action name is resolved to a handler in the action registry. The metrics each policy needs are worked out per component, and the components are looked up by name in Postgres.
2. **Persist policies.** Each policy is saved to the `policies` table.
3. **Run the control loop.** If `START_CONTROL_PLANE_LOOP=true`, one goroutine per policy runs every 10 seconds and:
   - queries Prometheus for the metrics the policy needs (`<metric>{collector_name="<component>"}`)
   - evaluates the `when` (trigger) and `recover.when` expressions
   - runs the `then` actions once the trigger condition has held for longer than `for`, and marks the policy `triggered`
   - runs the `recover.then` actions once the recovery condition has held for longer than `recover.for`, and returns the policy to `normal`
4. **Push config through OpAMP.** Actions send a remote config fragment to every connected agent through the embedded OpAMP server. Each action waits up to 10 s for all agents to report a status update.

### Policy state machine

```
 normal ──(trigger true for > `for`)──► triggered ──(recovery true for > `recover.for`)──► normal
```

If any action in a batch returns a failure, the status is not advanced, so the actions are retried on the next tick.

## Project layout

```
cmd/control-plane/          Entry point (wires everything together)
internal/
  config/                   .env / environment variable loading
  database/                 pgx connection pool
  repository/               Postgres repositories for policies and components
  models/                   Core types and interfaces (Policy, Condition, Action, ...)
  policy_manager/           YAML policy loader
  metric_collector/         Prometheus-backed MetricCollector
  policy_evaluator/         Evaluates and/or/condition expression trees
  control_plane/            Per-policy control loop and trigger/recovery logic
  action_runner/            Executes a list of policy actions
  actions_registry/         Built-in actions (sampling, memory limit)
  opamp_server/             OpAMP server, agent store, admin UI, TLS certs
policies/                   Example policy definitions
```

## Requirements

These are the requirements for the bundled OpenTelemetry setup. A different signal source or set of actions will have its own.

- Go **1.26+** (see `go.mod`)
- PostgreSQL
- Prometheus scraping your collectors' internal telemetry, with a `collector_name` label matching the component name
- One or more OpenTelemetry Collectors running an OpAMP client (for example the [OpAMP supervisor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/opampsupervisor)) that accepts remote configuration

## Getting started

### 1. Configure the environment

Create a `.env` file in the directory you run the binary from. It is required, because startup fails if `.env` is missing.

```dotenv
DATABASE_URL=postgres://postgres:postgres@localhost:5432/control_plane?sslmode=disable
POLICY_FILE_PATH=./policies
PROMETHEUS_ENDPOINT=http://localhost:9090
START_CONTROL_PLANE_LOOP=true
```

| Variable | Description |
|---|---|
| `DATABASE_URL` | Postgres connection string (pgx format) |
| `POLICY_FILE_PATH` | **Directory** containing policy YAML files |
| `PROMETHEUS_ENDPOINT` | Base URL of the Prometheus HTTP API |
| `START_CONTROL_PLANE_LOOP` | `true` to run the evaluation loop. Any other value starts only the OpAMP server and UI |

### 2. Create the database schema

The repo does not ship migrations yet. The schema below matches the columns the repositories read and write:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE components (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL,          -- otel-collector | clickhouse | postgres | nginx
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE policies (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_name             TEXT NOT NULL UNIQUE,
    status                  TEXT NOT NULL,   -- normal | pending_trigger | triggered | pending_recovery
    status_updated_at       TIMESTAMPTZ,
    condition_first_seen_at TIMESTAMPTZ,
    recovery_first_seen_at  TIMESTAMPTZ,
    triggered_at            TIMESTAMPTZ,
    recovered_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ
);
```

Register every component that a policy's `scope.component` refers to:

```sql
INSERT INTO components (name, type) VALUES ('otel-collector-1', 'otel-collector');
```

### 3. Build and run

```bash
go build -o control-plane ./cmd/control-plane
./control-plane
```

Or simply:

```bash
go run ./cmd/control-plane
```

Once it is running:

| Service | Address |
|---|---|
| OpAMP server (TLS) | `wss://0.0.0.0:4320/v1/opamp` |
| Admin UI | <http://localhost:4321> |

Press `Ctrl+C` to stop.

## Writing policies

Policies are YAML files in `POLICY_FILE_PATH`. Here is the bundled example, `policies/collector_cpu.yaml`:

```yaml
name: high_mem_and_refused_spans

when:
  and:
    - condition:
        metric: otelcol_process_runtime_heap_alloc_bytes
        operator: ">="
        threshold: 60000000        # 60 MB
        threshold_type: absolute
        scope:
          component: otel-collector-1
          instance: collector-1
    - condition:
        metric: otelcol_processor_memory_limiter_refused_spans
        operator: ">"
        threshold: 100
        threshold_type: absolute
        scope:
          component: otel-collector-1
          instance: collector-2

for: 30s

then:
  - action: set_collector_sampling
    parameters:
      value: 1

recover:
  when:
    condition:
      metric: otelcol_process_runtime_heap_alloc_bytes
      operator: "<"
      threshold: 60000000
      threshold_type: absolute
      scope:
        component: otel-collector-1
  for: 30s
  then:
    - action: set_collector_sampling
      parameters:
        value: 0
```

### Reference

| Field | Description |
|---|---|
| `name` | Unique policy name (stored in Postgres) |
| `when` | Trigger expression: `and: [...]`, `or: [...]`, or a single `condition` |
| `for` | How long the trigger must hold before actions run (Go duration, e.g. `30s`, `5m`) |
| `then` | List of actions to run on trigger |
| `recover.when` / `recover.for` / `recover.then` | Same shape, used to revert |

**Condition fields**

| Field | Values |
|---|---|
| `metric` | Prometheus metric name |
| `operator` | `>`, `>=`, `<`, `<=`, `==`, `!=` |
| `threshold` | Number |
| `threshold_type` | `absolute` (`percentage` is reserved for v2) |
| `scope.component` | Required. Must exist in the `components` table |
| `scope.instance` | Optional. Selects a single instance (reserved, not yet used in queries) |
| `aggregation.type` | `avg`, `min`, `max`, `sum` (reserved for v2; mutually exclusive with `scope.instance`) |

## Built-in actions

| Action | Parameters | Effect on every connected agent |
|---|---|---|
| `set_collector_sampling` | `value: 1` or `0` | `1` sets the traces pipeline to `[memory_limiter, tail_sampling, batch]`. `0` sets it to `[memory_limiter, batch]` |
| `set_collector_mem_limit` | `limit_mib: <int>` | Sets `processors.memory_limiter.limit_mib` |

The collector's base configuration must already define the `memory_limiter`, `tail_sampling` and `batch` processors that these fragments refer to.

## Extending the framework

OpenTelemetry and OpAMP are just the first plug-ins. The control loop in `internal/control_plane` only talks to interfaces defined in `internal/models`, so you can swap any piece without touching the trigger/recovery logic:

| Extension point | Interface | Shipped implementation | Examples of what you could plug in |
|---|---|---|---|
| **Signal source** | `MetricCollector` | Prometheus (PromQL) | Datadog, CloudWatch, InfluxDB, VictoriaMetrics, a SQL query, a health-check endpoint |
| **Reaction** | `ActionHandler` | OpAMP config push to collectors | Kubernetes scaling, Nginx/Envoy config reload, database tuning, feature flags, webhooks, Slack/PagerDuty alerts |
| **Policy source** | `PolicyManager` | YAML files on disk | A database, an HTTP API, a Kubernetes CRD |
| **Policy evaluation** | `PolicyEvaluator` | Threshold expressions with `and`/`or` | Rate-of-change, anomaly detection, percentage thresholds |
| **Targets** | `ComponentType` | `otel-collector` (`clickhouse`, `postgres` and `nginx` are already declared) | Any component you register in the `components` table |

```go
// Signal source: return the current value of each metric for each component.
type MetricCollector interface {
    Collect(ctx context.Context, metrics ComponentMetrics) (*ComponentMetricSnapshots, error)
}

// Reaction: do something to the outside world.
type ActionHandler interface {
    Execute(ctx context.Context, params ActionParams) (ActionResult, error)
}
```

### Adding an action

1. Implement `models.ActionHandler`. It can do anything: call an API, run a command, push config, or send a notification.
2. Add an `ActionName` constant in `internal/models/action.go`.
3. Register the handler in `internal/actions_registry/registry.go`.
4. Reference it by name in a policy's `then` or `recover.then`.

### Adding a signal source

1. Implement `models.MetricCollector` for your backend. It must return a `MetricSnapshot` (metric name → value) per component.
2. Wire it into `cmd/control-plane/main.go` in place of (or alongside) `NewPrometheusMetricCollector`.

Policies don't need to change. They refer to metric names and components, not to a particular backend.

## Admin UI

The UI on port `4321` comes from the OpAMP Go reference server. It lets you:

- list connected agents with their health, version and effective config
- set a custom config for an individual agent
- rotate an agent's client certificate
- offer new OpAMP connection settings
- send and view custom messages

## TLS certificates

`internal/opamp_server/certs/` contains a development CA and server certificate. They are embedded into the binary with `go:embed`. To regenerate them:

```bash
cd internal/opamp_server/certs
./clear.sh
./generate.sh
```

> ⚠️ The private keys in this directory are for **local development only**. Replace them, and keep real keys out of version control, before deploying anywhere.

## Project status

This is an early-stage project. Known gaps:

- **Expression evaluation:** the `and`/`or` evaluator currently returns the result of the first child only, and `!=` behaves like `==`.
- **Policy validation:** `validateDefinition` is a stub, so malformed policies are not rejected yet.
- **Scope:** `scope.instance`, `aggregation` and `threshold_type: percentage` are parsed but not yet applied.
- **Persistence:** policy state transitions are kept in memory and are not written back to Postgres. There are no migrations.
- **Targeting:** actions apply to all connected agents rather than only to the agents in the policy's scope.
- **Configuration:** the loop interval (10 s), action timeout (10 s) and ports are hard-coded.
- **Tests:** there is no test suite yet.

## Built with

- [opamp-go](https://github.com/open-telemetry/opamp-go): OpAMP server
- [prometheus/client_golang](https://github.com/prometheus/client_golang): Prometheus API client
- [pgx](https://github.com/jackc/pgx): PostgreSQL driver
- [yaml.v3](https://gopkg.in/yaml.v3): policy parsing
- [godotenv](https://github.com/joho/godotenv): `.env` loading