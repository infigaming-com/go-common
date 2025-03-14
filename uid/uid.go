package uid

type UID interface {
	New() (string, error)
}
