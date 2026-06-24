package services_test

import (
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/services"
)

func TestIsProtected(t *testing.T) {
	cases := []struct {
		name      string
		protected bool
	}{
		{"sshd.service", true},
		{"ssh.service", true},
		{"ctrld.service", true},
		{"nginx.service", false},
		{"postgresql.service", false},
		{"dbus.service", true},
	}

	for _, tc := range cases {
		got := services.IsProtectedExported(tc.name)
		if got != tc.protected {
			t.Errorf("%s: erwartet protected=%v, bekommen %v", tc.name, tc.protected, got)
		}
	}
}

func TestDemoServices(t *testing.T) {
	demo := services.DemoServicesExported()
	if len(demo) == 0 {
		t.Error("demo services sollten nicht leer sein")
	}

	// SSH sollte als protected markiert sein
	for _, s := range demo {
		if s.Name == "sshd.service" && !s.IsProtected {
			t.Error("sshd.service sollte als protected markiert sein")
		}
	}
}

func TestActionValidation(t *testing.T) {
	validActions := []services.Action{
		services.ActionStart,
		services.ActionStop,
		services.ActionRestart,
		services.ActionEnable,
		services.ActionDisable,
		services.ActionReload,
	}

	for _, a := range validActions {
		if !services.IsValidActionExported(a) {
			t.Errorf("aktion %q sollte gültig sein", a)
		}
	}

	if services.IsValidActionExported("delete") {
		t.Error("aktion 'delete' sollte ungültig sein")
	}
}

func TestProtectedServiceCannotBeStoppedOrDisabled(t *testing.T) {
	// sshd kann nicht gestoppt werden
	err := services.ValidateActionExported("sshd.service", services.ActionStop)
	if err == nil {
		t.Error("sshd.service stop sollte fehlschlagen")
	}

	err = services.ValidateActionExported("sshd.service", services.ActionDisable)
	if err == nil {
		t.Error("sshd.service disable sollte fehlschlagen")
	}

	// sshd kann aber restartet werden
	err = services.ValidateActionExported("sshd.service", services.ActionRestart)
	if err != nil {
		t.Errorf("sshd.service restart sollte erlaubt sein: %v", err)
	}
}

func TestNormalServiceCanBeStoppedAndDisabled(t *testing.T) {
	err := services.ValidateActionExported("nginx.service", services.ActionStop)
	if err != nil {
		t.Errorf("nginx.service stop sollte erlaubt sein: %v", err)
	}

	err = services.ValidateActionExported("nginx.service", services.ActionDisable)
	if err != nil {
		t.Errorf("nginx.service disable sollte erlaubt sein: %v", err)
	}
}
