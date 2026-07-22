// Package envvar resolves an environment variable that has been renamed.
package envvar

import (
	"log"
	"os"
	"strings"
	"sync"
)

var warned sync.Map

// Lookup returns the value of name, falling back to a superseded name.
//
// A superseded name is honoured and warned about, once per process. Neither
// silently accepting it nor silently ignoring it is acceptable: the first hides
// that a Terroir is running on an old spelling, the second loses configuration
// an operator believes is set.
func Lookup(name, superseded string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	value := strings.TrimSpace(os.Getenv(superseded))
	if value == "" {
		return ""
	}
	if _, already := warned.LoadOrStore(superseded, true); !already {
		log.Printf("⚠️  %s is superseded by %s. It still works; rename it.", superseded, name)
	}
	return value
}

// LookupBool is Lookup for a variable read as a boolean-ish string, preserving
// whether it was set at all.
func LookupBool(name, superseded string) (value string, present bool) {
	resolved := Lookup(name, superseded)
	return resolved, resolved != ""
}
