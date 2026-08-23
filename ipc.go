package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runIPC talks to the shell and to a plugin's loaded surface, wrapping
// `omarchy-shell shell <verb> ...`.
//
// Inside a project dir the plugin id always comes from manifest.json
// (--plugin overrides); outside, the first positional is the id. The first
// positional after the id is a verb:
//
//	summon [payload]  -> omarchy-shell shell summon <id> [payload]
//	toggle [payload]  -> omarchy-shell shell toggle <id> [payload]
//	hide              -> omarchy-shell shell hide <id>
//	ping              -> omarchy-shell shell ping
//	call <action> [args...]  -> shell call <id> <action> <arg>
//	<anything else>   -> shell call <id> <verb> <arg>
//
// The shell's call() takes three mandatory string params, so omarchy-shell
// only auto-fills the arg slot for summon/toggle - calls land on the strict
// arity error. This wrapper always fills the slot ("{}" when no arg is
// given) and decodes the result locally:
//
//   - "ok"      -> the action ran (void result)
//   - anything else -> printed as the action's return value
//   - "unknown" -> ambiguous: either the surface is not loaded (hidden panel
//     destroys the bridge unless keepLoaded) or the method does not exist.
//     Check the installed entry points and say which.
func runIPC(args []string) error {
	// flags: --plugin/-p, --json, -h
	pluginFlag := ""
	jsonMode := false
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			jsonMode = true
		case a == "-p" || a == "--plugin":
			if i+1 >= len(args) {
				return fmt.Errorf("flag %s needs a value", a)
			}
			pluginFlag = args[i+1]
			i++
		case strings.HasPrefix(a, "--plugin="):
			pluginFlag = strings.TrimPrefix(a, "--plugin=")
		case a == "-h" || a == "--help":
			printCmdHelp("ipc")
			return nil
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) == 0 {
		printCmdHelp("ipc")
		return nil
	}

	// ping is shell-level: no plugin id involved
	if pos[0] == "ping" {
		argv, err := ipcDispatch("", "ping", nil, false)
		if err != nil {
			return err
		}
		return runShellIPC(argv)
	}

	// resolve the plugin id: --plugin flag > manifest > first positional
	id := pluginFlag
	if id == "" {
		if m, err := readManifest(filepath.Join(".", "manifest.json")); err == nil {
			id = m.ID // inside a project: always from the manifest
		}
	}
	if id == "" {
		if len(pos) < 2 {
			return fmt.Errorf("no manifest.json here and no plugin id given (use --plugin <id>, or: oma ipc <pluginId> <verb> [args...])")
		}
		id = pos[0]
		pos = pos[1:]
	}

	verb := pos[0]
	rest := pos[1:]

	// local diagnostic: is the plugin installed at all?
	home, _ := os.UserHomeDir()
	installed := filepath.Join(home, ".config", "omarchy", "plugins", id)
	if _, err := os.Stat(installed); err != nil {
		return fmt.Errorf("plugin %q is not installed (%s) - run oma install first", id, installed)
	}

	argv, err := ipcDispatch(id, verb, rest, jsonMode)
	if err != nil {
		return err
	}
	return runShellIPC(argv)
}

// runShellIPC executes an omarchy-shell argv and maps the result:
// "ok" -> success, "unknown" -> ambiguous failure with a local diagnosis,
// "error" -> the action threw, anything else -> printed as a return value.
func runShellIPC(argv []string) error {
	out, err := exec.Command("omarchy-shell", argv...).CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		return fmt.Errorf("omarchy-shell: %v%s", err, indentOutput(out))
	}
	switch result {
	case "ok":
		return nil
	case "unknown":
		// only reachable on the call path: summon/toggle/hide answer ok or
		// plain shell output, never "unknown"
		action := argv[len(argv)-2]
		id := argv[len(argv)-3]
		return fmt.Errorf("unknown: no %s surface is loaded or %q is not a method on it (load the surface with 'oma ipc summon' or check the entry points%s)", id, action, methodHint(id, action))
	case "error":
		id := argv[len(argv)-3]
		action := argv[len(argv)-2]
		return fmt.Errorf("error: %s.%s threw (see oma log --level warn)", id, action)
	default:
		// the action returned a value (e.g. a snapshot string)
		fmt.Println(result)
		return nil
	}
}

// ipcDispatch maps (id, verb, args) onto omarchy-shell argv. The verbs
// summon/toggle/hide/ping are shell-level; everything else - including an
// explicit `call` - targets a surface action through shell call.
func ipcDispatch(id, verb string, args []string, jsonMode bool) ([]string, error) {
	switch verb {
	case "summon", "toggle":
		if len(args) > 1 {
			return nil, fmt.Errorf("%s takes at most one payload argument (JSON)", verb)
		}
		payload := "{}"
		if len(args) == 1 {
			payload = args[0]
		}
		return []string{"shell", verb, id, payload}, nil
	case "hide":
		if len(args) > 0 {
			return nil, fmt.Errorf("hide takes no arguments")
		}
		return []string{"shell", "hide", id}, nil
	case "ping":
		return []string{"shell", "ping"}, nil
	case "call":
		if len(args) == 0 {
			return nil, fmt.Errorf("call needs an action: oma ipc call <action> [args...]")
		}
		arg, err := callArg(args[1:], jsonMode)
		if err != nil {
			return nil, err
		}
		return []string{"shell", "call", id, args[0], arg}, nil
	default:
		arg, err := callArg(args, jsonMode)
		if err != nil {
			return nil, err
		}
		return []string{"shell", "call", id, verb, arg}, nil
	}
}

// callArg builds the mandatory third argument of shell call(id, method, arg):
// "{}" when omitted, the raw arg when --json (caller-supplied JSON), a JSON
// array of the args with --json and several args, space-joined otherwise.
func callArg(args []string, jsonMode bool) (string, error) {
	switch {
	case len(args) == 0:
		return "{}", nil
	case jsonMode:
		if len(args) == 1 {
			return args[0], nil // caller-supplied JSON, passed verbatim
		}
		enc, err := json.Marshal(args)
		if err != nil {
			return "", fmt.Errorf("encode args: %w", err)
		}
		return string(enc), nil
	default:
		return strings.Join(args, " "), nil
	}
}

// methodHint greps the installed plugin's entry points for `function <action>`
// so the "unknown" ambiguity resolves to a concrete diagnosis.
func methodHint(id, action string) string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "omarchy", "plugins", id)
	m, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return ""
	}
	for _, rel := range m.EntryPoints {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "function "+action) {
			return fmt.Sprintf(" (%s declares %q - the surface is probably just not loaded)", rel, action)
		}
	}
	return fmt.Sprintf(" (%s does not declare %q in any entry point)", id, action)
}

func indentOutput(out []byte) string {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return ""
	}
	return "\n" + s
}
