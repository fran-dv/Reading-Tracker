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
	// ErrInFuture is returned when a session would end after now.
	ErrInFuture = errors.New("session ends in the future")
	// ErrTooLong is returned when a session would cover more than MaxSessionLength.
	ErrTooLong = errors.New("session is longer than 16 hours")
	// ErrPastEnd is returned when a position reached is past the item's size.
	ErrPastEnd = errors.New("position is past the end of the item")
	// ErrUnitLocked is returned by UpdateItem when a change of format would
	// change the unit an item's sessions are measured in.
	ErrUnitLocked = errors.New("the item's sessions are measured in another unit")
	// ErrNotEmpty is returned by Import when the library already holds data.
	ErrNotEmpty = errors.New("library is not empty")
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

// OverlapError is returned when a session would cover time another closed
// session already covers. With is that session, so the caller can name it.
type OverlapError struct {
	With Session
}

func (e *OverlapError) Error() string {
	return fmt.Sprintf("overlaps session %s", e.With.ID)
}
