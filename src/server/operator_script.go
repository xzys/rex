package server

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/buke/quickjs-go"
	"github.com/xrfr/t-rex/v2/protocol"
)

// ScriptOperator runs a JavaScript plugin that defines a top-level handleKey(key)
// function. The plugin receives a key code (matching protocol.Key* constants
// injected as globals) and returns a string passed to `tmux send-keys -t 0`, or
// null/undefined if the key is unhandled (falls through to global fallbacks).
type ScriptOperator struct {
	mu        sync.Mutex
	name      string
	runtime   *quickjs.Runtime
	ctx       *quickjs.Context
	globals   *quickjs.Value
	handleKey *quickjs.Value
	repeatKeys []int // populated from JS repeatKeys array, nil if absent
}

var pluginKeyConstants = map[string]int{
	"KEY_DPAD_UP":    protocol.KeyDPadUp,
	"KEY_DPAD_DOWN":  protocol.KeyDPadDown,
	"KEY_DPAD_LEFT":  protocol.KeyDPadLeft,
	"KEY_DPAD_RIGHT": protocol.KeyDPadRight,
	"KEY_A":          protocol.KeyA,
	"KEY_B":          protocol.KeyB,
	"KEY_X":          protocol.KeyX,
	"KEY_Y":          protocol.KeyY,
	"KEY_L1":         protocol.KeyL1,
	"KEY_R1":         protocol.KeyR1,
	"KEY_L2":         protocol.KeyL2,
	"KEY_R2":         protocol.KeyR2,
	"KEY_SELECT":     protocol.KeySelect,
	"KEY_START":      protocol.KeyStart,
	"KEY_FN":         protocol.KeyFn,
	"KEY_L3":         protocol.KeyL3,
	"KEY_R3":         protocol.KeyR3,
}

func NewScriptOperator(name, script string) (*ScriptOperator, error) {
	// Disable the owner-goroutine check: the operator can be created on one
	// goroutine (NewServer) and called from another (input device loop). Our
	// ScriptOperator.mu already serializes all JS access, so this is safe.
	rt := quickjs.NewRuntime(quickjs.WithOwnerGoroutineCheck(false))
	ctx := rt.NewContext()
	globals := ctx.Globals()

	for k, v := range pluginKeyConstants {
		globals.Set(k, ctx.NewInt32(int32(v)))
	}

	result := ctx.Eval(script, quickjs.EvalFileName(name+".js"))
	if result.IsException() {
		err := ctx.Exception()
		result.Free()
		rt.Close()
		return nil, fmt.Errorf("eval %s: %w", name, err)
	}
	result.Free()

	handleKey := globals.Get("handleKey")
	if !handleKey.IsFunction() {
		handleKey.Free()
		rt.Close()
		return nil, fmt.Errorf("plugin %q: global handleKey() function not defined", name)
	}

	// Parse optional repeatKeys array for auto-repeat hints.
	var repeatKeys []int
	repeatKeysVal := globals.Get("repeatKeys")
	if !repeatKeysVal.IsUndefined() {
		if repeatKeysVal.IsArray() {
			length := repeatKeysVal.Len()
			repeatKeys = make([]int, 0, length)
			for i := int64(0); i < length; i++ {
				elem := repeatKeysVal.GetIdx(i)
				if elem.IsNumber() {
					repeatKeys = append(repeatKeys, int(elem.Int32()))
				}
				elem.Free()
			}
		}
	}
	repeatKeysVal.Free()

	return &ScriptOperator{
		name:       name,
		runtime:    rt,
		ctx:        ctx,
		globals:    globals,
		handleKey:  handleKey,
		repeatKeys: repeatKeys,
	}, nil
}

// HandleInput invokes the plugin's handleKey on key-down. Key-up events are
// ignored so plugins can stay pure mapping functions.
// RepeatKeys returns the keys the JS plugin wants auto-repeated, or nil if
// the plugin didn't declare a repeatKeys array.
func (o *ScriptOperator) RepeatKeys() []int {
	return o.repeatKeys
}

func (o *ScriptOperator) HandleInput(key int, isDown bool) (bool, error) {
	if !isDown {
		return false, nil
	}
	tmuxKey, ok := o.evalKey(key)
	if !ok {
		return false, nil
	}
	return true, execute("tmux", "send-keys", "-t", "0", tmuxKey)
}

// evalKey invokes the plugin's handleKey and returns (tmuxKeyString, handled).
// Errors from JS are logged and treated as unhandled so a buggy plugin can't
// crash the server.
func (o *ScriptOperator) evalKey(key int) (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	arg := o.ctx.NewInt32(int32(key))
	// Execute takes ownership of refs; don't free arg after the call.

	result := o.handleKey.Execute(o.globals, arg)
	defer result.Free()

	if result.IsException() {
		slog.Warn("plugin handleKey threw", "plugin", o.name, "err", o.ctx.Exception())
		return "", false
	}
	if result.IsNull() || result.IsUndefined() {
		return "", false
	}
	tmuxKey := result.String()
	if tmuxKey == "" {
		return "", false
	}
	return tmuxKey, true
}

func (o *ScriptOperator) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.handleKey != nil {
		o.handleKey.Free()
		o.handleKey = nil
	}
	// Don't free o.globals — Context caches it and frees on close.
	o.globals = nil
	if o.runtime != nil {
		o.runtime.Close()
		o.runtime = nil
	}
}
