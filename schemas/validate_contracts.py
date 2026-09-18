# Validate Hyperray schemas and canonical decision fixtures.
# A fixture name declares whether validation must accept or reject the data.

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

from jsonschema import Draft202012Validator
from referencing import Registry, Resource

SCHEMA_ROOT = Path(__file__).parent
FIXTURE_ROOT = SCHEMA_ROOT / "fixtures"


def load_schemas() -> tuple[dict[Path, dict[str, Any]], Registry[Any]]:
    schemas: dict[Path, dict[str, Any]] = {}
    registry: Registry[Any] = Registry()
    for path in sorted(SCHEMA_ROOT.rglob("*.schema.json")):
        schema = json.loads(path.read_text(encoding="utf-8"))
        Draft202012Validator.check_schema(schema)
        schemas[path] = schema
        resource = Resource.from_contents(schema)
        registry = registry.with_resource(schema["$id"], resource)
    return schemas, registry


def fixture_matches(
    fixture: Path,
    schemas: dict[Path, dict[str, Any]],
    registry: Registry[Any],
) -> tuple[bool, str]:
    instance = json.loads(fixture.read_text(encoding="utf-8"))
    schema_name = f"{fixture.name.split('.')[0]}.schema.json"
    version_dir = "v2" if fixture.parent.name.startswith("v2-") else ""
    schema = schemas[SCHEMA_ROOT / version_dir / schema_name]
    errors = list(Draft202012Validator(schema, registry=registry).iter_errors(instance))
    expected_valid = ".valid.json" in fixture.name
    if expected_valid == (not errors):
        return True, ""
    decision = "accepted" if not errors else f"rejected: {errors[0].message}"
    return False, f"{fixture}: expected {'accept' if expected_valid else 'reject'}, {decision}"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--schemas-only", action="store_true")
    parser.add_argument("--fixtures")
    arguments = parser.parse_args()
    schemas, registry = load_schemas()
    if arguments.schemas_only:
        print("schemas: valid")
        return 0
    if not arguments.fixtures:
        parser.error("one of --schemas-only or --fixtures is required")
    if arguments.fixtures == "all":
        fixtures = sorted(FIXTURE_ROOT.rglob("*.json"))
    else:
        fixtures = sorted((FIXTURE_ROOT / arguments.fixtures).glob("*.json"))
    failures: list[str] = []
    for fixture in fixtures:
        matches, message = fixture_matches(fixture, schemas, registry)
        if not matches:
            failures.append(message)
    if failures:
        print("\n".join(failures))
        return 1
    print(f"fixtures {arguments.fixtures}: {len(fixtures)} passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
