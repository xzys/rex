# Keyboard App

A simple terminal-based keyboard visualization app built with Bubble Tea.

## Prerequisites

1. Install Go (version 1.21 or later)

## Installation

1. Clone this repository
2. Install dependencies:
```bash
go mod download
```

## Running the Application

Run the application:
```bash
go run main.go
```

## Features

- Displays a keyboard layout in the terminal
- Press keys to cycle through different states (unselected, absent, present, correct)
- Press Ctrl+C to exit

## Controls

- Press any letter key to cycle through its states
- Ctrl+C to exit 