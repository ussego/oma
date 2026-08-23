package main

import "testing"

func TestStuckBarWidget(t *testing.T) {
	cases := []struct {
		name  string
		json  string
		id    string
		stuck bool
	}{
		{
			name:  "stuck: in plugins[] only",
			json:  `{"plugins":[{"id":"ussego.todo-app"}],"bar":{"layout":{"right":[{"id":"omarchy.power"}]}}}`,
			id:    "ussego.todo-app",
			stuck: true,
		},
		{
			name:  "placed: in bar.layout",
			json:  `{"plugins":[{"id":"ussego.todo-app"}],"bar":{"layout":{"right":[{"id":"ussego.todo-app"},{"id":"omarchy.power"}]}}}`,
			id:    "ussego.todo-app",
			stuck: false,
		},
		{
			name:  "not enabled anywhere",
			json:  `{"plugins":[{"id":"other"}],"bar":{"layout":{"right":[{"id":"omarchy.power"}]}}}`,
			id:    "ussego.todo-app",
			stuck: false,
		},
		{
			name:  "bare-string bar.layout entries tolerated",
			json:  `{"plugins":[{"id":"x.y"}],"bar":{"layout":{"left":["x.y"]}}}`,
			id:    "x.y",
			stuck: false,
		},
		{
			name:  "garbage config is not stuck (enable already warned)",
			json:  `not json`,
			id:    "x.y",
			stuck: false,
		},
		{
			name:  "empty config",
			json:  `{}`,
			id:    "x.y",
			stuck: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stuckBarWidget([]byte(tc.json), tc.id); got != tc.stuck {
				t.Errorf("stuckBarWidget(%s) = %v, want %v", tc.id, got, tc.stuck)
			}
		})
	}
}
