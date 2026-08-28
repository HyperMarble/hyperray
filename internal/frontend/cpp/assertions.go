package cpp

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// assertionCall is a lexical macro/function invocation. Its argument ranges
// refer to the original frozen source. Expressions are still obtained from
// Clang's typed AST; lexical discovery is used only because assertion
// frameworks intentionally erase their macro name during preprocessing.
type assertionCall struct {
	Name  string
	Start int
	End   int
	Args  []byteRange
}

type byteRange struct {
	Start int
	End   int
}

var assertionNames = map[string]bool{
	"assert":      true,
	"ASSERT_TRUE": true, "EXPECT_TRUE": true, "REQUIRE": true, "CHECK": true, "BOOST_CHECK": true,
	"ASSERT_FALSE": true, "EXPECT_FALSE": true, "REQUIRE_FALSE": true, "CHECK_FALSE": true,
	"ASSERT_EQ": true, "EXPECT_EQ": true, "BOOST_CHECK_EQUAL": true,
	"ASSERT_NE": true, "EXPECT_NE": true, "BOOST_CHECK_NE": true,
	"ASSERT_LT": true, "EXPECT_LT": true,
	"ASSERT_LE": true, "EXPECT_LE": true,
	"ASSERT_GT": true, "EXPECT_GT": true,
	"ASSERT_GE": true, "EXPECT_GE": true,
	"ASSERT_THROW": true, "EXPECT_THROW": true, "REQUIRE_THROWS_AS": true, "CHECK_THROWS_AS": true,
}

func scanAssertions(source []byte) []assertionCall {
	var calls []assertionCall
	for i := 0; i < len(source); {
		if source[i] == '/' && i+1 < len(source) && source[i+1] == '/' {
			i = skipLine(source, i+2)
			continue
		}
		if source[i] == '/' && i+1 < len(source) && source[i+1] == '*' {
			i = skipBlockComment(source, i+2)
			continue
		}
		if source[i] == '"' || source[i] == '\'' {
			i = skipQuoted(source, i)
			continue
		}
		if !identifierStart(source[i]) {
			i++
			continue
		}
		start := i
		for i < len(source) && identifierPart(source[i]) {
			i++
		}
		name := string(source[start:i])
		if !assertionNames[name] || preprocessorLine(source, start) {
			continue
		}
		open := i
		for open < len(source) && unicode.IsSpace(rune(source[open])) {
			open++
		}
		if open >= len(source) || source[open] != '(' {
			continue
		}
		args, end, ok := splitCallArguments(source, open)
		if !ok {
			continue
		}
		calls = append(calls, assertionCall{Name: name, Start: start, End: end, Args: args})
		i = end
	}
	return calls
}

func splitCallArguments(source []byte, open int) ([]byteRange, int, bool) {
	depth := 1
	argumentStart := open + 1
	var args []byteRange
	for i := open + 1; i < len(source); i++ {
		switch source[i] {
		case '"', '\'':
			i = skipQuoted(source, i) - 1
		case '/':
			if i+1 < len(source) && source[i+1] == '/' {
				i = skipLine(source, i+2) - 1
			} else if i+1 < len(source) && source[i+1] == '*' {
				i = skipBlockComment(source, i+2) - 1
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				args = append(args, trimRange(source, byteRange{Start: argumentStart, End: i}))
				if len(args) == 1 && args[0].Start == args[0].End {
					args = nil
				}
				return args, i + 1, true
			}
		case ',':
			if depth == 1 {
				args = append(args, trimRange(source, byteRange{Start: argumentStart, End: i}))
				argumentStart = i + 1
			}
		}
	}
	return nil, 0, false
}

func trimRange(source []byte, r byteRange) byteRange {
	for r.Start < r.End && unicode.IsSpace(rune(source[r.Start])) {
		r.Start++
	}
	for r.End > r.Start && unicode.IsSpace(rune(source[r.End-1])) {
		r.End--
	}
	return r
}

func preprocessorLine(source []byte, offset int) bool {
	lineStart := bytes.LastIndexByte(source[:offset], '\n') + 1
	for lineStart < offset && unicode.IsSpace(rune(source[lineStart])) {
		lineStart++
	}
	return lineStart < offset && source[lineStart] == '#'
}

func skipLine(source []byte, i int) int {
	for i < len(source) && source[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(source []byte, i int) int {
	for i+1 < len(source) {
		if source[i] == '*' && source[i+1] == '/' {
			return i + 2
		}
		i++
	}
	return len(source)
}

func skipQuoted(source []byte, start int) int {
	quote := source[start]
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\\' {
			i++
			continue
		}
		if source[i] == quote {
			return i + 1
		}
	}
	return len(source)
}

