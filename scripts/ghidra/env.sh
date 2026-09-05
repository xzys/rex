# Source this before running Ghidra headless on this host.
#   . scripts/ghidra/env.sh
export JAVA_HOME=/home/sachin/opt/jdk-21.0.12.1+1
export GHIDRA_HOME=/home/sachin/opt/ghidra_12.1.3_PUBLIC
# 3.8 GB box: Ghidra's 2G default swap-thrashes. See notes/ghidra-runbook.md.
export GHIDRA_HEADLESS_MAXMEM=1200M
export GHIDRA_HEADLESS="$GHIDRA_HOME/support/analyzeHeadless"
