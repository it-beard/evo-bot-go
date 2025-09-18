package bot

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseRegisterHandlerConstructors returns the set of constructor names (selectors that start with "New")
// that are invoked inside registerHandlers in bot.go.
func parseRegisterHandlerConstructors(t *testing.T) map[string]struct{} {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bot.go", nil, 0)
	assert.NoError(t, err)

	constructors := make(map[string]struct{})

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "registerHandlers" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if name := sel.Sel.Name; len(name) >= 3 && name[:3] == "New" {
						constructors[name] = struct{}{}
					}
				}
			}
			return true
		})
	}
	return constructors
}

// expectedConstructors is the canonical list that must be present in registerHandlers.
var expectedConstructors = []string{
	// Start
	"NewStartHandler",

	// Admin
	"NewEventDeleteHandler",
	"NewEventEditHandler",
	"NewEventSetupHandler",
	"NewEventStartHandler",
	"NewTryCreateCoffeePoolHandler",
	"NewTryGenerateCoffeePairsHandler",
	"NewTrySummarizeHandler",
	"NewAdminProfilesHandler",
	"NewShowTopicsHandler",

	// Group
	"NewSaveTopicsHandler",
	"NewAdminMessageControlHandler",
	"NewSaveMessagesHandler",
	"NewCleanClosedThreadsHandler",
	"NewDeleteJoinLeftMessagesHandler",
	"NewJoinLeftHandler",
	"NewRandomCoffeePollAnswerHandler",
	"NewRepliesFromClosedThreadsHandler",

	// Private
	"NewTopicAddHandler",
	"NewTopicsHandler",
	"NewContentHandler",
	"NewEventsHandler",
	"NewHelpHandler",
	"NewIntroHandler",
	"NewProfileHandler",
	"NewToolsHandler",
}

// TestRegisterHandlers_ExpectedConstructors runs a sub-test for every expected constructor.
func TestRegisterHandlers_ExpectedConstructors(t *testing.T) {
	found := parseRegisterHandlerConstructors(t)

	for _, ctor := range expectedConstructors {
		ctor := ctor // capture range variable
		t.Run(ctor, func(t *testing.T) {
			if _, ok := found[ctor]; !ok {
				t.Errorf("handler constructor %s is NOT registered in registerHandlers", ctor)
			}
		})
	}
}

// TestRegisterHandlers_NoUnexpectedConstructors fails if registerHandlers contains a constructor
// that is not in the expected list.
func TestRegisterHandlers_NoUnexpectedConstructors(t *testing.T) {
	found := parseRegisterHandlerConstructors(t)

	expectedSet := make(map[string]struct{}, len(expectedConstructors))
	for _, ctor := range expectedConstructors {
		expectedSet[ctor] = struct{}{}
	}

	var unexpected []string
	for ctor := range found {
		if _, ok := expectedSet[ctor]; !ok {
			unexpected = append(unexpected, ctor)
		}
	}
	sort.Strings(unexpected)

	if len(unexpected) > 0 {
		t.Fatalf("unexpected handler constructors in registerHandlers: %v", unexpected)
	}
}
