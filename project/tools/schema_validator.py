#!/usr/bin/env python3
"""
schema_validator.py - JSON & YAML Schema Validation Engine.

Validates data files (JSON and YAML) against JSON Schema specifications
(Draft-07 / Draft 2020-12). Uses `jsonschema` when installed, with a
robust standalone pure-Python validator fallback requiring zero external dependencies.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple, Union

# Optional PyYAML support
try:
    import yaml
    HAS_YAML = True
except ImportError:
    HAS_YAML = False

# Optional jsonschema support
try:
    import jsonschema
    HAS_JSONSCHEMA = True
except ImportError:
    HAS_JSONSCHEMA = False


@dataclass
class ValidationError:
    """Represents a schema validation error at a specific document path."""
    path: str
    message: str
    keyword: str
    invalid_value: Any = None

    def __str__(self) -> str:
        pointer = self.path if self.path else "/"
        return f"[{self.keyword}] at {pointer}: {self.message}"


class PureJsonSchemaValidator:
    """
    Lightweight, dependency-free JSON Schema validator supporting
    core Draft-07 / Draft 2020-12 keywords.
    """

    def __init__(self, schema: Dict[str, Any], strict: bool = False) -> None:
        if not isinstance(schema, dict):
            raise TypeError(f"Schema must be a dictionary, got {type(schema).__name__}")
        self.schema = schema
        self.strict = strict

    def validate(self, instance: Any) -> List[ValidationError]:
        """Validate instance against schema and return list of errors."""
        errors: List[ValidationError] = []
        self._validate_node(instance, self.schema, "", errors)
        return errors

    def _validate_node(
        self, instance: Any, schema: Union[Dict[str, Any], bool], path: str, errors: List[ValidationError]
    ) -> None:
        if schema is True:
            return
        if schema is False:
            errors.append(ValidationError(path=path, message="Schema evaluates to False", keyword="false_schema", invalid_value=instance))
            return

        # 1. Type validation
        if "type" in schema:
            expected_type = schema["type"]
            if not self._check_type(instance, expected_type):
                errors.append(
                    ValidationError(
                        path=path,
                        message=f"Expected type '{expected_type}', got '{type(instance).__name__}'",
                        keyword="type",
                        invalid_value=instance,
                    )
                )
                return  # Stop deeper validation if basic type fails

        # 2. Const & Enum
        if "const" in schema:
            if instance != schema["const"]:
                errors.append(
                    ValidationError(
                        path=path,
                        message=f"Value must equal constant '{schema['const']}'",
                        keyword="const",
                        invalid_value=instance,
                    )
                )

        if "enum" in schema:
            if instance not in schema["enum"]:
                errors.append(
                    ValidationError(
                        path=path,
                        message=f"Value '{instance}' not in allowed enum: {schema['enum']}",
                        keyword="enum",
                        invalid_value=instance,
                    )
                )

        # 3. Numeric constraints
        if isinstance(instance, (int, float)) and not isinstance(instance, bool):
            if "minimum" in schema and instance < schema["minimum"]:
                errors.append(
                    ValidationError(path=path, message=f"{instance} is less than minimum {schema['minimum']}", keyword="minimum", invalid_value=instance)
                )
            if "maximum" in schema and instance > schema["maximum"]:
                errors.append(
                    ValidationError(path=path, message=f"{instance} is greater than maximum {schema['maximum']}", keyword="maximum", invalid_value=instance)
                )
            if "exclusiveMinimum" in schema and instance <= schema["exclusiveMinimum"]:
                errors.append(
                    ValidationError(path=path, message=f"{instance} is <= exclusiveMinimum {schema['exclusiveMinimum']}", keyword="exclusiveMinimum", invalid_value=instance)
                )
            if "exclusiveMaximum" in schema and instance >= schema["exclusiveMaximum"]:
                errors.append(
                    ValidationError(path=path, message=f"{instance} is >= exclusiveMaximum {schema['exclusiveMaximum']}", keyword="exclusiveMaximum", invalid_value=instance)
                )
            if "multipleOf" in schema:
                mult = schema["multipleOf"]
                if mult > 0 and (instance % mult) > 1e-9 and abs((instance % mult) - mult) > 1e-9:
                    errors.append(
                        ValidationError(path=path, message=f"{instance} is not a multiple of {mult}", keyword="multipleOf", invalid_value=instance)
                    )

        # 4. String constraints
        if isinstance(instance, str):
            if "minLength" in schema and len(instance) < schema["minLength"]:
                errors.append(
                    ValidationError(path=path, message=f"String length {len(instance)} < minLength {schema['minLength']}", keyword="minLength", invalid_value=instance)
                )
            if "maxLength" in schema and len(instance) > schema["maxLength"]:
                errors.append(
                    ValidationError(path=path, message=f"String length {len(instance)} > maxLength {schema['maxLength']}", keyword="maxLength", invalid_value=instance)
                )
            if "pattern" in schema:
                if not re.search(schema["pattern"], instance):
                    errors.append(
                        ValidationError(path=path, message=f"String does not match regex pattern '{schema['pattern']}'", keyword="pattern", invalid_value=instance)
                    )

        # 5. Array constraints
        if isinstance(instance, list):
            if "minItems" in schema and len(instance) < schema["minItems"]:
                errors.append(
                    ValidationError(path=path, message=f"Array items {len(instance)} < minItems {schema['minItems']}", keyword="minItems", invalid_value=instance)
                )
            if "maxItems" in schema and len(instance) > schema["maxItems"]:
                errors.append(
                    ValidationError(path=path, message=f"Array items {len(instance)} > maxItems {schema['maxItems']}", keyword="maxItems", invalid_value=instance)
                )
            if schema.get("uniqueItems") and len(instance) > len(set(json.dumps(x, sort_keys=True) for x in instance)):
                errors.append(
                    ValidationError(path=path, message="Array items must be unique", keyword="uniqueItems", invalid_value=instance)
                )
            if "items" in schema:
                item_schema = schema["items"]
                for i, item in enumerate(instance):
                    item_path = f"{path}/{i}"
                    self._validate_node(item, item_schema, item_path, errors)

        # 6. Object constraints
        if isinstance(instance, dict):
            if "minProperties" in schema and len(instance) < schema["minProperties"]:
                errors.append(
                    ValidationError(path=path, message=f"Properties count {len(instance)} < minProperties {schema['minProperties']}", keyword="minProperties", invalid_value=instance)
                )
            if "maxProperties" in schema and len(instance) > schema["maxProperties"]:
                errors.append(
                    ValidationError(path=path, message=f"Properties count {len(instance)} > maxProperties {schema['maxProperties']}", keyword="maxProperties", invalid_value=instance)
                )

            # Required properties
            if "required" in schema:
                for req in schema["required"]:
                    if req not in instance:
                        errors.append(
                            ValidationError(path=path, message=f"Missing required property '{req}'", keyword="required")
                        )

            # Validate properties
            props = schema.get("properties", {})
            for k, val in instance.items():
                prop_path = f"{path}/{k}"
                if k in props:
                    self._validate_node(val, props[k], prop_path, errors)
                else:
                    # Additional properties check
                    add_props = schema.get("additionalProperties", not self.strict)
                    if add_props is False or (self.strict and "additionalProperties" not in schema):
                        errors.append(
                            ValidationError(path=prop_path, message=f"Unknown property '{k}' not permitted", keyword="additionalProperties", invalid_value=val)
                        )
                    elif isinstance(add_props, dict):
                        self._validate_node(val, add_props, prop_path, errors)

        # 7. Combinators: allOf, anyOf, oneOf, not
        if "allOf" in schema:
            for sub_schema in schema["allOf"]:
                self._validate_node(instance, sub_schema, path, errors)

        if "anyOf" in schema:
            any_valid = False
            sub_errs = []
            for sub_schema in schema["anyOf"]:
                trial_errs: List[ValidationError] = []
                self._validate_node(instance, sub_schema, path, trial_errs)
                if not trial_errs:
                    any_valid = True
                    break
                sub_errs.extend(trial_errs)
            if not any_valid:
                errors.append(ValidationError(path=path, message="Instance does not match any subschema in 'anyOf'", keyword="anyOf"))

        if "oneOf" in schema:
            valid_count = 0
            for sub_schema in schema["oneOf"]:
                trial_errs = []
                self._validate_node(instance, sub_schema, path, trial_errs)
                if not trial_errs:
                    valid_count += 1
            if valid_count != 1:
                errors.append(ValidationError(path=path, message=f"Instance matched {valid_count} subschemas in 'oneOf' (expected exactly 1)", keyword="oneOf"))

        if "not" in schema:
            trial_errs = []
            self._validate_node(instance, schema["not"], path, trial_errs)
            if not trial_errs:
                errors.append(ValidationError(path=path, message="Instance matched subschema in 'not'", keyword="not"))

    @staticmethod
    def _check_type(instance: Any, expected: Union[str, List[str]]) -> bool:
        if isinstance(expected, list):
            return any(PureJsonSchemaValidator._check_type(instance, t) for t in expected)

        type_map = {
            "string": lambda x: isinstance(x, str),
            "number": lambda x: isinstance(x, (int, float)) and not isinstance(x, bool),
            "integer": lambda x: isinstance(x, int) and not isinstance(x, bool),
            "boolean": lambda x: isinstance(x, bool),
            "array": lambda x: isinstance(x, list),
            "object": lambda x: isinstance(x, dict),
            "null": lambda x: x is None,
        }
        check_fn = type_map.get(expected)
        return check_fn(instance) if check_fn else True


def load_file(filepath: str, format_hint: str = "auto") -> Any:
    """Load JSON or YAML document from file."""
    if not os.path.exists(filepath):
        raise FileNotFoundError(f"File not found: '{filepath}'")

    ext = os.path.splitext(filepath)[1].lower()
    is_yaml = (format_hint == "yaml") or (format_hint == "auto" and ext in (".yaml", ".yml"))

    with open(filepath, mode="r", encoding="utf-8") as f:
        content = f.read()

    if is_yaml:
        if not HAS_YAML:
            raise ImportError("PyYAML is not installed. Please install 'pyyaml' to parse YAML files.")
        return yaml.safe_load(content)
    else:
        return json.loads(content)


def validate_data(data: Any, schema: Dict[str, Any], strict: bool = False) -> List[ValidationError]:
    """Validate data structure against schema."""
    if HAS_JSONSCHEMA:
        try:
            validator_cls = jsonschema.validators.validator_for(schema)
            validator = validator_cls(schema)
            errors = []
            for err in sorted(validator.iter_errors(data), key=lambda e: e.path):
                path_str = "/" + "/".join(str(p) for p in err.path)
                errors.append(ValidationError(path=path_str, message=err.message, keyword=err.validator, invalid_value=err.instance))
            return errors
        except Exception:
            # Fallback to pure validator on any library divergence
            pass

    pure_val = PureJsonSchemaValidator(schema, strict=strict)
    return pure_val.validate(data)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="JSON and YAML Schema Validator",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument("--schema", "-s", required=True, type=str, help="Path to JSON/YAML Schema file.")
    parser.add_argument("--data", "-d", nargs="+", help="One or more JSON/YAML data files to validate.")
    parser.add_argument("--format", "-f", choices=["auto", "json", "yaml"], default="auto", help="File format parser.")
    parser.add_argument("--strict", action="store_true", help="Reject undeclared properties (disallow additionalProperties).")
    parser.add_argument("--quiet", "-q", action="store_true", help="Only output validation errors.")
    parser.add_argument("--json-output", action="store_true", help="Emit validation report as JSON.")
    parser.add_argument("--dry-run", action="store_true", help="Validate schema syntax without validating data files.")

    args = parser.parse_args()

    # Load schema
    try:
        schema = load_file(args.schema, format_hint=args.format)
    except Exception as e:
        sys.stderr.write(f"Error loading schema file '{args.schema}': {e}\n")
        return 2

    if not isinstance(schema, dict):
        sys.stderr.write(f"Schema error: Root schema in '{args.schema}' must be a JSON object.\n")
        return 2

    if args.dry_run:
        engine = "jsonschema" if HAS_JSONSCHEMA else "PureJsonSchemaValidator (built-in)"
        print(f"[DRY-RUN] Schema '{args.schema}' loaded successfully.")
        print(f"Validation Engine: {engine}")
        print(f"Schema Title: {schema.get('title', 'N/A')}")
        print(f"Schema Type: {schema.get('type', 'N/A')}")
        return 0

    if not args.data:
        sys.stderr.write("Error: At least one data file must be provided via --data (or use --dry-run).\n")
        return 2

    overall_success = True
    reports = []

    for file_path in args.data:
        try:
            instance = load_file(file_path, format_hint=args.format)
            errs = validate_data(instance, schema, strict=args.strict)
            is_valid = len(errs) == 0
            if not is_valid:
                overall_success = False

            reports.append({
                "file": file_path,
                "valid": is_valid,
                "errors": [{"path": e.path, "keyword": e.keyword, "message": e.message} for e in errs],
            })
        except Exception as e:
            overall_success = False
            reports.append({
                "file": file_path,
                "valid": False,
                "errors": [{"path": "", "keyword": "load_error", "message": str(e)}],
            })

    if args.json_output:
        print(json.dumps({"success": overall_success, "reports": reports}, indent=2))
    else:
        for rep in reports:
            if rep["valid"]:
                if not args.quiet:
                    print(f"PASS: {rep['file']} is valid.")
            else:
                print(f"FAIL: {rep['file']} has {len(rep['errors'])} validation error(s):")
                for err in rep["errors"]:
                    ptr = err["path"] if err["path"] else "/"
                    print(f"  - [{err['keyword']}] {ptr}: {err['message']}")

    return 0 if overall_success else 1


if __name__ == "__main__":
    sys.exit(main())
