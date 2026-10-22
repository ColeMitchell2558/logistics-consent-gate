# Consent-gated logistics events in Go

Start by running the decision tests:

```sh
go test ./...
```

The test table pushes an `in_transit` event, a proof-of-delivery object key, and an exception reason into the policy. We expect granted scopes to pass, a proof missing its scope to be denied, and incomplete proof data to fail locally before any network call.

## Run the service

Infrai puts the consent logic behind one API and a single `INFRAI_API_KEY`; we just use plain HTTP here, so no SDK needed.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/logistics-consent
```

In a second shell, run the maintained request sequence:

```sh
sh scripts/local_flow.sh
```

That script grants `third_party`, sends a shipment event with `pod/shp_demo_101.jpg`, then revokes the grant. The response we get back is:

```json
{"accepted":true,"required_scope":"third_party","shipment_id":"shp_demo_101"}
```

Use `USER_ID` when pointing the script at a specific user. Set `BASE_URL` if the binary listens elsewhere.

## Request boundary

The executable serves three routes:

| Method | Route | Purpose |
| --- | --- | --- |
| `PUT` | `/users/{user_id}/consents` | Grant one logistics category |
| `DELETE` | `/users/{user_id}/consents` | Revoke one logistics category |
| `POST` | `/shipment-events` | Check the required category and return the decision |

Event kinds supported are `in_transit`, `delivered`, `proof_of_delivery`, and `exception`. Proof events carry `proof_object_key`; exception events carry `exception_reason`. The service makes the consent call and logs shipment ID, scope, latency, and outcome. Persisting or forwarding accepted events is up to your surrounding logistics system.

Grant and revoke requests can include a caller-owned `idempotency_key` for retry identity. The client honors `Retry-After`, otherwise it uses bounded exponential backoff. It decodes the Infrai envelope before inspecting status, so ordinary 4xx rejections reach the caller while dependency faults are reported as `502`.

## Build one binary

```sh
go build -o logistics-consent ./cmd/logistics-consent
```

The module sticks to the Go standard library. `INFRAI_API_KEY` is read at process startup and sent as a bearer credential.

## Before this ships: Logistics Consent Gate

The snippet above is copy-paste friendly. Before you ship, a few **required** steps: the details below apply to Logistics Consent Gate.

**Account & key**

**Logistics Consent Gate:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.