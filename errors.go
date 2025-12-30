package nexus

import (
	"errors"
	"fmt"
)

// FSM operation errors
var (
	ErrInvalidState       = errors.New("invalid state")
	ErrInvalidEvent       = errors.New("invalid event")
	ErrNoTransition       = errors.New("no transition registered for state and event")
	ErrStateNotRegistered = errors.New("state not registered")
	ErrStateAlreadyExists = errors.New("state already exists")
	ErrStateSizeExceeded  = errors.New("maximum number of states exceeded")
)

// Action errors
var (
	ErrActionNil       = errors.New("action function is nil")
	ErrActionFailed    = errors.New("action execution failed")
	ErrNoActionDefined = errors.New("no action function defined")
)

// State transition errors
var (
	ErrTransitionFailed        = errors.New("state transition failed")
	ErrInvalidTransition       = errors.New("invalid state transition")
	ErrTransitionAlreadyExists = errors.New("transition already exists")
)

// FSM lifecycle errors
var (
	ErrFSMNotInitialized = errors.New("FSM not initialized")
	ErrFSMAlreadyRunning = errors.New("FSM already running")
	ErrFSMStopped        = errors.New("FSM has been stopped")
)

type StateError struct {
	State State
	Op    string
	Err   error
}

func (e *StateError) Error() string {
	return fmt.Sprintf("state error in state '%s' during %s: %v", e.State, e.Op, e.Err)
}

func (e *StateError) Unwrap() error {
	return e.Err
}

type TransitionError struct {
	Message string
	State   State
	Event   Event
	Err     error
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("transition error in state '%s' on event '%s': %s: %v",
		e.State, e.Event, e.Message, e.Err)
}

func (e *TransitionError) Unwrap() error {
	return e.Err
}

type ActionError struct {
	ActionName string
	State      string
	Event      string
	Err        error
}

func (e *ActionError) Error() string {
	return fmt.Sprintf("action error executing '%s' in state '%s' (event '%s'): %v",
		e.ActionName, e.State, e.Event, e.Err)
}

func (e *ActionError) Unwrap() error {
	return e.Err
}

type EventError struct {
	Event string
	State string
	Err   error
}

func (e *EventError) Error() string {
	return fmt.Sprintf("event error processing '%s' in state '%s': %v",
		e.Event, e.State, e.Err)
}

func (e *EventError) Unwrap() error {
	return e.Err
}
