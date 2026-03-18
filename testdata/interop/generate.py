#!/usr/bin/env python3
"""Generate CDR binary test fixtures using pycdr2 (CycloneDDS).

Usage:
    pip install pycdr2
    python generate.py

Produces .bin files containing raw XCDR2 CDR payloads (no encapsulation header)
for cross-language interoperability testing with the Go CDR decoder.
"""

import struct
from dataclasses import dataclass
from pathlib import Path

from pycdr2 import IdlStruct
from pycdr2.types import float32, float64, int32
from pycdr2.annotations import final

OUTDIR = Path("/out")


@final
@dataclass
class SensorData(IdlStruct):
    sensor_id: int32
    temperature: float64
    humidity: float32
    location: str
    active: bool


@final
@dataclass
class PrimitiveTypes(IdlStruct):
    val_bool: bool
    val_int32: int32
    val_float32: float32
    val_float64: float64
    val_string: str


def strip_header(data: bytes) -> bytes:
    """Remove the 4-byte CDR encapsulation header."""
    return data[4:]


def write_fixture(name: str, data: bytes) -> None:
    path = OUTDIR / f"{name}.bin"
    path.write_bytes(data)
    print(f"  wrote {path.name} ({len(data)} bytes)")


def main() -> None:
    print("Generating CDR interop test fixtures...")

    # 1. SensorData -- basic FINAL struct
    sensor = SensorData(
        sensor_id=42,
        temperature=23.5,
        humidity=65.199996948242188,  # float32 exact value
        location="warehouse-A",
        active=True,
    )
    raw = strip_header(sensor.serialize(use_version_2=True))
    write_fixture("sensor_data_le", raw)

    # 2. PrimitiveTypes -- various primitive types
    prims = PrimitiveTypes(
        val_bool=False,
        val_int32=-12345,
        val_float32=3.140000104904175,  # float32 exact
        val_float64=2.718281828459045,
        val_string="Hello, DDS!",
    )
    raw = strip_header(prims.serialize(use_version_2=True))
    write_fixture("primitive_types_le", raw)

    # 3. Empty string edge case
    empty_str = PrimitiveTypes(
        val_bool=True,
        val_int32=0,
        val_float32=0.0,
        val_float64=0.0,
        val_string="",
    )
    raw = strip_header(empty_str.serialize(use_version_2=True))
    write_fixture("empty_string_le", raw)

    print("Done.")


if __name__ == "__main__":
    main()
