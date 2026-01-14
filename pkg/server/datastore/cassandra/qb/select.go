package qb

import (
	"errors"
	"strconv"
	"strings"
)

func (b *Builder) Column(name string) SelectBuilder {
	if b.retrieveColumns == nil {
		b.retrieveColumns = make(map[string]struct{})
	}

	if _, exists := b.retrieveColumns[name]; exists {
		return b
	}

	b.retrieveColumns[name] = struct{}{}
	b.cols = append(b.cols, name)

	return b
}

func (b *Builder) Columns(names []string) SelectBuilder {
	if b.retrieveColumns == nil {
		b.retrieveColumns = make(map[string]struct{})
	}

	for _, name := range names {
		b.Column(name)
	}

	return b
}

func (b *Builder) From(table string) SelectBuilder {
	b.table = table

	return b
}

func (b *Builder) Where(column string, ft Filter, values ...any) SelectBuilder {
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

func (b *Builder) Limit(num uint) SelectBuilder {
	b.limit = num

	return b
}

func (b *Builder) Build() (string, error) {
	switch b.verb {
	case Select:
		return buildSelectFrom(b)
	}

	return "", errors.New("Not implemented")
}

func buildSelectFrom(b *Builder) (string, error) {
	q := strings.Builder{}
	q.WriteString("SELECT ")
	if b.isDistinct {
		q.WriteString("DISTINCT ")
	}

	for i, col := range b.cols { // ordinal is important for scanning
		if i > 0 {
			q.WriteString(", ")
		}
		q.WriteString(col)
	}
	q.WriteString(" FROM ")
	q.WriteString(b.table)

	for i := range b.filterTerms {
		if i == 0 {
			q.WriteString(" WHERE ")
		} else {
			q.WriteString(" AND ")
		}

		q.WriteString(b.filterTerms[i].Column)
		q.WriteString(" ")
		q.WriteString(string(b.filterTerms[i].Operator))
		q.WriteString(" ")
		q.WriteString("?")
		b.queryValues = append(b.queryValues, b.filterTerms[i].Value)
	}

	if b.limit > 0 {
		q.WriteString(" LIMIT ")
		q.WriteString(strconv.Itoa(int(b.limit)))
	}

	if b.allowFiltering {
		q.WriteString(" ALLOW FILTERING")
	}

	// TODO(tjons): grab these builders from a mempool
	// also, validate
	return q.String(), nil
}

func (b *Builder) AllowFiltering() SelectBuilder {
	b.allowFiltering = true
	return b
}

func (b *Builder) Distinct() SelectBuilder {
	b.isDistinct = true
	return b
}

type SelectBuilder interface {
	QueryBuilder

	Distinct() SelectBuilder
	Column(string) SelectBuilder
	Columns([]string) SelectBuilder
	From(string) SelectBuilder
	Where(string, Filter, ...any) SelectBuilder
	Limit(uint) SelectBuilder
	AllowFiltering() SelectBuilder
}

func NewSelect() SelectBuilder {
	return &Builder{verb: Select}
}
