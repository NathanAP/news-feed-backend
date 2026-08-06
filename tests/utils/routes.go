package utils

import "github.com/gofiber/fiber/v3"

// AddRoute registers one route from a handler chain, mirroring main.go's addRoute so the test setups
// wire routes exactly the way the application does.
//
// Fiber v3 changed registration from v2's fully variadic `Get(path, handlers ...Handler)` to
// `Get(path, handler any, handlers ...any)`. The names mislead: `Add` does
// `append([]any{handler}, handlers...)` and registers the result IN ORDER, so this is still an
// ordered chain and the first element still runs first. That matters here more than anywhere — if the
// split were backwards, authentication would run AFTER the handler and a test asserting 200 on an
// authenticated route would still pass while the route was wide open.
//
// The helper exists because Go cannot spread one slice across `(first, rest...)`.
func AddRoute(router fiber.Router, method, path string, chain []fiber.Handler) {
	rest := make([]any, 0, len(chain)-1)
	for _, handler := range chain[1:] {
		rest = append(rest, handler)
	}
	router.Add([]string{method}, path, chain[0], rest...)
}
