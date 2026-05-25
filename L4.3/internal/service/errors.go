package service

import "errors"

var (
	// ErrInvalidInput возвращается при ошибках входных данных.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound возвращается, если событие не найдено.
	ErrNotFound = errors.New("event not found")
)
