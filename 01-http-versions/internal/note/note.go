package note

import (
	"errors"
	"strings"
	"time"
)

type Title string

const MaxTitleLen = 200

var (
	ErrEmptyTitle   = errors.New("title is empty")
	ErrTitleTooLong = errors.New("title is too long")
)

func TitleFromString(s string) (Title, error) {
	t := Title(strings.TrimSpace(s))
	if err := ValidateTitle(t); err != nil {
		return "", err
	}
	return t, nil
}

func TitleToString(t Title) string {
	return string(t)
}

func ValidateTitle(t Title) error {
	if t == "" {
		return ErrEmptyTitle
	}
	if len([]rune(t)) > MaxTitleLen {
		return ErrTitleTooLong
	}
	return nil
}

type Note struct {
	Key       string    `json:"key" xml:"key"`
	Title     Title     `json:"title" xml:"title"`
	Value     string    `json:"value" xml:"value"`
	UpdatedAt time.Time `json:"updated_at" xml:"updated_at"`
}
