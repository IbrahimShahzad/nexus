package nexus

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
)

// State represents a state in the finite state machine.
type State string

// States manages a collection of unique states.
type States struct {
	stateMap map[State]struct{}
	maxSize  int
}

// NewStates creates a new States collection with maximum size.
func NewStates(size int) *States {
	return &States{
		stateMap: make(map[State]struct{}),
		maxSize:  size,
	}
}

// limitReached checks if the maximum number of states has been reached.
func (s *States) limitReached() bool {
	return s.maxSize > 0 && len(s.stateMap) >= s.maxSize
}

// Add adds a new state to the collection.
func (s *States) Add(state State) error {
	if s.Exists(state) {
		return &StateError{
			Op:    "Add",
			State: state,
			Err:   ErrStateAlreadyExists,
		}
	}

	if s.limitReached() {
		return &StateError{
			Op:    "Add",
			State: state,
			Err:   ErrStateSizeExceeded,
		}
	}
	s.stateMap[state] = struct{}{}
	return nil
}

// Exists checks if a state exists in the collection.
func (s *States) Exists(state State) bool {
	_, exists := s.stateMap[state]
	return exists
}

// Keys returns a slice of all registered states.
func (s *States) Keys() []State {
	keys := make([]State, 0, len(s.stateMap))
	for k := range s.stateMap {
		keys = append(keys, k)
	}
	return keys
}

// Event represents an event that triggers a state transition.
type Event string

// Action that can be executed during a state transition.
type Action[T any] struct {
	Name string
	Fn   ActionFunc[T]
}

// ActionFunc is a function that performs an action during a state transition.
type ActionFunc[T any] func(ctx context.Context, args *T) (*T, error)

// Transition triggered by an event.
type Transition[T any] struct {
	From   State
	To     State
	Event  Event
	Action []Action[T]
}

// FSMOptions holds configuration options for the FSM.
type FSMOptions struct {
	LogLevel  zerolog.Level
	LogOutput io.Writer
	maxStates int
	UseStdOut bool
}

// DefaultOptions returns the default FSM configuration.
func DefaultOptions() FSMOptions {
	return FSMOptions{
		LogLevel:  zerolog.InfoLevel,
		LogOutput: os.Stdout,
		maxStates: 0, // 0 means no limit
	}
}

type FSMOptionFunc func(*FSMOptions)

// WithLogLevel sets the log level for the FSM.
func WithLogLevel(level zerolog.Level) FSMOptionFunc {
	return func(opts *FSMOptions) {
		opts.LogLevel = level
	}
}

// WithLogOutput sets the writer where logs will be written.
func WithLogOutput(w io.Writer) FSMOptionFunc {
	return func(opts *FSMOptions) {
		opts.LogOutput = w
	}
}

// WithLogConsole switches the FSM logger to human-friendly console output.
func WithLogConsole() FSMOptionFunc {
	return func(opts *FSMOptions) {
		opts.UseStdOut = true
	}
}

// WithMaxStates sets the maximum number of states allowed in the FSM.
func WithMaxStates(max int) FSMOptionFunc {
	return func(opts *FSMOptions) {
		opts.maxStates = max
	}
}

// FSM is the Finite State Machine
type FSM[T any] struct {
	FSMOptions
	logger       zerolog.Logger
	states       *States
	mu           sync.RWMutex
	currentState State
	transitions  []Transition[T]
	errorState   State
	errorHandler ActionFunc[T]
}

// SetLogLevel updates the log level at runtime.
func (f *FSM[T]) SetLogLevel(level zerolog.Level) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LogLevel = level
	f.logger = f.logger.Level(level)
}

// setLogger initializes the logger based on options.
func setLogger(useStdOut bool, logOutput io.Writer, level zerolog.Level) zerolog.Logger {
	var baseLogger zerolog.Logger
	if useStdOut {
		cw := zerolog.ConsoleWriter{Out: logOutput}
		baseLogger = zerolog.New(cw).With().Timestamp().Str("component", "fsm").Logger()
		return baseLogger.Level(level)
	}

	baseLogger = zerolog.New(logOutput).With().Timestamp().Str("component", "fsm").Logger()
	return baseLogger.Level(level)
}

// New creates a new FSM instance
func New[T any](initialState State, options ...FSMOptionFunc) *FSM[T] {
	opts := DefaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	fsm := &FSM[T]{
		currentState: initialState,
		FSMOptions:   opts,
		logger:       setLogger(opts.UseStdOut, opts.LogOutput, opts.LogLevel),
		states:       NewStates(opts.maxStates),
		transitions:  make([]Transition[T], 0),
	}

	if err := fsm.RegisterState(initialState); err != nil {
		// This should never happen
		panic("failed to register initial state: " + err.Error())
	}

	fsm.logger.Info().
		Str("initialState", string(initialState)).
		Msg("FSM initialized")

	return fsm
}

