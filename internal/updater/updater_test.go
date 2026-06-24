package updater_test

import (
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/updater"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v1.0.0", "v0.9.0", true},
		{"v1.0.1", "v1.0.0", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v1.0.0", false},
		{"v0.9.0", "v1.0.0", false},
		{"v1.0.0", "dev", false},
		{"v1.0.0", "", false},
		{"v1.0.0", "0.0.0", false},
		{"v1.1.0", "v1.0.9", true},
	}

	for _, tc := range cases {
		got := updater.IsNewerVersionExported(tc.latest, tc.current)
		if got != tc.want {
			t.Errorf("isNewerVersion(%q, %q): erwartet %v, bekommen %v",
				tc.latest, tc.current, tc.want, got)
		}
	}
}
