#!/bin/bash

SESSION_NAME="default"

# Check if the session exists, then reattach, or create a new one
screen -ls | grep -q "$SESSION_NAME" && screen -d -r "$SESSION_NAME" || screen -S "$SESSION_NAME"

