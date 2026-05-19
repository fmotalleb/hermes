package api

type repository struct{}

func newRepository() *repository {
	return new(repository)
}
