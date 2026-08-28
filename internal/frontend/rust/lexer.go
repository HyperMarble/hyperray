package rust

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type lexer struct {
	source []byte
	offset int
	line   int
	column int
}

func lex(source []byte) ([]token, []parseIssue) {
	l := lexer{source: source, line: 1, column: 1}
	var tokens []token
	var issues []parseIssue
	for {
		l.skipWhitespaceAndComments(&issues)
		if l.offset >= len(l.source) {
			pos := l.pos()
			tokens = append(tokens, token{Kind: tokenEOF, Span: sourceSpan{Start: pos, End: pos}})
			return tokens, issues
		}

		start := l.pos()
		r, size := utf8.DecodeRune(l.source[l.offset:])
		if r == utf8.RuneError && size == 1 {
			l.advanceBytes(1)
			issues = append(issues, parseIssue{Code: "RUST_INVALID_UTF8", Message: "source is not valid UTF-8", Span: sourceSpan{Start: start, End: l.pos()}})
			continue
		}

		switch {
		case isIdentStart(r):
			tokens = append(tokens, l.scanIdentifier())
		case unicode.IsDigit(r):
			tokens = append(tokens, l.scanNumber())
		case r == '"':
			tok, issue := l.scanQuoted('"', tokenString)
			tokens = append(tokens, tok)
			if issue != nil {
				issues = append(issues, *issue)
			}
		case r == '\'':
			tok, issue := l.scanApostrophe()
			tokens = append(tokens, tok)
			if issue != nil {
				issues = append(issues, *issue)
			}
		default:
			tokens = append(tokens, l.scanPunctuation())
		}
	}
}

func (l *lexer) skipWhitespaceAndComments(issues *[]parseIssue) {
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRune(l.source[l.offset:])
		if unicode.IsSpace(r) {
			l.advanceRune()
			continue
		}
		if l.hasPrefix("//") {
			for l.offset < len(l.source) && l.source[l.offset] != '\n' {
				l.advanceRune()
			}
			continue
		}
		if l.hasPrefix("/*") {
			start := l.pos()
			l.advanceBytes(2)
			depth := 1
			for l.offset < len(l.source) && depth > 0 {
				switch {
				case l.hasPrefix("/*"):
					depth++
					l.advanceBytes(2)
				case l.hasPrefix("*/"):
					depth--
					l.advanceBytes(2)
				default:
					l.advanceRune()
				}
			}
			if depth != 0 {
				*issues = append(*issues, parseIssue{Code: "RUST_UNTERMINATED_COMMENT", Message: "unterminated block comment", Span: sourceSpan{Start: start, End: l.pos()}})
			}
			continue
		}
		return
	}
}

func (l *lexer) scanIdentifier() token {
	start := l.pos()
	begin := l.offset
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRune(l.source[l.offset:])
		if !isIdentContinue(r) {
			break
		}
		l.advanceRune()
	}
	return token{Kind: tokenIdent, Text: string(l.source[begin:l.offset]), Span: sourceSpan{Start: start, End: l.pos()}}
}

func (l *lexer) scanNumber() token {
	start := l.pos()
	begin := l.offset
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRune(l.source[l.offset:])
		if r == '.' && l.hasPrefix("..") {
			break
		}
		if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_' || r == '.') {
			break
		}
		l.advanceRune()
	}
	return token{Kind: tokenNumber, Text: string(l.source[begin:l.offset]), Span: sourceSpan{Start: start, End: l.pos()}}
}

func (l *lexer) scanQuoted(quote rune, kind tokenKind) (token, *parseIssue) {
	start := l.pos()
	begin := l.offset
	l.advanceRune()
	escaped := false
	for l.offset < len(l.source) {
		r, _ := utf8.DecodeRune(l.source[l.offset:])
		l.advanceRune()
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == quote {
			return token{Kind: kind, Text: string(l.source[begin:l.offset]), Span: sourceSpan{Start: start, End: l.pos()}}, nil
		}
		if r == '\n' && quote == '\'' {
			break
		}
	}
	span := sourceSpan{Start: start, End: l.pos()}
	return token{Kind: kind, Text: string(l.source[begin:l.offset]), Span: span}, &parseIssue{Code: "RUST_UNTERMINATED_LITERAL", Message: "unterminated quoted literal", Span: span}
}

func (l *lexer) scanApostrophe() (token, *parseIssue) {
	start := l.pos()
	begin := l.offset
	// Rust lifetimes are lexically distinct from character literals. A lifetime
	// has no closing apostrophe; preserve it as an identifier for signatures.
	if l.offset+1 < len(l.source) {
		r, size := utf8.DecodeRune(l.source[l.offset+1:])
		if isIdentStart(r) {
			i := l.offset + 1 + size
			for i < len(l.source) {
				r2, s2 := utf8.DecodeRune(l.source[i:])
				if !isIdentContinue(r2) {
					break
				}
				i += s2
			}
			if i >= len(l.source) || l.source[i] != '\'' {
				l.advanceBytes(i - l.offset)
				return token{Kind: tokenIdent, Text: string(l.source[begin:l.offset]), Span: sourceSpan{Start: start, End: l.pos()}}, nil
			}
		}
	}
	return l.scanQuoted('\'', tokenChar)
}

func (l *lexer) scanPunctuation() token {
	start := l.pos()
	begin := l.offset
	for _, width := range []int{3, 2} {
		if l.offset+width <= len(l.source) {
			candidate := string(l.source[l.offset : l.offset+width])
			if rustMultiPunct[candidate] {
				l.advanceBytes(width)
				return token{Kind: tokenPunct, Text: candidate, Span: sourceSpan{Start: start, End: l.pos()}}
			}
		}
	}
	l.advanceRune()
	return token{Kind: tokenPunct, Text: string(l.source[begin:l.offset]), Span: sourceSpan{Start: start, End: l.pos()}}
}

var rustMultiPunct = map[string]bool{
	"..=": true, "<<=": true, ">>=": true,
	"->": true, "=>": true, "::": true, "==": true, "!=": true,
	"<=": true, ">=": true, "&&": true, "||": true, "..": true,
	"+=": true, "-=": true, "*=": true, "/=": true, "%=": true,
	"<<": true, ">>": true, "&=": true, "|=": true, "^=": true,
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentContinue(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}

func (l *lexer) hasPrefix(prefix string) bool {
	return len(l.source)-l.offset >= len(prefix) && string(l.source[l.offset:l.offset+len(prefix)]) == prefix
}

func (l *lexer) pos() sourcePos {
	return sourcePos{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *lexer) advanceRune() {
	if l.offset >= len(l.source) {
		return
	}
	r, size := utf8.DecodeRune(l.source[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
}

func (l *lexer) advanceBytes(n int) {
	target := l.offset + n
	if target > len(l.source) {
		target = len(l.source)
	}
	for l.offset < target {
		l.advanceRune()
	}
}

func tokenDescription(tok token) string {
	if tok.Kind == tokenEOF {
		return "end of file"
	}
	return fmt.Sprintf("%q", tok.Text)
}
