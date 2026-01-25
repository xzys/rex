# Testing Guide

This document describes how to write tests for the rex project.

## Running Tests

```bash
# Build before testing
make build-local

# Run all tests
go test ./...

# Run specific test
go test ./keyboard/... -run TestChordedKeyboardAllLetters

# Run without cache (useful for flaky tests)
go test ./keyboard/... -run TestChordedKeyboardAllLetters -count=1

# Run multiple times to check stability
go test ./keyboard/... -run TestChordedKeyboardAllLetters -count=10
```

## Test Architecture Patterns

### Mock Server Pattern

When testing components that connect to Unix sockets (like `keyboard` or `menu`), use the mock server pattern:

1. Test creates socket listeners (acting as server)
2. Test starts the component process (acting as client)
3. Test accepts connections from component
4. Test sends/receives messages directly

**Example:**

```go
// Test acts as server
inputListener, _ := net.Listen("unix", protocol.InputSocketPath())
outputListener, _ := net.Listen("unix", protocol.OutputSocketPath())
defer inputListener.Close()
defer outputListener.Close()

// Start component (client)
cmd := exec.CommandContext(ctx, "rex", "keyboard")
cmd.Start()

// Accept connections
inputConn, _ := inputListener.Accept()
outputConn, _ := outputListener.Accept()

// Send messages
encoder := json.NewEncoder(inputConn)
decoder := json.NewDecoder(outputConn)

msg := protocol.KeyInputMessage{...}
encoder.Encode(msg)

response, _ := protocol.ParseMessage(decoder)
```

**Why:** Simpler than running full server. Direct control over message timing and order.

### PTY Management

When testing Bubble Tea processes (UI components), use PTYs instead of regular stdin/stdout:

```go
cmd, ptmx, err := utils.StartRexProcessWithPTY(ctx, rexPath, args, env)
if err != nil {
    t.Fatalf("start process: %v", err)
}
defer ptmx.Close()
defer utils.TerminateProcess(cmd, 2*time.Second)

// CRITICAL: Drain PTY output to prevent buffer overflow
go func() {
    buf := make([]byte, 4096)
    for {
        _, err := ptmx.Read(buf)
        if err != nil {
            return
        }
    }
}()
```

**Why:** Bubble Tea requires a terminal (TTY). PTY provides this. See `notes/bugs/pty_buffer_overflow.md` for why draining is critical.

## Common Utilities

### Starting Rex Processes

Located in `utils/test_utils.go`:

- `ResolveRexBinary()` - Finds the rex binary (checks `REX_BIN` env var, then looks for `rex-local`)
- `StartRexProcessWithPTY()` - Starts a process with a PTY (for UI components)
- `TerminateProcess()` - Gracefully stops a process with timeout

### Protocol Messages

Located in `protocol/protocol.go`:

- `KeyInputMessage` - Send key events to components
- `PressKeyMessage` - Receive key press events from components
- `ParseMessage()` - Decode messages from JSON

### Test Timeouts

Always use contexts with timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

## Writing Tests

### Testing Keyboard Components

See `keyboard/keyboard_test.go` for a complete example.

**Key considerations:**

1. **Use PTY for Bubble Tea processes**
   ```go
   cmd, ptmx, err := utils.StartRexProcessWithPTY(ctx, rexPath, args, env)
   ```

2. **Drain PTY output continuously**
   ```go
   go func() {
       buf := make([]byte, 4096)
       for {
           _, err := ptmx.Read(buf)
           if err != nil {
               return
           }
       }
   }()
   ```

3. **Add delays between rapid messages**
   ```go
   encoder.Encode(modifierMsg)
   time.Sleep(50 * time.Millisecond)  // Allow Bubble Tea to process
   encoder.Encode(triggerMsg)
   time.Sleep(10 * time.Millisecond)
   ```

4. **Set read deadlines on sockets**
   ```go
   outputConn.SetReadDeadline(time.Now().Add(2 * time.Second))
   ```

5. **Use table-driven tests**
   ```go
   testCases := []struct{
       name     string
       modifier int
       trigger  int
       expected string
   }{
       {"a", protocol.KeyDPadUp, protocol.KeyL1, "a"},
       // ...
   }
   
   for _, tc := range testCases {
       t.Run(tc.name, func(t *testing.T) {
           // test logic
       })
   }
   ```

**How the keyboard test is organized:**

- `TestKeyboardModes` runs sub-suites per mode (`chorded`, `chorded-simultaneous`, `simple`) so each mode gets an isolated keyboard process and a clear failure surface.
- The chorded suites are built from base `chordTestCases` plus helpers that layer in numbers mode, special key chords, and shift-mode uppercase coverage.
- The simple suite reuses `simpleLayout` to drive both lowercase letters and special-key selections, so the test stays aligned with the UI layout.
- `keyboardCase` + `inputEvent` model the input stream and expected outputs, while `runKeyboardCase` wires those through socket IO for each case.

### Testing Server Components

See `server/inputs_stdin_test.go` for examples.

**Key considerations:**

1. **Use FIFOs for stdin input**
   ```go
   stdinFIFO := "/tmp/test-stdin.fifo"
   syscall.Mkfifo(stdinFIFO, 0666)
   defer os.Remove(stdinFIFO)
   
   stdinFile, _ := os.OpenFile(stdinFIFO, os.O_RDWR, 0)
   defer stdinFile.Close()
   
   cmd.Stdin = stdinFile
   ```

2. **Set environment variables**
   ```go
   cmd.Env = append(os.Environ(), "REX_INPUT_DEVICE=stdin")
   ```

3. **Write to FIFO in goroutine**
   ```go
   go func() {
       writer, _ := os.OpenFile(stdinFIFO, os.O_WRONLY, 0)
       defer writer.Close()
       writer.WriteString("test input\n")
   }()
   ```

## Debugging Flaky Tests

### Check for Race Conditions

```bash
go test -race ./...
```

### Run Multiple Times

```bash
# Run 20 times without caching
go test ./... -count=20
```

### Capture Stderr

```go
var stderr bytes.Buffer
cmd.Stderr = &stderr

// After test
if stderr.Len() > 0 {
    t.Logf("Stderr: %s", stderr.String())
}
```

### Add Debug Logging

Use `t.Logf()` liberally:

```go
t.Logf("Sending modifier: %d", tc.modifier)
t.Logf("Received response: %+v", response)
```

## Common Pitfalls

1. **Forgetting to drain PTY output** → "broken pipe" errors
   - Solution: Always add PTY drain goroutine

2. **Sending messages too quickly** → JSON decoder panics
   - Solution: Add `time.Sleep()` between rapid messages

3. **Not setting socket deadlines** → Tests hang forever
   - Solution: Use `SetReadDeadline()` on all socket reads

4. **Process not cleaned up** → Zombie processes, port conflicts
   - Solution: Use `defer utils.TerminateProcess(cmd, timeout)`

5. **Not removing temporary files** → Disk clutter, test conflicts
   - Solution: Use `defer os.Remove(path)` for all temp files

6. **Assuming socket is ready immediately** → Connection refused
   - Solution: Add small delay after starting listener before starting client

## Test Stability Checklist

- [ ] Uses context with timeout
- [ ] Cleans up all processes with defer
- [ ] Removes all temporary files
- [ ] Drains PTY output (if using PTY)
- [ ] Sets read deadlines on sockets
- [ ] Includes appropriate delays between rapid operations
- [ ] Runs reliably 20+ times without failures
- [ ] No race conditions (`go test -race`)
