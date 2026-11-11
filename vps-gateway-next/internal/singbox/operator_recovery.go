package singbox

// NewOperatorForRecovery is the construction boundary for the original startup
// factory. Callers must complete owned journal recovery before exposing runtime
// producers. The original production factory is still being integrated.
func NewOperatorForRecovery(d OperatorDeps) *Operator {
	return constructOperatorForStartup(d, true)
}
