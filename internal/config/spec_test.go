package config

// spec and aspec unwrap an entry to the built-in settings struct the tests
// inspect; they panic on an adapter this package does not own.
func spec(e RuntimeEntry) RuntimeSpec { return e.Settings.(RuntimeSpec) }

func aspec(e AgentEntry) AgentSpec { return e.Settings.(AgentSpec) }
