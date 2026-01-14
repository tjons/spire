package qb

type QueryType string

const (
	Insert QueryType = "INSERT"
	Update QueryType = "UPDATE"
	Select QueryType = "SELECT"
	Delete QueryType = "DELETE"
)

type QueryBuilder interface {
	Build() (string, error)
}

var supportedQueryTypes = map[QueryType]struct{}{
	Insert: {},
	Update: {},
	Select: {},
	Delete: {},
}

type Builder struct {
	cols            []string
	retrieveColumns map[string]struct{}
	verb            QueryType
	table           string
	filterTerms     []*FilterTerm
	limit           uint
	values          []any
	queryValues     []any
	allowFiltering  bool
	isDistinct      bool
}
