package main

import (
	"reflect"
	"testing"
)

// Shell-level verbs route to the shell surface; the call arg slot is always
// filled ("{}" when omitted) so the shell's strict arity never bites.
func TestIPCDispatchShellVerbs(t *testing.T) {
	cases := []struct {
		verb string
		args []string
		want []string
	}{
		{"summon", nil, []string{"shell", "summon", "a.b", "{}"}},
		{"summon", []string{`{"menu":"root"}`}, []string{"shell", "summon", "a.b", `{"menu":"root"}`}},
		{"toggle", nil, []string{"shell", "toggle", "a.b", "{}"}},
		{"hide", nil, []string{"shell", "hide", "a.b"}},
		{"ping", nil, []string{"shell", "ping"}},
	}
	for _, c := range cases {
		argv, err := ipcDispatch("a.b", c.verb, c.args, false)
		if err != nil {
			t.Fatalf("%s: %v", c.verb, err)
		}
		if !reflect.DeepEqual(argv, c.want) {
			t.Errorf("%s: argv = %v, want %v", c.verb, argv, c.want)
		}
	}
}

// Non-shell verbs (and the explicit `call` verb) target a surface action.
func TestIPCDispatchCall(t *testing.T) {
	cases := []struct {
		verb     string
		args     []string
		jsonMode bool
		want     []string
	}{
		{"clearDone", nil, false, []string{"shell", "call", "a.b", "clearDone", "{}"}},
		{"addTodo", []string{"buy milk"}, false, []string{"shell", "call", "a.b", "addTodo", "buy milk"}},
		{"addTodo", []string{"buy milk"}, true, []string{"shell", "call", "a.b", "addTodo", "buy milk"}},
		{"call", []string{"setVolume", "80"}, false, []string{"shell", "call", "a.b", "setVolume", "80"}},
		{"call", []string{"setVolume", "80", "90"}, true, []string{"shell", "call", "a.b", "setVolume", `["80","90"]`}},
		{"snapshot", nil, false, []string{"shell", "call", "a.b", "snapshot", "{}"}},
	}
	for _, c := range cases {
		argv, err := ipcDispatch("a.b", c.verb, c.args, c.jsonMode)
		if err != nil {
			t.Fatalf("%s: %v", c.verb, err)
		}
		if !reflect.DeepEqual(argv, c.want) {
			t.Errorf("%s: argv = %v, want %v", c.verb, argv, c.want)
		}
	}
}

func TestIPCDispatchErrors(t *testing.T) {
	if _, err := ipcDispatch("a.b", "summon", []string{"x", "y"}, false); err == nil {
		t.Error("summon with two args should error")
	}
	if _, err := ipcDispatch("a.b", "hide", []string{"x"}, false); err == nil {
		t.Error("hide with an arg should error")
	}
	if _, err := ipcDispatch("a.b", "call", nil, false); err == nil {
		t.Error("call without an action should error")
	}
}
