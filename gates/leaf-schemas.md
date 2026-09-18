# Gates: versioned proof artifact schemas

Scope: Keep the version 1 explicit graph contract and add a coordinated version 2 symbolic circuit contract.

- [x] G1: Every schema is valid JSON Schema Draft 2020-12.
  CHECK: python3 schemas/validate_contracts.py --schemas-only
  EXPECT: schemas: valid
  EVIDENCE: schemas: valid

- [x] G2: Version 1 accepts explicit graphs and rejects sequential circuits.
  CHECK: python3 schemas/validate_contracts.py --fixtures v1
  EXPECT: fixtures v1: 2 passed
  EVIDENCE: fixtures v1: 2 passed

- [x] G3: Version 2 model fixtures decide exact ports, tagged effects, and circuit roots.
  CHECK: python3 schemas/validate_contracts.py --fixtures v2-model
  EXPECT: fixtures v2-model: 3 passed
  EVIDENCE: fixtures v2-model: 3 passed

- [x] G4: Version 2 coverage fixtures decide both reconciliation directions and retained query mappings without transition IDs.
  CHECK: python3 schemas/validate_contracts.py --fixtures v2-coverage
  EXPECT: fixtures v2-coverage: 3 passed
  EVIDENCE: fixtures v2-coverage: 3 passed

- [x] G5: Version 2 requirement fixtures decide finite monitors, observation functions, roots, fairness, and provenance.
  CHECK: python3 schemas/validate_contracts.py --fixtures v2-requirement
  EXPECT: fixtures v2-requirement: 2 passed
  EVIDENCE: fixtures v2-requirement: 2 passed

- [x] G6: Version 2 results have disjoint branches, symbolic witnesses, and no raw proof-proposal path to PROVED.
  CHECK: python3 schemas/validate_contracts.py --fixtures v2-result
  EXPECT: fixtures v2-result: 10 passed
  EVIDENCE: fixtures v2-result: 10 passed

- [x] G7: Raw proposals and validated envelopes are distinct; model IR, missing obligations, and caller acceptance booleans are rejected.
  CHECK: python3 schemas/validate_contracts.py --fixtures v2-certificate
  EXPECT: fixtures v2-certificate: 15 passed
  EVIDENCE: fixtures v2-certificate: 15 passed

- [x] G8: Every canonical valid and adversarial fixture has the expected decision.
  CHECK: python3 schemas/validate_contracts.py --fixtures all
  EXPECT: fixtures all: 35 passed
  EVIDENCE: fixtures all: 35 passed

- [x] G9: The version 2 schemas do not require explicit state or transition identifiers.
  CHECK: if rg -n '"(state_ids|transition_ids|transition_id|source_state_id|target_state_id)"' schemas/v2; then exit 1; else echo 'symbolic identifiers: clean'; fi
  EXPECT: symbolic identifiers: clean
  EVIDENCE: symbolic identifiers: clean

- [x] G10: The schema validator runs and stays within the workspace file-size limit.
  CHECK: python3 schemas/validate_contracts.py --schemas-only > /dev/null && wc -l schemas/validate_contracts.py
  EXPECT: 75 schemas/validate_contracts.py
  EVIDENCE: 75 schemas/validate_contracts.py
