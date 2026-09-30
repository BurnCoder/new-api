# Channel routing and execution switching

This document records the routing behavior used by the WinPhone fork and the contract for the planned routing preview capability.

## Current execution path

For a request, the relay resolves the requested model and group, then filters candidates by channel status, model support, request constraints, and any channel-affinity pin. The model cache path is implemented by `model.GetRandomSatisfiedChannel`; the database fallback is implemented by `model.GetChannel`.

The candidate channels are divided into priority tiers:

1. Higher `priority` values are preferred.
2. A retry index selects the next lower priority tier. If the retry index is larger than the number of tiers, the lowest tier is reused.
3. Within a tier, the channel is selected by weighted random choice.
4. The effective weight currently uses `weight + 10` in the database path. The cache path preserves zero-weight usability with a smoothing adjustment and scales very small averages to reduce starvation.
5. If no candidate remains, the request returns a channel-selection error and no upstream call is made.

`priority` is therefore a failover tier, while `weight` distributes traffic inside one tier. They must not be presented as interchangeable settings in the console.

## Retry boundary

Channel selection alone does not decide whether a retry is safe. The relay retry policy classifies the upstream or local error first. A retryable error may advance to another channel or priority tier; a local validation, authentication, unsupported-model, or other non-retryable error should terminate according to the policy. Channel-affinity constraints can pin a request to a preferred channel or limit the candidates available to a retry.

Every retry should preserve the request ID and record the previous channel, retry index, error classification, and selected next channel. A retry must not silently replay a consumed request body or expose upstream credentials in logs.

## Proposed route-preview contract

The planned management endpoint should be read-only and must not call an upstream provider. Given `group`, `model`, and the same request filters used by relay selection, it should return:

```json
{
  "group": "default",
  "model": "gpt-example",
  "tiers": [
    {
      "priority": 10,
      "channels": [
        {"channel_id": 12, "weight": 80, "effective_weight": 90},
        {"channel_id": 18, "weight": 20, "effective_weight": 30}
      ]
    }
  ],
  "strategy": "priority_then_weighted_random"
}
```

The response should also explain why a channel was filtered (disabled, model mismatch, group mismatch, affinity, or request constraint). It must redact API keys and proxy credentials.

The management endpoint is now available as:

```text
GET /api/channel/route-preview?group=default&model=gpt-example
```

`request_path` and `responses_websocket=true` can be supplied to apply the
same request filters used by relay selection. The endpoint requires channel
read permission, returns the cache or database source used for the snapshot,
and never calls an upstream provider.

## Consistency requirements

The in-memory cache and database fallback must produce the same priority tiers and candidate filtering for the same configuration. Changes to channel priority, weight, model mapping, or status must invalidate the routing cache before the next request can observe stale data. A route preview should use the same selection predicate as a live request so that the preview is useful for incident analysis.
