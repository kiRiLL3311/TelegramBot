package storage

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	e "test/lib"
)

type Storage interface {
	Save(p *Page) error
	PickRandom(username string) (*Page, error)
	Remove(p *Page) error
	IsExists(p *Page) (bool, error)
	ListPrepared(username string) (*[]Page, error)
}

var ErrNoSavedPages = errors.New("no saved pages")

type Page struct {
	URL      string
	Username string
}

func (p Page) Hash() (string, error) {
	h := sha1.New()

	if _, err := io.WriteString(h, p.URL); err != nil {
		return "", e.Wrap("cant calculate hash", err)
	}

	if _, err := io.WriteString(h, p.Username); err != nil {
		return "", e.Wrap("cant calculate hash", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
