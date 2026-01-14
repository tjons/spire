package qb

import "strings"

type DeleteBuilder interface {
	From(table string) DeleteBuilder
	Where(column string, ft Filter, values ...any) DeleteBuilder
	IfExists() DeleteBuilder
	Build() (string, error)
}

type deleteBuilder struct {
	table       string
	filterTerms []*FilterTerm
	ifExists    bool
}

func NewDelete() DeleteBuilder {
	return &deleteBuilder{}
}

func (b *deleteBuilder) IfExists() DeleteBuilder {
	b.ifExists = true

	return b
}

func (b *deleteBuilder) From(table string) DeleteBuilder {
	b.table = table

	return b
}

func (b *deleteBuilder) Where(column string, ft Filter, values ...any) DeleteBuilder {
	t := &FilterTerm{
		Column:   column,
		Operator: ft,
	}

	if len(values) == 1 {
		t.Value = values[0]
	}

	// handle when multivalue

	b.filterTerms = append(b.filterTerms, t)

	return b
}

func (b *deleteBuilder) Build() (string, error) {
	return buildDeleteFrom(b)
}

func buildDeleteFrom(b *deleteBuilder) (string, error) {
	var sb strings.Builder

	sb.WriteString("DELETE FROM ")
	sb.WriteString(b.table)

	for i := range b.filterTerms {
		if i == 0 {
			sb.WriteString(" WHERE ")
		} else {
			sb.WriteString(" AND ")
		}

		sb.WriteString(b.filterTerms[i].Column)
		sb.WriteString(" ")
		sb.WriteString(string(b.filterTerms[i].Operator))
		sb.WriteString(" ?")
	}

	if b.ifExists {
		sb.WriteString(" IF EXISTS")
	}

	return sb.String(), nil
}
