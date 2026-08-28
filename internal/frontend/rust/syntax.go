package rust

import "fmt"

type sourcePos struct {
	Offset int
	Line   int
	Column int
}

type sourceSpan struct {
	Start sourcePos
	End   sourcePos
}

func (s sourceSpan) String() string {
	return fmt.Sprintf("%d:%d-%d:%d", s.Start.Line, s.Start.Column, s.End.Line, s.End.Column)
}

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenNumber
	tokenString
	tokenChar
	tokenPunct
)

type token struct {
	Kind tokenKind
	Text string
	Span sourceSpan
}

type functionDecl struct {
	Name       string
	Parameters []parameter
	ReturnType string
	IsTest     bool
	Body       block
	Span       sourceSpan
}

type parameter struct {
	Name string
	Type string
	Span sourceSpan
}

type block struct {
	Statements []statement
	Tail       *expression
	Span       sourceSpan
}

type statementKind uint8

const (
	statementLet statementKind = iota
	statementExpr
	statementReturn
	statementAssign
	statementFor
)

type statement struct {
	Kind    statementKind
	Name    string
	Type    string
	Mutable bool
	Expr    expression
	Body    *block
	Span    sourceSpan
}

type expressionKind uint8

const (
	expressionIdentifier expressionKind = iota
	expressionLiteral
	expressionUnary
	expressionBinary
	expressionCall
	expressionMacro
	expressionIf
	expressionMatch
	expressionBlock
	expressionTuple
	expressionRange
)

type expression struct {
	Kind     expressionKind
	Text     string
	Children []expression
	Then     *block
	Else     *block
	Arms     []matchArm
	Span     sourceSpan
}

type matchArm struct {
	Pattern string
	Guard   *expression
	Value   expression
	Span    sourceSpan
}

type parseIssue struct {
	Code    string
	Message string
	Span    sourceSpan
}

func mergeSpan(a, b sourceSpan) sourceSpan {
	return sourceSpan{Start: a.Start, End: b.End}
}
