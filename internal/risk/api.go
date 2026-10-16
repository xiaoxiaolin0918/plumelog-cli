package risk

import "strings"

// ForAPI classifies an escape-hatch call from its method and path.
func ForAPI(method, path string) Policy {
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "GET" || m == "HEAD" {
		return ReadOnly()
	}
	switch segment(path) {
	case "deletequeue":
		return HighRisk(true, true)
	case "deleteindex":
		return HighRisk(true, true)
	case "resetindex", "deletebyquery", "deletewarningrule", "delextendfield":
		return HighRisk(false, false)
	case "login", "logout", "savewarningrulelist", "addextendfield", "sendlog", "sendrunlog", "sendtracelog":
		return Mutation()
	case "clientquery", "query", "queryappname", "queryappnames", "getappnames",
		"getwarningrulelist", "getqueuecounts", "getserverinfo", "getrunmodel", "getextendfieldlist":
		return ReadOnly()
	default:
		if m == "POST" || m == "PUT" || m == "PATCH" || m == "DELETE" {
			return Mutation()
		}
		return ReadOnly()
	}
}

func segment(path string) string {
	path = strings.Split(path, "?")[0]
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	return strings.ToLower(path)
}
