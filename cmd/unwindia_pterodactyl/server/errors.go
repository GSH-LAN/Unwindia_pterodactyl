package server

import "fmt"

type InvalidGameError struct {
	Game string
}

func NewInvalidGameError(game string) *InvalidGameError {
	return &InvalidGameError{Game: game}
}

func (e *InvalidGameError) Error() string {
	return fmt.Sprintf("game %s is invalid", e.Game)
}
