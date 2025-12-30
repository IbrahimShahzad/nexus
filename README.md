# nexus

An event-driven finite state machine library.

## Usage Steps

1. Define States
2. Define Actions
3. Define Event
4. Add transitions
5. Trigger events


see [examples](examples/)

## States

- The "initial" state is provided when calling New and is registered automatically. 
- You need to register any other states you want to use.

```go
machine := nexus.New[YourType]("initial_state")
_ = machine.RegisterState("state_name")
```

## Actions
Functions that run when a transition happens. Each action gets the context and your data, can modify the data, and should return an error if something goes wrong.

```go
action := nexus.Action[YourType]{
	Name: "descriptive_name",
	Fn: func(ctx context.Context, data *YourType) (*YourType, error) {
		// Do your work here
		return data, nil
	},
}
```

## Transitions
The rules: "when in state X and event Y happens, run these actions and go to state Z".

```go
machine.AddTransition("state_x", "state_z", "event_y", []nexus.Action[YourType]{action1, action2})
```

## Logging

Change the log level anytime:

```go
machine.SetLogLevel(slog.LevelDebug)
```

Or set it on creation:

```go
machine := nexus.New[MyType]("initial",
	nexus.WithLogLevel(slog.LevelDebug),
	nexus.WithLogger(myCustomLogger))
```

## Error Handling

You can set up a global error handler that catches any action failures:

```go
machine := nexus.New[MyType]("start")
machine.SetErrorHandler("error_state", func(ctx context.Context, data *MyType) (*MyType, error) {
	// Log it, clean up, panic, whatever floats your boat
	return data, nil
})
```

When any action fails, this handler runs and the FSM moves to the error state.


## Context

Actions receive context, so you can pass values or handle cancellation:

```go
action := nexus.Action[Request]{
	Name: "check_user",
	Fn: func(ctx context.Context, req *Request) (*Request, error) {
		userID := ctx.Value("user_id").(string)
		// Use userID for something
		return req, nil
	},
}

ctx := context.WithValue(context.Background(), "user_id", "12345")
machine.Trigger(ctx, "authenticate", req)
```

## API Reference

### Creating an FSM

```go
New[T any](initialState State, options ...OptionFunc) *FSM[T]
```
- `WithLogLevel(level zerolog.Level)`  - log level for the lib
- `WithLogOutput(w io.Writer)` - output
- `WithLogConsole()` - whether to use console writer or not. if not used, logs in json format
- `WithMaxStates(max int)` - Maximum number of states allowed (default 0 = unlimited)

### Core Methods

```go
RegisterState(state State)
```

- Add a state. You need to register all states before using them. 
- The initial state is auto-registered when creating the FSM by calling `New()`
- You cannot register the same state twice.
- You cannot remove state after registering.

```go
AddTransition(from, to State, event Event, actions []Action[T])
```

- Define a transition. Actions can be empty if you just want state changes.

```go
Trigger(ctx context.Context, event Event, args *T) (*T, error)
```

- Fire an event. Returns modified args and any error from actions.
- The return value is argument to the next action in the chain.

```go
GetState() State
```

- Current state

```go
SetState(s State)
```

- Manually set state (by Force). Use with caution. 
- better to use ErrorHandler to get to known states.

> [!WARNING]
> Bypasses the state machine 

```go
SetErrorHandler(errorState State, handler ActionFunc[T])
```

- Set up error handler function to be used if an error occurs during transition.

```go
SetLogLevel(level zerolog.Level)
```

- Change logging verbosity at runtime.

## License

See [LICENSE](LICENSE)

