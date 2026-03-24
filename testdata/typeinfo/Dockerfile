###############################################################################
# Dockerfile: Cyclone DDS 11.0.1 TypeInformation byte dumper
#
# Builds Cyclone DDS from source, compiles a test IDL with idlc, and runs
# a small C program that prints the serialized TypeInformation bytes for
# each type. Use the output to create golden test data for the Go XTypes
# implementation.
#
# Build & run:
#   docker build -t typeinfo-dump testdata/typeinfo/
#   docker run --rm typeinfo-dump
#
# The output format is:
#   TYPE_NAME size=N bytes=<hex>
###############################################################################

FROM debian:bookworm-slim AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# ---------- Build Cyclone DDS 11.0.1 ----------
WORKDIR /src
RUN git clone --branch 11.0.1 --depth 1 \
    https://github.com/eclipse-cyclonedds/cyclonedds.git

WORKDIR /src/cyclonedds/build
RUN cmake .. \
    -DCMAKE_INSTALL_PREFIX=/opt/cyclonedds \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_EXAMPLES=OFF \
    -DBUILD_TESTING=OFF \
    && cmake --build . --parallel $(nproc) \
    && cmake --install .

# ---------- Generate C code from IDL ----------
WORKDIR /work
COPY test_types.idl .
RUN /opt/cyclonedds/bin/idlc -l c test_types.idl

# ---------- Build the dump program ----------
COPY dump_typeinfo.c .
RUN gcc -Wall -Wextra -o dump_typeinfo dump_typeinfo.c test_types.c \
    -I. \
    -I/opt/cyclonedds/include \
    -L/opt/cyclonedds/lib \
    -lddsc \
    -Wl,-rpath,/opt/cyclonedds/lib

# ---------- Runtime stage ----------
FROM debian:bookworm-slim

COPY --from=builder /opt/cyclonedds/lib/ /opt/cyclonedds/lib/
COPY --from=builder /work/dump_typeinfo /usr/local/bin/dump_typeinfo

ENV LD_LIBRARY_PATH=/opt/cyclonedds/lib

CMD ["dump_typeinfo"]
