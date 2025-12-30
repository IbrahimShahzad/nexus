package nexus

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestData struct {
	Value   string
	Counter int
}

func TestNew(t *testing.T) {
	initialState := State("initial")
	fsm := New[TestData](initialState)
	assert.NotNil(t, fsm)
	assert.Equal(t, initialState, fsm.GetState())
	assert.Equal(t, len(fsm.states.stateMap), 1)
	assert.Equal(t, initialState, fsm.states.Keys()[0])
}

func TestFSM_RegisterState(t *testing.T) {
	fsm := New[TestData](State("initial"))
	newState := State("new_state")
	fsm.RegisterState(newState)
	assert.Len(t, fsm.states.stateMap, 2)
	assert.Contains(t, fsm.states.stateMap, newState)
}

func TestFSM_AddTransition(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	fsm.RegisterState(state2)
	event := Event("trigger")
	fsm.AddTransition(State("state1"), state2, event, nil)
	assert.Len(t, fsm.transitions, 1)
	trans := fsm.transitions[0]
	assert.Equal(t, State("state1"), trans.From)
	assert.Equal(t, state2, trans.To)
	assert.Equal(t, event, trans.Event)
}

func TestFSM_SetGetState(t *testing.T) {
	fsm := New[TestData](State("initial"))
	newState := State("new_state")
	fsm.SetState(newState)
	assert.Equal(t, newState, fsm.GetState())
}

func TestFSM_Trigger_SimpleTransition(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	fsm.RegisterState(state2)

	event := Event("go_to_state2")

	// Add transition without action
	fsm.AddTransition(State("state1"), state2, event, nil)

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}

	result, err := fsm.Trigger(ctx, event, data)
	assert.NoError(t, err)
	assert.Equal(t, state2, fsm.GetState())
	assert.Equal(t, data, result)
}

func TestFSM_Trigger_WithAction(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	fsm.RegisterState(state2)

	event := Event("process")

	action := Action[TestData]{
		Name: "IncrementCounter",
		Fn: func(ctx context.Context, args *TestData) (*TestData, error) {
			args.Counter = args.Counter + 2
			args.Value = args.Value + "_processed"
			return args, nil
		},
	}

	fsm.AddTransition(State("state1"), state2, event, []Action[TestData]{action})

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}

	result, err := fsm.Trigger(ctx, event, data)
	assert.NoError(t, err)
	assert.Equal(t, state2, fsm.GetState())
	assert.Equal(t, 2, result.Counter)
	assert.Equal(t, "test_processed", result.Value)
}

func TestFSM_Trigger_WithMultipleActions(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	fsm.RegisterState(state2)

	event := Event("process")

	action1 := Action[TestData]{
		Name: "IncrementCounter",
		Fn: func(ctx context.Context, args *TestData) (*TestData, error) {
			args.Counter++
			return args, nil
		},
	}

	action2 := Action[TestData]{
		Name: "AppendValue",
		Fn: func(ctx context.Context, args *TestData) (*TestData, error) {
			args.Value = args.Value + "_step1"
			return args, nil
		},
	}

	action3 := Action[TestData]{
		Name: "AppendValue2",
		Fn: func(ctx context.Context, args *TestData) (*TestData, error) {
			args.Value = args.Value + "_step2"
			return args, nil
		},
	}
	fsm.AddTransition(State("state1"), state2, event, []Action[TestData]{action1, action2, action3})

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}
	result, err := fsm.Trigger(ctx, event, data)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Counter)
	expectedValue := "test_step1_step2"
	assert.Equal(t, expectedValue, result.Value)
}

func TestFSM_Trigger_NoTransitionError(t *testing.T) {
	fsm := New[TestData](State("state1"))

	event := Event("nonexistent_event")

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}

	_, err := fsm.Trigger(ctx, event, data)

	assert.NotNil(t, err)

	transErr, ok := err.(*TransitionError)
	assert.True(t, ok)
	if transErr != nil {
		assert.Equal(t, State("state1"), transErr.State)
		assert.Equal(t, event, transErr.Event)
	}

	assert.Equal(t, State("state1"), fsm.GetState())
}

func TestFSM_Trigger_ActionError(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	fsm.RegisterState(state2)

	event := Event("failing_process")
	expectedError := errors.New("action failed")

	action := Action[TestData]{
		Name: "FailingAction",
		Fn: func(ctx context.Context, args *TestData) (*TestData, error) {
			return args, expectedError
		},
	}

	fsm.AddTransition(State("state1"), state2, event, []Action[TestData]{action})

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}

	_, err := fsm.Trigger(ctx, event, data)

	assert.NotNil(t, err)
	assert.Equal(t, expectedError, err)
}

func TestFSM_Trigger_NilActionFunction(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	fsm.RegisterState(state2)

	event := Event("nil_action")

	action := Action[TestData]{
		Name: "NilAction",
		Fn:   nil,
	}

	fsm.AddTransition(State("state1"), state2, event, []Action[TestData]{action})

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}

	_, err := fsm.Trigger(ctx, event, data)
	assert.NotNil(t, err)
	transErr, ok := err.(*TransitionError)
	assert.True(t, ok)
	assert.NotNil(t, transErr)
	assert.Equal(t, transErr.State, State("state1"))
	assert.Equal(t, transErr.Event, event)
}

func TestFSM_ErrorHandler_WithErrorState(t *testing.T) {
	fsm := New[TestData](State("state1"))
	state2 := State("state2")
	errorState := State("error_state")
	fsm.RegisterState(state2)
	fsm.RegisterState(errorState)

	errorHandlerCalled := false
	errorHandler := func(ctx context.Context, args *TestData) (*TestData, error) {
		errorHandlerCalled = true
		args.Value = "error_handled"
		return args, nil
	}
	fsm.SetErrorHandler(errorState, errorHandler)

	event := Event("failing_event")
	expectedError := errors.New("action failed")

	action := Action[TestData]{
		Name: "FailingAction",
		Fn: func(ctx context.Context, args *TestData) (*TestData, error) {
			return args, expectedError
		},
	}
	fsm.AddTransition(State("state1"), state2, event, []Action[TestData]{action})

	ctx := context.Background()
	data := &TestData{Value: "test", Counter: 0}

	_, err := fsm.Trigger(ctx, event, data)

	assert.NotNil(t, err)
	assert.Equal(t, expectedError, err)

	assert.True(t, errorHandlerCalled)
	assert.Equal(t, errorState, fsm.GetState())
	assert.Equal(t, "error_handled", data.Value)
}

func TestTransitionError_Error(t *testing.T) {
	err := &TransitionError{
		Message: "test error",
		State:   State("test_state"),
		Event:   Event("test_event"),
		Err:     errors.New("underlying error"),
	}

	errorMsg := err.Error()
	expectedMsg := "transition error in state 'test_state' on event 'test_event': test error: underlying error"
	assert.Equal(t, expectedMsg, errorMsg)
}
