package logs

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
)

// ListUnits gibt alle systemd-Units zurück die Log-Einträge haben.
func ListUnits(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "journalctl", "--field=_SYSTEMD_UNIT", "--no-pager")
	out, err := cmd.Output()
	if err != nil {
		if isNotFound(err) {
			return demoUnits(), nil
		}
		return nil, err
	}

	seen := make(map[string]bool)
	var units []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		unit := strings.TrimSpace(scanner.Text())
		if unit == "" || seen[unit] {
			continue
		}
		seen[unit] = true
		units = append(units, unit)
	}

	return units, nil
}

func demoUnits() []string {
	return []string{
		"nginx.service",
		"sshd.service",
		"cron.service",
		"systemd-timesyncd.service",
		"systemd-networkd.service",
		"docker.service",
	}
}
