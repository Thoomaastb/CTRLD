package services

import "fmt"

// IsProtectedExported ist eine exportierte Version für Tests.
func IsProtectedExported(name string) bool {
	return isProtected(name)
}

// DemoServicesExported gibt Demo-Services für Tests zurück.
func DemoServicesExported() []Service {
	return demoServices()
}

// IsValidActionExported prüft ob eine Action gültig ist.
func IsValidActionExported(a Action) bool {
	switch a {
	case ActionStart, ActionStop, ActionRestart, ActionEnable, ActionDisable, ActionReload:
		return true
	}
	return false
}

// ValidateActionExported prüft ob eine Aktion auf einem Service erlaubt ist.
func ValidateActionExported(name string, action Action) error {
	if isProtected(name) {
		switch action {
		case ActionStop, ActionDisable:
			return fmt.Errorf("'%s' ist ein kritischer service", name)
		}
	}
	if !IsValidActionExported(action) {
		return fmt.Errorf("ungültige aktion: %s", action)
	}
	return nil
}
