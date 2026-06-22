package logs

// PriorityToSeverityExported ist eine exportierte Version für Tests.
func PriorityToSeverityExported(priority string) string {
	return priorityToSeverity(priority)
}
