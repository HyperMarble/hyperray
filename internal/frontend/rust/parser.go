package rust

import (
	"fmt"
	"strings"
)

type parser struct {
	tokens []token
	index  int
	issues []parseIssue
}

func parseRust(source []byte) ([]functionDecl, []parseIssue) {
	tokens, issues := lex(source)
	p := parser{tokens: tokens, issues: issues}
	functions := p.parseFile()
	return functions, p.issues
}

func (p *parser) parseFile() []functionDecl {
	var functions []functionDecl
	var attributes []string
	for !p.atEOF() {
		if p.match("#") {
			attribute, ok := p.parseAttribute()
			if ok {
				attributes = append(attributes, attribute)
			}
			continue
		}
		if p.match("use") {
			p.skipBalancedUntil(";")
			attributes = nil
			continue
		}
		if p.match("pub") {
			if p.match("(") {
				p.skipBalancedGroup("(", ")")
			}
		}
		if p.match("fn") {
			if fn, ok := p.parseFunction(attributes); ok {
				functions = append(functions, fn)
			}
			attributes = nil
			continue
		}

		tok := p.current()
		code := "RUST_UNSUPPORTED_ITEM"
		message := fmt.Sprintf("unsupported top-level Rust construct starting at %s", tokenDescription(tok))
		switch tok.Text {
		case "unsafe":
			code, message = "RUST_UNSAFE", "unsafe Rust is outside the bounded frontend"
		case "extern":
			code, message = "RUST_FFI", "extern/FFI declarations are outside the bounded frontend"
		case "impl", "trait", "struct", "enum", "type", "const", "static", "mod":
			message = fmt.Sprintf("%s items are not translated by the bounded Rust frontend", tok.Text)
		}
		p.issue(code, message, tok.Span)
		p.recoverTopLevel()
		attributes = nil
	}
	return functions
}

func (p *parser) parseAttribute() (string, bool) {
	start := p.previous().Span
	if !p.expect("[", "RUST_INVALID_ATTRIBUTE", "expected '[' after '#'") {
		return "", false
	}
	var parts []token
	depth := 1
	for !p.atEOF() && depth > 0 {
		tok := p.advance()
		switch tok.Text {
		case "[":
			depth++
		case "]":
			depth--
			if depth == 0 {
				return joinTokens(parts), true
			}
		}
		if depth > 0 {
			parts = append(parts, tok)
		}
	}
	p.issue("RUST_INVALID_ATTRIBUTE", "unterminated Rust attribute", start)
	return "", false
}

func (p *parser) parseFunction(attributes []string) (functionDecl, bool) {
	start := p.previous().Span
	name := p.current()
	if name.Kind != tokenIdent {
		p.issue("RUST_INVALID_FUNCTION", "expected function name", name.Span)
		p.recoverTopLevel()
		return functionDecl{}, false
	}
	p.advance()
	if p.check("<") {
		p.issue("RUST_UNSUPPORTED_GENERICS", "generic functions require monomorphized input", p.current().Span)
		p.recoverTopLevel()
		return functionDecl{}, false
	}
	if !p.expect("(", "RUST_INVALID_FUNCTION", "expected '(' after function name") {
		p.recoverTopLevel()
		return functionDecl{}, false
	}
	parameters, ok := p.parseParameters()
	if !ok {
		p.recoverTopLevel()
		return functionDecl{}, false
	}

	returnType := "()"
	if p.match("->") {
		typeTokens := p.collectUntilTopLevel("{", "where")
		if len(typeTokens) == 0 {
			p.issue("RUST_INVALID_FUNCTION", "missing function return type", p.current().Span)
			return functionDecl{}, false
		}
		returnType = joinTokens(typeTokens)
	}
	if p.match("where") {
		p.issue("RUST_UNSUPPORTED_WHERE", "where clauses require monomorphized input", p.previous().Span)
		p.recoverTopLevel()
		return functionDecl{}, false
	}
	if !p.check("{") {
		p.issue("RUST_INVALID_FUNCTION", "expected function body", p.current().Span)
		p.recoverTopLevel()
		return functionDecl{}, false
	}
	body, ok := p.parseBlock()
	if !ok {
		return functionDecl{}, false
	}
	fn := functionDecl{
		Name:       name.Text,
		Parameters: parameters,
		ReturnType: returnType,
		IsTest:     hasTestAttribute(attributes),
		Body:       body,
		Span:       mergeSpan(start, body.Span),
	}
	for _, attr := range attributes {
		if attr != "test" && attr != "cfg(test)" {
			p.issue("RUST_UNSUPPORTED_ATTRIBUTE", fmt.Sprintf("attribute #[%s] is not translated", attr), start)
		}
	}
	return fn, true
}