// RegisterState adds a new state to the FSM.
func (f *FSM[T]) RegisterState(state State) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.states == nil {
		panic("FSM states slice is nil, this should not happen since it is initialized in New()")
	}

	err := f.states.Add(state)
	if err != nil {
		return err
	}

	f.logger.Debug().Str("state", string(state)).Msg("State registered")
	return nil
}

// AddTransition registers a new transition in the FSM from one state to another on a given event.
func (f *FSM[T]) AddTransition(from, to State, event Event, actions []Action[T]) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.transitions == nil {
		panic("FSM transitions slice is nil, this should not happen since it is initialized in New()")
	}

	f.transitions = append(f.transitions, Transition[T]{
		From:   from,
		To:     to,
		Event:  event,
		Action: actions,
	})

	actionNames := make([]string, len(actions))
	for i, a := range actions {
		actionNames[i] = a.Name
	}
	f.logger.Debug().
		Str("from", string(from)).
		Str("to", string(to)).
		Str("event", string(event)).
		Interface("actions", actionNames).
		Msg("Transition registered")
}

// Trigger attempts to transition the FSM to a new state based on the given event.
//
// Returns an error if no transition is registered for the current state or event, or if the action fails.
// If an error occurs and an error handler is configured, it will be called and the FSM will
// transition to the error state before returning the error.
func (f *FSM[T]) Trigger(ctx context.Context, event Event, args *T) (*T, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.Debug().Str("currentState", string(f.currentState)).Str("event", string(event)).Msg("Trigger called")

	var err error
	var nextState State
	var handlers []Action[T]
	transitionFound := false

	// TODO: Optimize this lookup with a map
	// maybe `map[State]map[Event]int`
	// the index can point to the transition in the slice
	for _, transition := range f.transitions {
		if transition.From == f.currentState && transition.Event == event {
			nextState = transition.To
			handlers = transition.Action
			transitionFound = true
			break
		}
	}

	if !transitionFound {
		err = &TransitionError{
			Message: "no transition found",
			State:   f.currentState,
			Event:   event,
			Err:     nil,
		}

		f.logger.Warn().
			Str("state", string(f.currentState)).
			Str("event", string(event)).
			Msg("No transition found")

		if f.errorHandler != nil || f.errorState != "" {
			f.handleError(ctx, args, err)
		}
		return args, err
	}

	f.logger.Info().Str("from", string(f.currentState)).Str("to", string(nextState)).Str("event", string(event)).Msg("Transitioning")

	for _, handler := range handlers {
		if handler.Fn == nil {
			err = &TransitionError{
				Message: "no handler function defined",
				State:   f.currentState,
				Event:   event,
				Err:     nil,
			}

			f.logger.Error().
				Str("action", handler.Name).
				Str("state", string(f.currentState)).
				Str("event", string(event)).
				Msg("Handler function is nil")

			if f.errorHandler != nil || f.errorState != "" {
				f.handleError(ctx, args, err)
			}
			return args, err
		}

		f.logger.Debug().Str("action", handler.Name).Str("state", string(f.currentState)).Str("event", string(event)).Msg("Executing action")

		if args, err = handler.Fn(ctx, args); err != nil {
			f.logger.Error().Err(err).
				Str("action", handler.Name).
				Str("state", string(f.currentState)).
				Str("event", string(event)).
				Msg("Action failed")

			if f.errorHandler != nil || f.errorState != "" {
				f.handleError(ctx, args, err)
			}
			return args, err
		}

		f.logger.Debug().Str("action", handler.Name).Msg("Action completed")
	}

	f.currentState = nextState

	f.logger.Info().Str("newState", string(f.currentState)).Msg("Transition completed")

	return args, nil
}

// handleError is called when an error occurs during a transition.
// It executes the error handler and transitions to the error state.
// NOTE: Should be called with the lock
func (f *FSM[T]) handleError(ctx context.Context, args *T, originalErr error) {
	if f.errorHandler != nil {
		_, err := f.errorHandler(ctx, args)
		if err != nil {
			ev := f.logger.Error().Err(err)
			if originalErr != nil {
				ev = ev.Str("originalError", originalErr.Error())
			}
			ev.Msg("Error in FSM error handler")
		}
	}
	if f.errorState != "" {
		f.currentState = f.errorState
	}
}

// GetState returns the current state of the FSM.
func (f *FSM[T]) GetState() State {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.currentState
}

// SetState sets the current state of the FSM.
//
// WARN: This bypasses the normal transition mechanism.
func (f *FSM[T]) SetState(s State) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.Warn().
		Str("oldState", string(f.currentState)).
		Str("newState", string(s)).
		Msg("State manually set (bypassing transitions)")
	f.currentState = s
}

// SetErrorHandler configures an error handler and error state.
// When a transition error occurs, the error handler will be called
// and the FSM will transition to the error state.
func (f *FSM[T]) SetErrorHandler(errorState State, handler ActionFunc[T]) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errorState = errorState
	f.errorHandler = handler
}
