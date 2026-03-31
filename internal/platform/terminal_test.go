package platform

import "testing"

func TestKeyConstantsDistinct(t *testing.T) {
	keys := []struct {
		name string
		val  Key
	}{
		{"KeyNone", KeyNone},
		{"KeyUp", KeyUp},
		{"KeyDown", KeyDown},
		{"KeyEnter", KeyEnter},
		{"KeyEsc", KeyEsc},
		{"KeyQ", KeyQ},
		{"KeyR", KeyR},
		{"KeyS", KeyS},
		{"KeyRune", KeyRune},
	}

	seen := make(map[Key]string, len(keys))
	for _, k := range keys {
		if prev, dup := seen[k.val]; dup {
			t.Errorf("duplicate Key value %d: %s and %s", k.val, prev, k.name)
		}
		seen[k.val] = k.name
	}
}

func TestParseKeyBytes(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  Key
	}{
		{"arrow up", []byte{0x1b, '[', 'A'}, KeyUp},
		{"arrow down", []byte{0x1b, '[', 'B'}, KeyDown},
		{"escape", []byte{0x1b}, KeyEsc},
		{"enter CR", []byte{'\r'}, KeyEnter},
		{"enter LF", []byte{'\n'}, KeyEnter},
		{"q lower", []byte{'q'}, KeyQ},
		{"Q upper", []byte{'Q'}, KeyQ},
		{"r lower", []byte{'r'}, KeyR},
		{"R upper", []byte{'R'}, KeyR},
		{"s lower", []byte{'s'}, KeyS},
		{"S upper", []byte{'S'}, KeyS},
		{"other rune", []byte{'x'}, KeyRune},
		{"unknown escape seq", []byte{0x1b, '[', 'C'}, KeyNone},
		{"empty", []byte{}, KeyNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseKeyBytes(tt.input)
			if got != tt.want {
				t.Errorf("parseKeyBytes(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