func identifierStart(b byte) bool { return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func identifierPart(b byte) bool  { return identifierStart(b) || b >= '0' && b <= '9' }

func (l *lowerer) assertionsWithin(r astRange) []assertionCall {
	var result []assertionCall
	for _, assertion := range l.assertions {
		if assertion.Start >= r.Begin.Offset && assertion.End <= r.End.Offset+r.End.TokLen {
			result = append(result, assertion)
		}
	}
	return result
}

func (l *lowerer) assertionForExpansion(node *astNode, assertions []assertionCall) *assertionCall {
	if node == nil {
		return nil
	}
	r := node.sourceRange()
	for i := range assertions {
		assertion := &assertions[i]
		if r.Begin.Offset == assertion.Start || r.Begin.Offset >= assertion.Start && r.End.Offset+r.End.TokLen <= assertion.End {
			return assertion
		}
	}
	return nil
}

func (l *lowerer) isAssertionExpansion(node *astNode, assertions []assertionCall) bool {
	return l.assertionForExpansion(node, assertions) != nil
}

func (l *lowerer) lowerCodeAssertion(call assertionCall, operationNode *astNode) []semanticir.Statement {
	assertion, ok := l.lowerAssertion(call, operationNode)
	if !ok {
		return nil
	}
	condition, ok := assertionPredicate(assertion)
	if !ok {
		l.blockAtRange(call.Start, call.End, "assertion-kind", fmt.Sprintf("assertion %s cannot be represented as a code guard", call.Name))
		return nil
	}
	raise := semanticir.Statement{
		Kind:          semanticir.StmtRaise,
		ExceptionType: "AssertionError",
		Message:       call.Name,
		Then:          []semanticir.Statement{},
		Else:          []semanticir.Statement{},
		Effects:       []semanticir.Effect{},
		Provenance:    assertion.Provenance,
	}
	l.accept()
	return []semanticir.Statement{{Kind: semanticir.StmtBranch, Condition: &condition, Then: []semanticir.Statement{}, Else: []semanticir.Statement{raise}, Effects: []semanticir.Effect{}, Provenance: assertion.Provenance}}
}

func assertionPredicate(assertion semanticir.Assertion) (semanticir.Expression, bool) {
	if assertion.Actual == nil {
		return semanticir.Expression{}, false
	}
	switch assertion.Kind {
	case semanticir.AssertTrue:
		return *assertion.Actual, true
	case semanticir.AssertFalse:
		return semanticir.Expression{Kind: semanticir.ExprUnary, Type: semanticir.TypeBool, Operator: semanticir.OpNot, Operands: []semanticir.Expression{*assertion.Actual}, Provenance: assertion.Provenance}, true
	case semanticir.AssertEqual, semanticir.AssertNotEqual:
		if assertion.Expected == nil {
			return semanticir.Expression{}, false
		}
		op := semanticir.OpEQ
		if assertion.Kind == semanticir.AssertNotEqual {
			op = semanticir.OpNE
		}
		return semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: op, Operands: []semanticir.Expression{*assertion.Actual, *assertion.Expected}, Provenance: assertion.Provenance}, true
	default:
		return semanticir.Expression{}, false
	}
}

