package config

// This file is compiled only under `go test` and adds no exported surface as
// consumers see the package.
//
// validateCritic is unexported because it is a step of loading, not a thing a caller
// invokes. A test needs it directly: reaching it through Load means writing files and
// setting environment variables, which tests the loader rather than the rule.

// ValidateCriticForTest resolves one TOML document over the defaults and validates it.
func ValidateCriticForTest(doc string) error {
	_, err := resolve([][]byte{[]byte(doc)})
	return err
}
