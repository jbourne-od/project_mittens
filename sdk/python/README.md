# Project Mittens Python SDK

A lightweight, **zero-dependency** Python client SDK for **Project Mittens**, the high-performance Go carrier optimization and MOMDP matching engine.

Built exclusively with standard library `dataclasses` (Python 3.10+) and `urllib.request`—**zero Pydantic, zero external dependencies**.

---

## Installation

```bash
cd sdk/python
pip install -e .
```

---

## Quickstart

```python
from mittens import MittensClient

# Initialize client
client = MittensClient(base_url="http://localhost:8080")

# 1. Load an authoritative benchmark scenario
scenario = client.get_scenario("17_geoconstraints")

# 2. Run single-epoch CFA optimizer with POMDP N=1 competitor dynamics
result = client.optimize(
    drivers=scenario.drivers,
    loads=scenario.loads,
    policy_class="CFA",
    competitor_scale="N1",
)

print(f"Matched {result.match_count} loads | Net Contribution: ${result.total_net_contribution:,.2f}")
for match in result.matches:
    print(f" - Driver {match.driver_id} -> Load {match.load_id} (${match.estimated_contribution:,.2f})")

# 3. Explain decision economic attribution waterfall
explanation = client.explain_decision(result.decision_id)
print(f"Evaluated candidates: {explanation.evaluated_arcs_count}")
print(explanation.markdown_summary)

# 4. Cryptographic offline deterministic replay audit
replay = client.replay_decision(result.decision_id)
assert replay.status == "BIT_EXACT_MATCH"
assert replay.drift_amount == 0.0
print("Bit-exact state and action hashes verified!")

# 5. Verify SHA-256 Merkle chain integrity
chain = client.verify_merkle_chain(result.run_id)
assert chain.is_valid
print(f"Sealed Merkle Hash: {chain.latest_record_hash}")
```

---

## Features

* **Strict Type Safety:** Uses `@dataclass(slots=True)` models strictly mapped to `api/openapi.yaml`.
* **Zero Dependencies:** Runs on standard Python 3.10+ standard library.
* **Full Engine Coverage:** All 14 endpoints (Optimization, Simulation, Scenarios, Explainability, Replay, Merkle Chain, and Repositioning).
* **Mathematical Provenance:** First-class support for retrieving decision alternatives and cryptographic replay scorecards.
