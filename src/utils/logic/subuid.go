package logic

import "fmt"

// Subordinate id range for rootless containers (single source — the
// duplicated literals lived in three places: misc.go, users.go and
// features/podman.go).
const (
	SubuidStart = 100000
	SubuidCount = 65536
)

// SubuidLines returns the /etc/subuid and /etc/subgid line for user —
// single owner of the literal format.
func SubuidLines(user string) (subuid, subgid string) {
	subuid = fmt.Sprintf("%s:%d:%d\n", user, SubuidStart, SubuidCount)
	subgid = fmt.Sprintf("%s:%d:%d\n", user, SubuidStart, SubuidCount)
	return subuid, subgid
}