func (p *parser) parseParameters() ([]parameter, bool) {
	var parameters []parameter
	if p.match(")") {
		return parameters, true
	}
	for !p.atEOF() {
		start := p.current().Span
		if p.match("&") || p.check("self") {
			p.issue("RUST_UNSUPPORTED_RECEIVER", "method receivers are not supported; translate a free function", start)
			p.skipBalancedUntil(",", ")")
			return nil, false
		}
		if p.match("mut") {
			start = mergeSpan(start, p.previous().Span)
		}
		name := p.current()
		if name.Kind != tokenIdent {
			p.issue("RUST_INVALID_PARAMETER", "expected parameter name", name.Span)
			return nil, false
		}
		p.advance()
		if !p.expect(":", "RUST_INVALID_PARAMETER", "expected ':' after parameter name") {
			return nil, false
		}
		typeTokens := p.collectUntilTopLevel(",", ")")
		if len(typeTokens) == 0 {
			p.issue("RUST_INVALID_PARAMETER", "expected parameter type", p.current().Span)
			return nil, false
		}
		parameters = append(parameters, parameter{Name: name.Text, Type: joinTokens(typeTokens), Span: mergeSpan(start, typeTokens[len(typeTokens)-1].Span)})
		if p.match(")") {
			return parameters, true
		}
		if !p.expect(",", "RUST_INVALID_PARAMETER", "expected ',' between parameters") {
			return nil, false
		}
		if p.match(")") {
			return parameters, true
		}
	}
	p.issue("RUST_INVALID_PARAMETER", "unterminated parameter list", p.current().Span)
	return nil, false
}

func (p *parser) parseBlock() (block, bool) {
	open := p.current()
	if !p.expect("{", "RUST_INVALID_BLOCK", "expected '{'") {
		return block{}, false
	}
	result := block{Span: open.Span}
	for !p.atEOF() && !p.check("}") {
		if p.match("for") {
			stmt, ok := p.parseFor(p.previous().Span)
			if !ok {
				p.recoverStatement()
				continue
			}
			result.Statements = append(result.Statements, stmt)
			continue
		}
		if p.match("let") {
			stmt, ok := p.parseLet(p.previous().Span)
			if !ok {
				p.recoverStatement()
				continue
			}
			result.Statements = append(result.Statements, stmt)
			continue
		}
		if p.current().Kind == tokenIdent && p.index+1 < len(p.tokens) && p.tokens[p.index+1].Text == "=" {
			start := p.advance()
			p.advance()
			value, ok := p.parseExpression(0)
			if !ok || !p.expect(";", "RUST_INVALID_ASSIGNMENT", "assignment must end with ';'") {
				p.recoverStatement()
				continue
			}
			result.Statements = append(result.Statements, statement{Kind: statementAssign, Name: start.Text, Expr: value, Span: mergeSpan(start.Span, p.previous().Span)})
			continue
		}
		if p.match("return") {
			start := p.previous().Span
			var value expression
			if p.check(";") {
				value = expression{Kind: expressionTuple, Text: "()", Span: p.current().Span}
			} else {
				var ok bool
				value, ok = p.parseExpression(0)
				if !ok {
					p.recoverStatement()
					continue
				}
			}
			if !p.expect(";", "RUST_INVALID_RETURN", "return expressions must end with ';'") {
				p.recoverStatement()
			}
			result.Statements = append(result.Statements, statement{Kind: statementReturn, Expr: value, Span: mergeSpan(start, value.Span)})
			continue
		}

		expr, ok := p.parseExpression(0)
		if !ok {
			p.recoverStatement()
			continue
		}
		if p.match(";") {
			result.Statements = append(result.Statements, statement{Kind: statementExpr, Expr: expr, Span: mergeSpan(expr.Span, p.previous().Span)})
			continue
		}
		if expr.Kind == expressionIf || expr.Kind == expressionMatch || expr.Kind == expressionBlock {
			// Rust block expressions may be used as statements without a
			// trailing semicolon when another statement follows.
			if !p.check("}") {
				result.Statements = append(result.Statements, statement{Kind: statementExpr, Expr: expr, Span: expr.Span})
				continue
			}
		}
		if p.check("}") {
			result.Tail = &expr
			break
		}
		p.issue("RUST_INVALID_STATEMENT", "expected ';' or end of block after expression", p.current().Span)
		p.recoverStatement()
	}
	if !p.expect("}", "RUST_INVALID_BLOCK", "unterminated block") {
		return result, false
	}
	result.Span = mergeSpan(open.Span, p.previous().Span)
	return result, true
}

