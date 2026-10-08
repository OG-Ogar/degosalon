package storage

import (
	"errors"
	"fmt"
)

const ListPageSize = 24

var ErrInvalidPage = errors.New("invalid results page")

type PageCursor struct{ After, Before, Limit int }
type Page[T any] struct {
	Items                []T
	HasPrevious, HasNext bool
}

func (c PageCursor) Normalize() (PageCursor, error) {
	if c.Limit == 0 {
		c.Limit = ListPageSize
	}
	if c.After < 0 || c.Before < 0 || c.After > 0 && c.Before > 0 || c.Limit < 1 || c.Limit > 100 {
		return c, ErrInvalidPage
	}
	return c, nil
}
func pageReverse(c PageCursor, ascending bool) bool {
	return ascending && c.Before > 0 || !ascending && c.After > 0
}

// column is a constant supplied by a storage query, never request input.
func pageSQL(column string, c PageCursor, ascending bool, args *[]any) string {
	clause := ""
	if c.After > 0 {
		*args = append(*args, c.After)
		clause += fmt.Sprintf(" and %s>$%d", column, len(*args))
	}
	if c.Before > 0 {
		*args = append(*args, c.Before)
		clause += fmt.Sprintf(" and %s<$%d", column, len(*args))
	}
	direction := "desc"
	if ascending != pageReverse(c, ascending) {
		direction = "asc"
	}
	*args = append(*args, c.Limit+1)
	return clause + fmt.Sprintf(" order by %s %s limit $%d", column, direction, len(*args))
}
func finishPage[T any](items []T, c PageCursor, ascending bool) Page[T] {
	extra := len(items) > c.Limit
	if extra {
		items = items[:c.Limit]
	}
	result := Page[T]{Items: items}
	if len(items) == 0 {
		return result
	}
	if pageReverse(c, ascending) {
		result.HasPrevious = extra
		result.HasNext = true
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	} else {
		result.HasPrevious = c.After > 0 || c.Before > 0
		result.HasNext = extra
	}
	return result
}
func cursorIncludes(id int, c PageCursor) bool {
	return (c.After == 0 || id > c.After) && (c.Before == 0 || id < c.Before)
}
