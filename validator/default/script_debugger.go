package defaultvalidator

import (
	"fmt"

	"github.com/libsv/go-bt/v2/bscript/interpreter"
)

type LogDebugger struct{}

func (l *LogDebugger) BeforeExecute(state *interpreter.State) {
	fmt.Printf("BeforeExecute: %+v\n", state.Scripts)
}

func (l *LogDebugger) AfterExecute(state *interpreter.State) {
	fmt.Printf("AfterExecute: %+v\n", state.DataStack)
}

func (l *LogDebugger) BeforeStep(state *interpreter.State) {
	fmt.Printf("BeforeStep: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterStep(state *interpreter.State) {
	fmt.Printf("AfterStep: %+v\n", state.DataStack)
}

func (l *LogDebugger) BeforeExecuteOpcode(state *interpreter.State) {
	fmt.Printf("BeforeExecuteOpcode: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterExecuteOpcode(state *interpreter.State) {
	fmt.Printf("AfterExecuteOpcode: %+v\n", state.DataStack)
}

func (l *LogDebugger) BeforeScriptChange(state *interpreter.State) {
	fmt.Printf("BeforeScriptChange: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterScriptChange(state *interpreter.State) {
	fmt.Printf("AfterScriptChange: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterSuccess(state *interpreter.State) {
	fmt.Printf("AfterSuccess: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterError(state *interpreter.State, err error) {
	fmt.Printf("AfterError: %+v\n", state.DataStack)
}

func (l *LogDebugger) BeforeStackPush(state *interpreter.State, bytes []byte) {
	fmt.Printf("BeforeStackPush: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterStackPush(state *interpreter.State, bytes []byte) {
	fmt.Printf("AfterStackPush: %+v\n", state.DataStack)
}

func (l *LogDebugger) BeforeStackPop(state *interpreter.State) {
	fmt.Printf("BeforeStackPop: %+v\n", state.DataStack)
}

func (l *LogDebugger) AfterStackPop(state *interpreter.State, bytes []byte) {
	fmt.Printf("AfterStackPop: %+v\n", state.DataStack)
}
