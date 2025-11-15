# CODE REVIEW

## Remaining Issues

### 8. R2 modifier has no chord definitions
- `modifierR2` is tracked in state and the bindings map desktop `v` / SDL button 7 to `KeyR2`, but `getChordMaps` never defines any entries for R2.
- Every R2 chord logs an error and returns `nil`, so that modifier can't emit characters.

**Fix:** either remove the R2 modifier entirely (including bindings) or define a chord & alt map for it.

### 11. Server only handles one connection then exits
- `handleConnection` blocks synchronously, so when the connection ends, the error propagates and the server exits instead of accepting new connections.

```55:65:server/server.go
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		
		if err := handleConnection(conn); err != nil {
			return err
		}
	}
```

**Fix:** don't propagate errors from `handleConnection` back to the main loop; log them and continue:
```go
for {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	
	if err := handleConnection(conn); err != nil {
		slog.Error("connection error", "err", err)
		// Continue accepting new connections
	}
}
```

### 12. InputHandler connection is never wired to the keyboard client
- `server.Run()` creates an `InputHandler` but never calls `SetKeyboardConn()` when the keyboard client connects.
- When SDL events arrive, `HandleKey` logs "no connection" and drops every input.

**Fix:** pass `inputHandler` into `handleConnection` and call `SetKeyboardConn(&conn)` once the connection is established.

### 13. Server hangs on blank scanner output
- `handleConnection` skips empty lines but never checks `scanner.Scan()`'s boolean return.
- When the client disconnects, the scanner hits EOF, `Scan()` returns false, the loop terminates, and the function returns—but there's no mechanism to notify the input handler that the keyboard is gone.

**Fix:** detect EOF vs. errors explicitly, and clear the input handler's connection when the client disconnects.

### 14. Special key constants collide with printable characters
- `specialKeyShift = 'S'`, `specialKeyDelete = 'D'`, etc., are actual printable ASCII, so if the daemon wanted to emit a capital S or D (or 1, X, E), those would be misinterpreted by `pressKey` as special commands.
- The daemon has no way to distinguish "literal S" from "shift toggle."

```89:96:keyboard/model_chorded.go
const (
	specialKeySpace   = ' '
	specialKeyShift   = 'S'
	specialKeyDelete  = 'D'
	specialKeyNumbers = '1'
	specialKeyClose   = 'X'
	specialKeyEnter   = 'E'
)
```

**Fix:** use non-printable runes (e.g., control codes, private-use Unicode) or a structured message type so special keys can't collide with normal text. For example:
```go
const (
	specialKeySpace   = ' '
	specialKeyShift   = '\x01'  // Control-A
	specialKeyDelete  = '\x02'  // Control-B
	specialKeyNumbers = '\x03'  // Control-C
	specialKeyClose   = '\x04'  // Control-D
	specialKeyEnter   = '\n'
)
```

---

## Summary
- **5 issues remaining** that need attention
- Code is more robust but server/client integration and modifier coverage need work
