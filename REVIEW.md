# CODE REVIEW

## Remaining Issues

### 8. R2 modifier has no chord definitions
- `modifierR2` is tracked in state and the bindings map desktop `v` / SDL button 7 to `KeyR2`, but `getChordMaps` never defines any entries for R2.
- Every R2 chord logs an error and returns `nil`, so that modifier can't emit characters.

**Fix:** either remove the R2 modifier entirely (including bindings) or define a chord & alt map for it.

### 11. Server only handles one connection then exits
- The server now logs errors and continues accepting connections (the main issue is fixed), but connection lifecycle management could be improved. Each connection is handled synchronously, so if one connection blocks or takes a long time, new connections wait.

```56:66:server/server.go
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "err", err)
			continue
		}

		if err := handleConnection(conn); err != nil {
			slog.Error("failed to handle connection", "err", err)
		}
	}
```

**Status:** Main issue fixed (server no longer exits), but consider handling connections concurrently for better scalability.

### 12. InputHandler connection is never wired to the keyboard client
- `server.Run()` creates an `InputHandler` but never calls `SetKeyboardConn()` when the keyboard client connects.
- When SDL events arrive, `HandleKey` logs "no connection" and drops every input.

**Fix:** pass `inputHandler` into `handleConnection` and call `SetKeyboardConn(&conn)` once the connection is established.

### 13. Server hangs on blank scanner output
- `handleConnection` skips empty lines and checks `scanner.Scan()` in the loop condition, but when the client disconnects, the scanner hits EOF, `Scan()` returns false, the loop terminates, and the function returns—but there's no mechanism to notify the input handler that the keyboard is gone.

```69:95:server/server.go
func handleConnection(conn net.Conn) error {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) == 0 {
			continue
		}

		msg, err := protocol.ParseMessage(data)
		if err != nil {
			return fmt.Errorf("failed to parse message: %w", err)
		}

		switch m := msg.(type) {
		case *protocol.PressKeyMessage:
			if err := pressKey(m.KeyPress); err != nil {
				return fmt.Errorf("failed to press key: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan message: %w", err)
	}
	return nil
}
```

**Fix:** detect EOF vs. errors explicitly, and clear the input handler's connection when the client disconnects.

---

## Summary
- **4 issues remaining** that need attention
- Code is more robust but server/client integration and modifier coverage need work
