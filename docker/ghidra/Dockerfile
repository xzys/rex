# Ghidra 12.1.3 headless, aarch64.
#
# Why a build step rather than a stock image: the official Ghidra release ships native
# helper binaries for linux_x86_64 and win_x86_64 ONLY. On arm64 the decompiler is
# missing, so analysis runs but produces no C. This image builds it from the C++ source
# that ships inside the distribution.
#
# Build (needs ~600MB download + ~15 min compile on this box):
#   docker build -t ghidra-headless:12.1.3 docker/ghidra
#
# Run -- keep ALL project data on bind mounts, nothing baked in:
#   docker run --rm \
#     --memory=1600m --memory-swap=1600m --cpus=3 \
#     -v "$PWD/targets:/work/targets" \
#     -v "$PWD/scripts/ghidra:/work/scripts:ro" \
#     -v "$PWD/../ghidra-projects:/work/proj" \
#     ghidra-headless:12.1.3 \
#     /work/proj brick -import /work/targets/... \
#     -scriptPath /work/scripts -postScript FindPanelInit.java jd9168
#
FROM eclipse-temurin:21-jdk

ARG GHIDRA_VERSION=12.1.3
ARG GHIDRA_ZIP=ghidra_12.1.3_PUBLIC_20260817.zip
ARG GHIDRA_URL=https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_12.1.3_build/${GHIDRA_ZIP}

RUN apt-get update && apt-get install -y --no-install-recommends \
        unzip curl ca-certificates \
        g++ make bison flex \
        binwalk device-tree-compiler u-boot-tools abootimg \
        squashfs-tools cpio lz4 zstd xz-utils python3 \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL -o /tmp/ghidra.zip "${GHIDRA_URL}" \
    && unzip -q /tmp/ghidra.zip -d /opt \
    && rm /tmp/ghidra.zip \
    && ln -s /opt/ghidra_${GHIDRA_VERSION}_PUBLIC /opt/ghidra

# Build the aarch64 decompiler. The shipped Makefile has an x86-only arch check
# (its own comment: "TODO: need to revise to support arm64/aarch64 arch") -- anything
# that is not x86_64 falls through to -m32/linux_x86_32, which cannot build here.
# Overriding ARCH_TYPE and OSDIR on the command line is the fix.
RUN set -eux; \
    cd /opt/ghidra/Ghidra/Features/Decompiler/src/decompile/cpp; \
    make ghidra_opt ARCH_TYPE= OSDIR=linux_arm_64 -j"$(nproc)"; \
    mkdir -p ../../../os/linux_arm_64; \
    cp ghidra_opt ../../../os/linux_arm_64/decompile; \
    chmod +x ../../../os/linux_arm_64/decompile; \
    find . -name '*.o' -delete

# Heap must stay well under the container --memory limit: the JVM needs non-heap room
# on top of it. 1200m heap inside a 1600m container is the tested pairing.
ENV GHIDRA_HEADLESS_MAXMEM=1200M
ENV JAVA_HOME=/opt/java/openjdk

WORKDIR /work
ENTRYPOINT ["/opt/ghidra/support/analyzeHeadless"]
