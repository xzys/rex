#!/bin/bash
# Switch to Rex permanently by updating systemd defaults, then switch now.
sudo systemctl enable rex.service
sudo systemctl disable emulationstation.service
sudo systemctl start rex.service
sudo systemctl stop emulationstation.service
