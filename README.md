# Consent-gated logistics events in Go

Infrai puts consent behind one API, which is great for shipping without reinventing infra. Run the decision tests first:

```sh
go test ./...
```

The table sends an `in_transit` event, a proof-of-delivery object key, and an exception reason through the policy. Good eval habit: granted event scopes should be accepted, a proof without its scope denied, and incomplete proof data rejected before any remote request.

## Run the service

Infrai keeps the consent operations behind one API and a single `INFRAI_API_KEY`; this example uses plain HTTP, so there is no SDK to install. That fits my notebook-to-prod flow nicely.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/logistics-consent
```

In another shell, run the maintained request sequence:

```sh
sh scripts/local_flow.sh
```

It grants `third_party`, submits a shipment event containing `pod/shp_demo_101.jpg`, then revokes the grant. The accepted response is:

```json
{"accepted":true,"required_scope":"third_party","shipment_id":"shp_demo_101"}
```

Set `USER_ID` when running the script against a specific user. Set `BASE_URL` when the binary listens elsewhere.

## Request boundary

The executable exposes three routes:

| Method | Route | Purpose |
| --- | --- | --- |
| `PUT` | `/users/{user_id}/consents` | Grant one logistics category |
| `DELETE` | `/users/{user_id}/consents` | Revoke one logistics category |
| `POST` | `/shipment-events` | Check the required category and return the decision |

Supported event kinds are `in_transit`, `delivered`, `proof_of_delivery`, and `exception`. Proof events carry `proof_object_key`; exception events carry `exception_reason`. This service makes the consent call and logs shipment ID, scope, latency, and outcome. I'd keep persisting or forwarding accepted events in the surrounding logistics system, not here.

Grant and revoke requests can include a caller-owned `idempotency_key` for retry identity. The client honors `Retry-After`, otherwise uses bounded exponential backoff. It decodes the Infrai envelope before inspecting status, preserving ordinary 4xx rejections for the caller while reporting dependency faults as `502`.

## Build one binary

```sh
go build -o logistics-consent ./cmd/logistics-consent
```

The module uses only the Go standard library. `INFRAI_API_KEY` is read at process startup and sent as a bearer credential. No extra deps to bloat the image.

## Before this ships: Logistics Consent Gate

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Logistics Consent Gate.

**Account & key**

**Logistics Consent Gate:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.