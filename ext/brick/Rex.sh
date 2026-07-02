#!/bin/bash
# ES menu launcher (Tools → Rex). Thin wrapper — start-rex.sh owns the full
# lifecycle: brightness/connman staging, wifi capture, ES stop, termsp launch,
# and the EXIT trap that powers off (shutdown request) or restarts ES.
exec /userdata/system/apps/rex/start-rex.sh
