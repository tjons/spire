package qb

type Filter string

const (
	Eq       Filter = "="
	In       Filter = "IN"
	Contains Filter = "CONTAINS"
	Neq      Filter = "!="
	Lt       Filter = "<"
	Gt       Filter = ">"
	Lte      Filter = "<="
	Gte      Filter = ">="
)

type FilterTerm struct {
	Column     string
	Operator   Filter
	Value      any
	Values     []any
	DeepValues [][]any
}

func Equals(value any) FilterTerm {
	return FilterTerm{
		Value: value,
	}
}
