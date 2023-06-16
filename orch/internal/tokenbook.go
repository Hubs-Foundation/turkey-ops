package internal

import (
	"sync"

	"k8s.io/utils/strings/slices"
)

type tokenBook struct {
	book []string
	mu   sync.Mutex
}

func NewTokenBook(bookSize int) *tokenBook {
	_book := []string{}
	for i := 0; i < bookSize; i++ {
		_book = append(_book, "")
	}
	return &tokenBook{
		book: _book,
	}
}

func (tb *tokenBook) NewToken(token string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	//shift right
	for i := len(tb.book) - 1; i > 0; i-- {
		tb.book[i] = tb.book[i-1]
	}
	tb.book[0] = token
}

func (tb *tokenBook) CheckToken(token string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return slices.Contains(tb.book, token)
}