func (p *parser) parseLet(start sourceSpan) (statement, bool) {
	mutable := p.match("mut")
	name := p.current()
	if name.Kind != tokenIdent {
		p.issue("RUST_UNSUPPORTED_PATTERN", "only identifier let-bindings are supported", name.Span)
		return statement{}, false
	}
	p.advance()
	declaredType := ""
	if p.match(":") {
		typeTokens := p.collectUntilTopLevel("=")
		if len(typeTokens) == 0 {
			p.issue("RUST_INVALID_LET", "missing type after ':'", p.current().Span)
			return statement{}, false
		}
		declaredType = joinTokens(typeTokens)
	}
	if !p.expect("=", "RUST_INVALID_LET", "bounded let-bindings require an initializer") {
		return statement{}, false
	}
	value, ok := p.parseExpression(0)
	if !ok {
		return statement{}, false
	}
	if !p.expect(";", "RUST_INVALID_LET", "let-bindings must end with ';'") {
		return statement{}, false
	}
	return statement{Kind: statementLet, Name: name.Text, Type: declaredType, Mutable: mutable, Expr: value, Span: mergeSpan(start, p.previous().Span)}, true
}

func (p *parser) parseFor(start sourceSpan) (statement, bool) {
	name := p.current()
	if name.Kind != tokenIdent {
		p.issue("RUST_UNSUPPORTED_PATTERN", "bounded for-loop target must be one identifier", name.Span)
		return statement{}, false
	}
	p.advance()
	if !p.expect("in", "RUST_INVALID_LOOP", "expected 'in' after for-loop target") {
		return statement{}, false
	}
	iterator, ok := p.parseExpression(0)
	if !ok || iterator.Kind != expressionRange {
		p.issue("RUST_UNSUPPORTED_LOOP", "bounded for-loop iterator must be an explicit finite range", iterator.Span)
		return statement{}, false
	}
	if !p.check("{") {
		p.issue("RUST_INVALID_LOOP", "expected loop body", p.current().Span)
		return statement{}, false
	}
	body, ok := p.parseBlock()
	if !ok {
		return statement{}, false
	}
	return statement{Kind: statementFor, Name: name.Text, Expr: iterator, Body: &body, Span: mergeSpan(start, body.Span)}, true
}

func (p *parser) parseExpression(minPrecedence int) (expression, bool) {
	left, ok := p.parsePrefix()
	if !ok {
		return expression{}, false
	}
	for {
		op := p.current()
		precedence, supported := binaryPrecedence[op.Text]
		if !supported || precedence < minPrecedence {
			break
		}
		p.advance()
		right, ok := p.parseExpression(precedence + 1)
		if !ok {
			return expression{}, false
		}
		kind := expressionBinary
		if op.Text == ".." || op.Text == "..=" {
			kind = expressionRange
		}
		left = expression{Kind: kind, Text: op.Text, Children: []expression{left, right}, Span: mergeSpan(left.Span, right.Span)}
	}
	return left, true
}

var binaryPrecedence = map[string]int{
	"..": 0, "..=": 0,
	"||": 1,
	"&&": 2,
	"==": 3, "!=": 3, "<": 3, "<=": 3, ">": 3, ">=": 3,
	"+": 4, "-": 4,
	"*": 5, "/": 5, "%": 5,
}

