package provider

import "sort"

var registry = map[string]Provider{}

// Register adds a Provider to the registry, keyed by its Name(). Providers
// call this from an init() in their own package, e.g.:
//
//	func init() { provider.Register(New()) }
func Register(p Provider) {
	registry[p.Name()] = p
}

// Get looks up a registered provider by name.
func Get(name string) (Provider, bool) {
	p, ok := registry[name]
	return p, ok
}

// All returns every registered provider, sorted by name for stable output.
func All() []Provider {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Provider, 0, len(names))
	for _, name := range names {
		out = append(out, registry[name])
	}
	return out
}
