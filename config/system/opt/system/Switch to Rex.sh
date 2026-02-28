#!/bin/bash
# Start Rex via systemd, then stop EmulationStation.
sudo systemctl start rex.service
sudo systemctl stop emulationstation.service
