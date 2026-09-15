package library

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when an item, shelf, session or rank does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalidTransition is returned when an item or session cannot move to the requested state.
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrWIPCapReached is returned by Start when settings.WIPCap items are already in progress.
	ErrWIPCapReached = errors.New("work-in-progress cap reached")
	// ErrHasSessions is returned by DeleteItem when the item has session history.
	ErrHasSessions = errors.New("item has sessions; abandon it instead")
	// ErrReasonRequired is returned by Abandon when no reason is given.
	ErrReasonRequired = errors.New("abandon reason is required")
	// ErrShelfNotEmpty is returned by DeleteShelf when items still live on the shelf.
	ErrShelfNotEmpty = errors.New("shelf is not empty")
	// ErrDuplicateShelf is returned when a shelf name is already taken (case-insensitive).
	ErrDuplicateShelf = errors.New("shelf name already exists")
	// ErrNotVisibleOnShelf is returned by Rank when the item is neither homed on nor borrowed by the shelf.
	ErrNotVisibleOnShelf = errors.New("item is not visible on this shelf")
	// ErrItemNotInProgress is returned when a session is recorded on an item that is not in progress.
	ErrItemNotInProgress = errors.New("item is not in progress")
	// ErrInvalidRange is returned when a session would end before or at its start.
	ErrInvalidRange = errors.New("session end must be after start")
	// ErrInvalidPositions is returned when only one of the two positions is given.
	ErrInvalidPositions = errors.New("positions must be both set or both empty")
)

// ValidationError reports a single invalid field.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Msg }

// SessionRunningError is returned when an action conflicts with the session
// that is currently running. ID identifies that session so the caller can
// offer to stop it.
type SessionRunningError struct {
	ID string
}

func (e *SessionRunningError) Error() string {
	return fmt.Sprintf("a session is already running (%s)", e.ID)
}