func (l *lowerer) lowerAssertion(call assertionCall, operationNode *astNode) (semanticir.Assertion, bool) {
	prov := l.provenanceRange(call.Start, call.End, semanticir.TranslationTranslated)
	result := semanticir.Assertion{Provenance: prov, OutcomeIDs: []string{}}
	need := func(count int) bool {
		if len(call.Args) < count {
			l.blockAtRange(call.Start, call.End, "assertion-arity", fmt.Sprintf("assertion %s requires at least %d argument(s)", call.Name, count))
			return false
		}
		return true
	}
	lowerArg := func(index int) (*semanticir.Expression, bool) {
		node := findExpressionForRange(operationNode, call.Args[index])
		if node == nil {
			l.blockAtRange(call.Args[index].Start, call.Args[index].End, "assertion-expression", fmt.Sprintf("clang did not retain typed expression for argument %d of %s", index+1, call.Name))
			return nil, false
		}
		expr, ok := l.lowerExpression(node)
		return &expr, ok
	}

	switch call.Name {
	case "assert", "ASSERT_TRUE", "EXPECT_TRUE", "REQUIRE", "CHECK", "BOOST_CHECK":
		if !need(1) {
			return result, false
		}
		actual, ok := lowerArg(0)
		if !ok {
			return result, false
		}
		result.Kind, result.Actual = semanticir.AssertTrue, actual
	case "ASSERT_FALSE", "EXPECT_FALSE", "REQUIRE_FALSE", "CHECK_FALSE":
		if !need(1) {
			return result, false
		}
		actual, ok := lowerArg(0)
		if !ok {
			return result, false
		}
		result.Kind, result.Actual = semanticir.AssertFalse, actual
	case "ASSERT_EQ", "EXPECT_EQ", "BOOST_CHECK_EQUAL", "ASSERT_NE", "EXPECT_NE", "BOOST_CHECK_NE":
		if !need(2) {
			return result, false
		}
		actual, ok := lowerArg(0)
		if !ok {
			return result, false
		}
		expected, ok := lowerArg(1)
		if !ok {
			return result, false
		}
		result.Kind, result.Actual, result.Expected = semanticir.AssertEqual, actual, expected
		if strings.Contains(call.Name, "_NE") || call.Name == "BOOST_CHECK_NE" {
			result.Kind = semanticir.AssertNotEqual
		}
	case "ASSERT_LT", "EXPECT_LT", "ASSERT_LE", "EXPECT_LE", "ASSERT_GT", "EXPECT_GT", "ASSERT_GE", "EXPECT_GE":
		if !need(2) {
			return result, false
		}
		left, ok := lowerArg(0)
		if !ok {
			return result, false
		}
		right, ok := lowerArg(1)
		if !ok {
			return result, false
		}
		op := map[string]semanticir.Operator{"ASSERT_LT": semanticir.OpLT, "EXPECT_LT": semanticir.OpLT, "ASSERT_LE": semanticir.OpLE, "EXPECT_LE": semanticir.OpLE, "ASSERT_GT": semanticir.OpGT, "EXPECT_GT": semanticir.OpGT, "ASSERT_GE": semanticir.OpGE, "EXPECT_GE": semanticir.OpGE}[call.Name]
		comparison := semanticir.Expression{Kind: semanticir.ExprCompare, Type: semanticir.TypeBool, Operator: op, Operands: []semanticir.Expression{*left, *right}, Provenance: prov}
		result.Kind, result.Actual = semanticir.AssertTrue, &comparison
	case "ASSERT_THROW", "EXPECT_THROW", "REQUIRE_THROWS_AS", "CHECK_THROWS_AS":
		if !need(2) {
			return result, false
		}
		actual, ok := lowerArg(0)
		if !ok {
			return result, false
		}
		result.Kind, result.Actual = semanticir.AssertRaises, actual
		result.ExceptionType = strings.TrimSpace(string(l.request.Source[call.Args[1].Start:call.Args[1].End]))
		if result.ExceptionType == "" {
			l.blockAtRange(call.Args[1].Start, call.Args[1].End, "assertion-exception", "throw assertion has empty exception type")
			return result, false
		}
	default:
		l.blockAtRange(call.Start, call.End, "assertion-framework", fmt.Sprintf("assertion %s is unsupported", call.Name))
		return result, false
	}
	l.accept()
	return result, true
}

func findExpressionForRange(root *astNode, target byteRange) *astNode {
	var best *astNode
	bestWidth := -1
	var visit func(*astNode)
	visit = func(node *astNode) {
		if node == nil {
			return
		}
		r, ok := nodeSpellingRange(node)
		if ok && r.Start >= target.Start && r.End <= target.End && r.End > r.Start && expressionNodeKind(node.Kind) {
			width := r.End - r.Start
			if width > bestWidth {
				best, bestWidth = node, width
			}
		}
		for _, child := range node.Inner {
			visit(child)
		}
	}
	visit(root)
	return best
}

func nodeSpellingRange(node *astNode) (byteRange, bool) {
	if node == nil {
		return byteRange{}, false
	}
	begin := node.Range.Begin.spelling()
	end := node.Range.End.spelling()
	if begin.Offset < 0 || end.Offset < begin.Offset {
		return byteRange{}, false
	}
	return byteRange{Start: begin.Offset, End: end.Offset + max(1, end.TokLen)}, true
}

func expressionNodeKind(kind string) bool {
	return strings.HasSuffix(kind, "Expr") || strings.HasSuffix(kind, "Literal") || kind == "BinaryOperator" || kind == "UnaryOperator" || kind == "ConstantExpr"
}

func (l *lowerer) blockAtRange(start, end int, kind, reason string) {
	prov := l.provenanceRange(start, end, semanticir.TranslationUnsupported)
	key := fmt.Sprintf("%s:%d:%d:%s", kind, prov.Location.StartLine, prov.Location.StartColumn, reason)
	if l.blockedKeys[key] {
		return
	}
	l.blockedKeys[key] = true
	l.total++
	l.unsupported = append(l.unsupported, semanticir.UnsupportedConstruct{Kind: kind, Reason: reason, Provenance: prov})
	l.diagnostics = append(l.diagnostics, semanticir.Diagnostic{Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported, Message: reason, Provenance: prov})
}

func (l *lowerer) provenanceRange(start, end int, status semanticir.TranslationStatus) semanticir.Provenance {
	startLine, startColumn := offsetLineColumn(l.request.Source, start)
	endLine, endColumn := offsetLineColumn(l.request.Source, max(start, end-1))
	return semanticir.NewProvenance(l.request.Artifact, semanticir.SourceLocation{Path: l.request.Artifact.Path, StartLine: startLine, StartColumn: startColumn, EndLine: endLine, EndColumn: endColumn}, status)
}
