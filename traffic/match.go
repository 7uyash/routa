package traffic

import "strings"

// Canonical request matching for Routa's user-supplied rules.
//
// Routes, traffic mutations, network simulation and mocks all match requests
// against patterns typed by a user, so they all go through MatchPath and
// MatchRequest. Before this existed each feature had its own matcher with its
// own idea of what "*" meant, which meant the same pattern could route a
// request one way while a mutation or mock on that same pattern stayed silent.
//
// # Path patterns
//
//	""         match every path
//	"*"        match every path
//	"/*"       match every path (the default catch-all route)
//	"/api/*"   "/api", and anything below it, but not "/apifoo"
//	"/api*"    anything beginning with "/api", including "/apifoo"
//	"/health"  exactly "/health"
//
// A pattern ending in "/*" is segment aware: the slash before the star is a
// segment boundary, so "/api/*" does not match "/apifoo". A pattern ending in
// a bare "*" is a plain string prefix, so "/api*" does match "/apifoo". A
// pattern with no trailing star is compared for exact equality, and a star
// anywhere other than the end is literal.
//
// An empty pattern matches everything, which is what config.MatchConfig
// documents and what the dashboard shows as "*".
//
// # Method patterns
//
// An empty method or "*" matches any method. Comparison is case insensitive,
// so a rule written as "get" still matches a GET request.
func MatchPath(pattern, path string) bool {
	switch pattern {
	case "", "*", "/*":
		return true
	}

	// Segment aware: the prefix itself, or anything below it.
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}

	// Plain string prefix.
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(path, prefix)
	}

	return pattern == path
}

// MatchMethod reports whether a rule's method pattern matches a request method.
// An empty pattern or "*" matches any method, and comparison is case
// insensitive.
func MatchMethod(pattern, method string) bool {
	return pattern == "" || pattern == "*" || strings.EqualFold(pattern, method)
}

// MatchRequest reports whether req satisfies a rule's method and path pattern.
// Both patterns are optional: an empty method or path matches anything.
func MatchRequest(method, pattern string, req Request) bool {
	return MatchMethod(method, req.Method) && MatchPath(pattern, req.Path)
}
