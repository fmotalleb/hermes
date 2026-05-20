package queries

type scanner interface {
	Scan(dest ...any) error
}
