package utils

import (
	"reflect"
	"runtime"
	"strings"
)

// GetTypeName returns the name of the type for the current function.
// It uses runtime reflection to get the full function name and extracts the type part.
//
// Example usage:
//
//  1. For a method reference:
//     ```go
//     // Inside a handler method
//     func (h *myHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
//     typeName := utils.GetTypeName(h.handleCommand)
//     // typeName will be "myHandler"
//     log.Printf("%s: Processing command", typeName)
//     // ...
//     }
//     ```
//
//  2. Alternatively, use GetCurrentTypeName for the current function:
//     ```go
//     func (h *myHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
//     typeName := utils.GetCurrentTypeName()
//     // typeName will be "myHandler"
//     log.Printf("%s: Processing command", typeName)
//     // ...
//     }
//     ```
func GetTypeName(i interface{}) string {
	// Get the full function name through reflection
	fullName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()

	// Split the name by dot to get the components
	parts := strings.Split(fullName, ".")

	// The type name should be the second-to-last part, typically in the form "(*typeName)"
	if len(parts) < 2 {
		return ""
	}

	typePart := parts[len(parts)-2]

	// Remove the pointer notation if present
	typeName := strings.TrimPrefix(strings.TrimSuffix(typePart, ")"), "(*")

	return typeName
}

// GetCurrentTypeName returns the name of the type for the current function.
func GetCurrentTypeName() string {
	// Get the program counter and function data for the caller
	pc, _, _, ok := runtime.Caller(1) // skip 1 for the current function
	if !ok {
		return ""
	}

	// Get the full function name
	fullName := runtime.FuncForPC(pc).Name()

	// Split the name by dot to get the components
	parts := strings.Split(fullName, ".")

	// The type name should be the second-to-last part, typically in the form "(*typeName)"
	if len(parts) < 2 {
		return ""
	}

	typePart := parts[len(parts)-2]

	// Remove the pointer notation if present
	typeName := strings.TrimPrefix(strings.TrimSuffix(typePart, ")"), "(*")

	return typeName
}