func (p *parser) parsePrefix() (expression, bool) {
	if p.match("!") || p.match("-") {
		op := p.previous()
		child, ok := p.parseExpression(6)
		if !ok {
			return expression{}, false
		}
		return expression{Kind: expressionUnary, Text: op.Text, Children: []expression{child}, Span: mergeSpan(op.Span, child.Span)}, true
	}
	if p.match("if") {
		return p.parseIf(p.previous().Span)
	}
	if p.match("match") {
		return p.parseMatch(p.previous().Span)
	}
	if p.check("{") {
		body, ok := p.parseBlock()
		return expression{Kind: expressionBlock, Then: &body, Span: body.Span}, ok
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (expression, bool) {
	base, ok := p.parsePrimary()
	if !ok {
		return expression{}, false
	}
	for {
		switch {
		case p.match("("):
			args, close, ok := p.parseArguments()
			if !ok {
				return expression{}, false
			}
			base = expression{Kind: expressionCall, Text: base.Text, Children: args, Span: mergeSpan(base.Span, close.Span)}
		case p.match("!"):
			if base.Kind != expressionIdentifier {
				p.issue("RUST_UNRESOLVED_MACRO", "macro path could not be resolved", base.Span)
				return expression{}, false
			}
			if !p.expect("(", "RUST_INVALID_MACRO", "supported macros require parenthesized arguments") {
				return expression{}, false
			}
			args, close, ok := p.parseArguments()
			if !ok {
				return expression{}, false
			}
			base = expression{Kind: expressionMacro, Text: base.Text, Children: args, Span: mergeSpan(base.Span, close.Span)}
		default:
			return base, true
		}
	}
}

func (p *parser) parsePrimary() (expression, bool) {
	tok := p.current()
	switch tok.Kind {
	case tokenIdent:
		p.advance()
		text := tok.Text
		span := tok.Span
		for p.match("::") {
			next := p.current()
			if next.Kind != tokenIdent {
				p.issue("RUST_INVALID_PATH", "expected identifier after '::'", next.Span)
				return expression{}, false
			}
			p.advance()
			text += "::" + next.Text
			span = mergeSpan(span, next.Span)
		}
		if text == "unsafe" {
			p.issue("RUST_UNSAFE", "unsafe Rust is outside the bounded frontend", span)
			return expression{}, false
		}
		return expression{Kind: expressionIdentifier, Text: text, Span: span}, true
	case tokenNumber, tokenString, tokenChar:
		p.advance()
		return expression{Kind: expressionLiteral, Text: tok.Text, Span: tok.Span}, true
	case tokenPunct:
		if tok.Text != "(" {
			p.issue("RUST_UNSUPPORTED_EXPRESSION", fmt.Sprintf("unsupported expression token %s", tokenDescription(tok)), tok.Span)
			p.advance()
			return expression{}, false
		}
		p.advance()
		if p.match(")") {
			return expression{Kind: expressionTuple, Text: "()", Span: mergeSpan(tok.Span, p.previous().Span)}, true
		}
		first, ok := p.parseExpression(0)
		if !ok {
			return expression{}, false
		}
		if p.match(",") {
			values := []expression{first}
			for !p.check(")") && !p.atEOF() {
				value, ok := p.parseExpression(0)
				if !ok {
					return expression{}, false
				}
				values = append(values, value)
				if !p.match(",") {
					break
				}
			}
			if !p.expect(")", "RUST_INVALID_TUPLE", "unterminated tuple") {
				return expression{}, false
			}
			return expression{Kind: expressionTuple, Children: values, Span: mergeSpan(tok.Span, p.previous().Span)}, true
		}
		if !p.expect(")", "RUST_INVALID_EXPRESSION", "unterminated parenthesized expression") {
			return expression{}, false
		}
		first.Span = mergeSpan(tok.Span, p.previous().Span)
		return first, true
	default:
		p.issue("RUST_UNSUPPORTED_EXPRESSION", fmt.Sprintf("unsupported expression token %s", tokenDescription(tok)), tok.Span)
		p.advance()
		return expression{}, false
	}
}

func (p *parser) parseArguments() ([]expression, token, bool) {
	if p.match(")") {
		return nil, p.previous(), true
	}
	var args []expression
	for !p.atEOF() {
		arg, ok := p.parseExpression(0)
		if !ok {
			return nil, token{}, false
		}
		args = append(args, arg)
		if p.match(")") {
			return args, p.previous(), true
		}
		if !p.expect(",", "RUST_INVALID_CALL", "expected ',' between arguments") {
			return nil, token{}, false
		}
		if p.match(")") {
			return args, p.previous(), true
		}
	}
	p.issue("RUST_INVALID_CALL", "unterminated argument list", p.current().Span)
	return nil, token{}, false
}

func (p *parser) parseIf(start sourceSpan) (expression, bool) {
	condition, ok := p.parseExpression(0)
	if !ok {
		return expression{}, false
	}
	if !p.check("{") {
		p.issue("RUST_INVALID_IF", "expected block after if condition", p.current().Span)
		return expression{}, false
	}
	thenBlock, ok := p.parseBlock()
	if !ok {
		return expression{}, false
	}
	result := expression{Kind: expressionIf, Children: []expression{condition}, Then: &thenBlock, Span: mergeSpan(start, thenBlock.Span)}
	if p.match("else") {
		if p.match("if") {
			nested, ok := p.parseIf(p.previous().Span)
			if !ok {
				return expression{}, false
			}
			elseBlock := block{Tail: &nested, Span: nested.Span}
			result.Else = &elseBlock
			result.Span = mergeSpan(start, nested.Span)
		} else if p.check("{") {
			elseBlock, ok := p.parseBlock()
			if !ok {
				return expression{}, false
			}
			result.Else = &elseBlock
			result.Span = mergeSpan(start, elseBlock.Span)
		} else {
			p.issue("RUST_INVALID_IF", "expected 'if' or block after else", p.current().Span)
			return expression{}, false
		}
	}
	return result, true
}

func (p *parser) parseMatch(start sourceSpan) (expression, bool) {
	subject, ok := p.parseExpression(0)
	if !ok {
		return expression{}, false
	}
	if !p.expect("{", "RUST_INVALID_MATCH", "expected '{' after match subject") {
		return expression{}, false
	}
	result := expression{Kind: expressionMatch, Children: []expression{subject}, Span: start}
	for !p.atEOF() && !p.check("}") {
		armStart := p.current().Span
		patternTokens := p.collectUntilTopLevel("if", "=>")
		if len(patternTokens) == 0 {
			p.issue("RUST_INVALID_MATCH", "missing match-arm pattern", p.current().Span)
			return expression{}, false
		}
		pattern := joinTokens(patternTokens)
		if !supportedPattern(patternTokens) {
			p.issue("RUST_UNSUPPORTED_PATTERN", fmt.Sprintf("match pattern %q is not in the bounded pattern subset", pattern), mergeSpan(patternTokens[0].Span, patternTokens[len(patternTokens)-1].Span))
		}
		var guard *expression
		if p.match("if") {
			value, ok := p.parseExpression(0)
			if !ok {
				return expression{}, false
			}
			guard = &value
		}
		if !p.expect("=>", "RUST_INVALID_MATCH", "expected '=>' after match pattern") {
			return expression{}, false
		}
		value, ok := p.parseExpression(0)
		if !ok {
			return expression{}, false
		}
		result.Arms = append(result.Arms, matchArm{Pattern: pattern, Guard: guard, Value: value, Span: mergeSpan(armStart, value.Span)})
		if !p.match(",") && !p.check("}") {
			p.issue("RUST_INVALID_MATCH", "expected ',' between match arms", p.current().Span)
			return expression{}, false
		}
	}
	if !p.expect("}", "RUST_INVALID_MATCH", "unterminated match expression") {
		return expression{}, false
	}
	result.Span = mergeSpan(start, p.previous().Span)
	if len(result.Arms) == 0 {
		p.issue("RUST_INVALID_MATCH", "match expression must have at least one arm", result.Span)
		return result, false
	}
	return result, true
}

func supportedPattern(tokens []token) bool {
	if len(tokens) == 1 {
		return tokens[0].Kind == tokenIdent || tokens[0].Kind == tokenNumber || tokens[0].Kind == tokenString || tokens[0].Kind == tokenChar
	}
	// Exact variant patterns such as Ok(value), Err(_), Some(value).
	if len(tokens) == 4 && tokens[0].Kind == tokenIdent && tokens[1].Text == "(" && (tokens[2].Kind == tokenIdent || tokens[2].Kind == tokenNumber || tokens[2].Kind == tokenString || tokens[2].Kind == tokenChar) && tokens[3].Text == ")" {
		return true
	}
	return false
}

func (p *parser) collectUntilTopLevel(stops ...string) []token {
	stopSet := make(map[string]bool, len(stops))
	for _, stop := range stops {
		stopSet[stop] = true
	}
	var result []token
	var stack []string
	for !p.atEOF() {
		tok := p.current()
		if len(stack) == 0 && stopSet[tok.Text] {
			break
		}
		switch tok.Text {
		case "(", "[", "<":
			stack = append(stack, tok.Text)
		case ")", "]", ">":
			if len(stack) == 0 {
				return result
			}
			stack = stack[:len(stack)-1]
		}
		result = append(result, tok)
		p.advance()
	}
	return result
}

func (p *parser) skipBalancedUntil(stops ...string) {
	p.collectUntilTopLevel(stops...)
	for _, stop := range stops {
		if p.match(stop) {
			return
		}
	}
}

func (p *parser) skipBalancedGroup(open, close string) {
	depth := 1
	for !p.atEOF() && depth > 0 {
		tok := p.advance()
		if tok.Text == open {
			depth++
		} else if tok.Text == close {
			depth--
		}
	}
}

func (p *parser) recoverStatement() {
	depth := 0
	for !p.atEOF() {
		switch p.current().Text {
		case "{", "(", "[":
			depth++
		case "}":
			if depth == 0 {
				return
			}
			depth--
		case ";":
			p.advance()
			if depth == 0 {
				return
			}
		}
		p.advance()
	}
}

func (p *parser) recoverTopLevel() {
	depth := 0
	for !p.atEOF() {
		tok := p.advance()
		switch tok.Text {
		case "{":
			depth++
		case "}":
			if depth > 0 {
				depth--
			}
			if depth == 0 {
				return
			}
		case ";":
			if depth == 0 {
				return
			}
		}
	}
}

func (p *parser) issue(code, message string, span sourceSpan) {
	p.issues = append(p.issues, parseIssue{Code: code, Message: message, Span: span})
}

func (p *parser) expect(text, code, message string) bool {
	if p.match(text) {
		return true
	}
	p.issue(code, message+"; found "+tokenDescription(p.current()), p.current().Span)
	return false
}

func (p *parser) match(text string) bool {
	if !p.check(text) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) check(text string) bool {
	return p.current().Text == text
}

func (p *parser) current() token {
	if p.index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.index]
}

func (p *parser) previous() token {
	if p.index == 0 {
		return p.tokens[0]
	}
	return p.tokens[p.index-1]
}

func (p *parser) advance() token {
	tok := p.current()
	if !p.atEOF() {
		p.index++
	}
	return tok
}

func (p *parser) atEOF() bool {
	return p.current().Kind == tokenEOF
}

func hasTestAttribute(attributes []string) bool {
	for _, attr := range attributes {
		if attr == "test" {
			return true
		}
	}
	return false
}

func joinTokens(tokens []token) string {
	var b strings.Builder
	for i, tok := range tokens {
		if i > 0 && needsTokenSpace(tokens[i-1], tok) {
			b.WriteByte(' ')
		}
		b.WriteString(tok.Text)
	}
	return b.String()
}

func needsTokenSpace(left, right token) bool {
	leftWord := left.Kind == tokenIdent || left.Kind == tokenNumber || left.Kind == tokenString || left.Kind == tokenChar
	rightWord := right.Kind == tokenIdent || right.Kind == tokenNumber || right.Kind == tokenString || right.Kind == tokenChar
	return leftWord && rightWord
}
