# CODE REVIEW

## Remaining Issues

### 8. R2 modifier has no chord definitions
- `modifierR2` is tracked in state and the bindings map desktop `v` / SDL button 7 to `KeyR2`, but `getChordMaps` never defines any entries for R2.
- Every R2 chord logs an error and returns `nil`, so that modifier can't emit characters.

**Fix:** either remove the R2 modifier entirely (including bindings) or define a chord & alt map for it.

### 11. Server only handles one connection then exits
- The server now logs errors and continues accepting connections (the main issue is fixed), but connection lifecycle management could be improved. Each connection is handled synchronously, so if one connection blocks or takes a long time, new connections wait.

```63:73:server/server.go
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "err", err)
			continue
		}

		if err := s.handleConnection(conn); err != nil {
			slog.Error("failed to handle connection", "err", err)
		}
	}
```

**Status:** Main issue fixed (server no longer exits), but consider handling connections concurrently for better scalability.

---

## Summary
- **2 issues remaining** that need attention
- Server/client integration issues (#12, #13) have been fixed through refactoring to Server struct
- Remaining issues: R2 modifier coverage and connection concurrency
