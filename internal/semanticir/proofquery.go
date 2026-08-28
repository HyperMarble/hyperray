package semanticir

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// CanonicalProofQuery deterministically constructs the only accepted SMT-LIB2
// query for a typed compiler-path claim.
func CanonicalProofQuery(claim ProofClaim) ([]byte, error) {
	if err := validateProofContext(claim.Context); err != nil {
		return nil, err
	}
	if err := validateClaimPredicate(claim.Scope, claim.Context); err != nil {
		return nil, fmt.Errorf("scope predicate: %w", err)
	}
	if err := validatePredicateExpression(claim.Scope.Formula); err != nil {
		return nil, fmt.Errorf("scope predicate: %w", err)
	}
	declarations, err := canonicalDeclarations(claim)
	if err != nil {
		return nil, err
	}
	memberships := append([]CompilerPredicate(nil), claim.Memberships...)
	sort.Slice(memberships, func(i, j int) bool { return memberships[i].FormulaDigest < memberships[j].FormulaDigest })
	memberExpressions := make([]string, 0, len(memberships))
	for _, predicate := range memberships {
		if err := validateClaimPredicate(predicate, claim.Context); err != nil {
			return nil, fmt.Errorf("membership predicate: %w", err)
		}
		if err := validatePredicateExpression(predicate.Formula); err != nil {
			return nil, fmt.Errorf("membership predicate: %w", err)
		}
		memberExpressions = append(memberExpressions, string(bytes.TrimSpace(predicate.Formula)))
	}
	outcomes := append([]CompilerOutcomePredicate(nil), claim.Outcomes...)
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].OutcomeID < outcomes[j].OutcomeID })
	outcomeExpressions := make([]string, 0, len(outcomes))
	previousOutcomeID := ""
	for index, outcome := range outcomes {
		if outcome.OutcomeID == "" || (index > 0 && outcome.OutcomeID == previousOutcomeID) {
			return nil, fmt.Errorf("realization claim has an empty or duplicate outcome ID")
		}
		if err := validatePredicateExpression(outcome.Predicate.Formula); err != nil {
			return nil, fmt.Errorf("outcome %q predicate: %w", outcome.OutcomeID, err)
		}
		if err := validateClaimPredicate(outcome.Predicate, claim.Context); err != nil {
			return nil, fmt.Errorf("outcome %q predicate: %w", outcome.OutcomeID, err)
		}
		outcomeExpressions = append(outcomeExpressions, string(bytes.TrimSpace(outcome.Predicate.Formula)))
		previousOutcomeID = outcome.OutcomeID
	}
	if claim.LeftPass != nil {
		if err := validateClaimPredicate(*claim.LeftPass, claim.Context); err != nil {
			return nil, fmt.Errorf("left pass predicate: %w", err)
		}
		if err := validatePredicateExpression(claim.LeftPass.Formula); err != nil {
			return nil, fmt.Errorf("left pass predicate: %w", err)
		}
	}
	if claim.RightPass != nil {
		if err := validateClaimPredicate(*claim.RightPass, claim.Context); err != nil {
			return nil, fmt.Errorf("right pass predicate: %w", err)
		}
		if err := validatePredicateExpression(claim.RightPass.Formula); err != nil {
			return nil, fmt.Errorf("right pass predicate: %w", err)
		}
	}
	hasPassPredicates := claim.LeftPass != nil || claim.RightPass != nil
	scope := string(bytes.TrimSpace(claim.Scope.Formula))
	var assertion string
	switch claim.Kind {
	case ClaimReachability, ClaimUnreachability, ClaimExclusion:
		if (claim.Kind != ClaimExclusion && len(memberExpressions) == 0) || len(outcomes) != 0 || hasPassPredicates {
			return nil, fmt.Errorf("%s claim has invalid membership/outcome arity", claim.Kind)
		}
		assertion = smtAnd(append([]string{scope}, memberExpressions...)...)
	case ClaimTotality:
		if len(memberExpressions) == 0 || len(outcomes) != 0 || hasPassPredicates {
			return nil, fmt.Errorf("totality claim requires memberships and no behavior/outcomes")
		}
		assertion = smtAnd(scope, "(not "+smtOr(memberExpressions...)+")")
	case ClaimDisjointness:
		if len(memberExpressions) == 0 || len(outcomes) != 0 || hasPassPredicates {
			return nil, fmt.Errorf("disjointness claim requires memberships and no outcomes")
		}
		var overlaps []string
		for left := 0; left < len(memberExpressions); left++ {
			for right := left + 1; right < len(memberExpressions); right++ {
				overlaps = append(overlaps, smtAnd(memberExpressions[left], memberExpressions[right]))
			}
		}
		if len(overlaps) == 0 {
			assertion = "false"
		} else {
			assertion = smtAnd(scope, smtOr(overlaps...))
		}
	case ClaimRealization:
		if len(outcomeExpressions) == 0 || hasPassPredicates {
			return nil, fmt.Errorf("realization claim requires outcome predicates")
		}
		assertion = smtAnd(append(append([]string{scope}, memberExpressions...), "(not "+smtOr(outcomeExpressions...)+")")...)
	case ClaimTestObservationCompleteness:
		if len(outcomes) != 0 || claim.LeftPass == nil || claim.RightPass == nil {
			return nil, fmt.Errorf("test-observation-completeness claim requires two pass predicates and no outcomes")
		}
		left := string(bytes.TrimSpace(claim.LeftPass.Formula))
		right := string(bytes.TrimSpace(claim.RightPass.Formula))
		assertion = smtAnd(append(append([]string{scope}, memberExpressions...), "(not (= "+left+" "+right+"))")...)
	case ClaimScopeClosure:
		if len(memberExpressions) != 0 || len(outcomes) != 0 || hasPassPredicates {
			return nil, fmt.Errorf("scope-closure claim uses only its typed omission predicate")
		}
		assertion = scope
	default:
		return nil, fmt.Errorf("unsupported proof claim kind %q", claim.Kind)
	}
	subjects := proofClaimSubjects(claim)
	var builder strings.Builder
	builder.WriteString("(set-logic ALL)\n")
	for _, declaration := range declarations {
		builder.Write(declaration)
		builder.WriteByte('\n')
	}
	builder.WriteString("; ray-claim ")
	builder.WriteString(string(claim.Kind))
	builder.WriteByte('\n')
	for _, digest := range subjects {
		builder.WriteString("; ray-subject ")
		builder.WriteString(digest)
		builder.WriteByte('\n')
	}
	builder.WriteString("(assert ")
	builder.WriteString(assertion)
	builder.WriteString(")\n(check-sat)\n")
	return []byte(builder.String()), nil
}

// BuildProofQuery is retained as a descriptive alias.
func BuildProofQuery(claim ProofClaim) ([]byte, error) { return CanonicalProofQuery(claim) }

// NewProofClaim returns a deterministically ordered claim. CanonicalProofQuery
// still performs all semantic validation, so construction never hides errors.
func NewProofClaim(kind ProofClaimKind, context CompilerProofContext, scope CompilerPredicate, memberships []CompilerPredicate, outcomes []CompilerOutcomePredicate) ProofClaim {
	memberships = append([]CompilerPredicate(nil), memberships...)
	sort.Slice(memberships, func(i, j int) bool { return memberships[i].FormulaDigest < memberships[j].FormulaDigest })
	outcomes = append([]CompilerOutcomePredicate(nil), outcomes...)
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].OutcomeID < outcomes[j].OutcomeID })
	return ProofClaim{Kind: kind, Context: context, Scope: scope, Memberships: memberships, Outcomes: outcomes}
}

// NewTestObservationCompletenessClaim proves there cannot be two concrete
// implementations with the same modeled behavior vector but different
// verifier pass values.
func NewTestObservationCompletenessClaim(context CompilerProofContext, scope CompilerPredicate, behaviorEqualities []CompilerPredicate, leftPass, rightPass CompilerPredicate) ProofClaim {
	claim := NewProofClaim(ClaimTestObservationCompleteness, context, scope, behaviorEqualities, nil)
	claim.LeftPass = &leftPass
	claim.RightPass = &rightPass
	return claim
}

// ProofClaimSubjectDigests returns the canonical, sorted set of immutable
// inputs bound by a typed claim and its generated query.
func ProofClaimSubjectDigests(claim ProofClaim) []string {
	seen := map[string]struct{}{}
	add := func(value string) {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	add(claim.Context.SourceDigest)
	add(claim.Context.WorkspaceTreeDigest)
	add(claim.Context.EmittedIRDigest)
	add(claim.Context.HarnessDigest)
	add(claim.Context.Compiler.Digest)
	if digest, err := Digest(claim.Context.Compiler); err == nil {
		add(digest)
	}
	addPredicate := func(predicate CompilerPredicate) {
		add(predicate.IRDigest)
		add(predicate.DeclarationsDigest)
		add(predicate.FormulaDigest)
		nodeIDs := append([]string(nil), predicate.CompilerNodeIDs...)
		sort.Strings(nodeIDs)
		if digest, err := Digest(nodeIDs); err == nil {
			add(digest)
		}
	}
	addPredicate(claim.Scope)
	for _, predicate := range claim.Memberships {
		addPredicate(predicate)
	}
	var outcomeIDs []string
	for _, outcome := range claim.Outcomes {
		outcomeIDs = append(outcomeIDs, outcome.OutcomeID)
		addPredicate(outcome.Predicate)
	}
	if claim.LeftPass != nil {
		addPredicate(*claim.LeftPass)
	}
	if claim.RightPass != nil {
		addPredicate(*claim.RightPass)
	}
	sort.Strings(outcomeIDs)
	if len(outcomeIDs) != 0 {
		digest, _ := Digest(outcomeIDs)
		add(digest)
	}
	result := make([]string, 0, len(seen))
	for digest := range seen {
		result = append(result, digest)
	}
	sort.Strings(result)
	return result
}

func proofClaimSubjects(claim ProofClaim) []string { return ProofClaimSubjectDigests(claim) }

func canonicalDeclarations(claim ProofClaim) ([][]byte, error) {
	predicates := []CompilerPredicate{claim.Scope}
	predicates = append(predicates, claim.Memberships...)
	for _, outcome := range claim.Outcomes {
		predicates = append(predicates, outcome.Predicate)
	}
	if claim.LeftPass != nil {
		predicates = append(predicates, *claim.LeftPass)
	}
	if claim.RightPass != nil {
		predicates = append(predicates, *claim.RightPass)
	}
	byDigest := map[string][]byte{}
	for _, predicate := range predicates {
		if predicate.DeclarationsDigest != DigestBytes(predicate.Declarations) {
			return nil, fmt.Errorf("predicate declaration digest mismatch")
		}
		trimmed := bytes.TrimSpace(predicate.Declarations)
		if len(trimmed) == 0 {
			continue
		}
		if err := validateDeclarationScript(trimmed); err != nil {
			return nil, err
		}
		byDigest[predicate.DeclarationsDigest] = append([]byte(nil), trimmed...)
	}
	digests := make([]string, 0, len(byDigest))
	for digest := range byDigest {
		digests = append(digests, digest)
	}
	sort.Strings(digests)
	if len(digests) > 1 {
		return nil, fmt.Errorf("claim predicates must share one canonical declaration script")
	}
	result := make([][]byte, 0, len(digests))
	for _, digest := range digests {
		result = append(result, byDigest[digest])
	}
	return result, nil
}

func validateProofContext(context CompilerProofContext) error {
	for name, digest := range map[string]string{
		"source": context.SourceDigest, "workspace": context.WorkspaceTreeDigest,
		"emitted IR": context.EmittedIRDigest, "harness": context.HarnessDigest,
	} {
		if !ValidDigest(digest) {
			return fmt.Errorf("proof context has invalid %s digest", name)
		}
	}
	if err := validateToolRef(context.Compiler); err != nil {
		return fmt.Errorf("proof context compiler: %w", err)
	}
	return nil
}

func validateClaimPredicate(predicate CompilerPredicate, context CompilerProofContext) error {
	if predicate.Logic != ProofLogicSMTLIB2 {
		return fmt.Errorf("unsupported logic %q", predicate.Logic)
	}
	if predicate.FormulaDigest != DigestBytes(predicate.Formula) || predicate.DeclarationsDigest != DigestBytes(predicate.Declarations) {
		return fmt.Errorf("formula or declaration digest mismatch")
	}
	if predicate.Tool != context.Compiler || predicate.IRDigest != context.EmittedIRDigest {
		return fmt.Errorf("compiler or emitted IR binding differs from proof context")
	}
	if len(predicate.CompilerNodeIDs) == 0 {
		return fmt.Errorf("compiler node binding is empty")
	}
	return nil
}

func validateDeclarationScript(script []byte) error {
	text := strings.TrimSpace(string(script))
	depth := 0
	start := -1
	inString := false
	for index, character := range text {
		if inString {
			if character == '"' {
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case ';':
			return fmt.Errorf("comments are forbidden in predicate declarations")
		case '(':
			if depth == 0 {
				start = index
			}
			depth++
		case ')':
			depth--
			if depth < 0 {
				return fmt.Errorf("unbalanced predicate declarations")
			}
			if depth == 0 {
				form := strings.TrimSpace(text[start : index+1])
				head := strings.Fields(strings.TrimSpace(form[1:]))
				if len(head) == 0 {
					return fmt.Errorf("empty predicate declaration")
				}
				operator := strings.Trim(head[0], "()")
				switch operator {
				case "declare-fun", "declare-const", "define-fun", "define-const":
				default:
					return fmt.Errorf("forbidden declaration command %q", operator)
				}
			}
		default:
			if depth == 0 && !unicode.IsSpace(character) {
				return fmt.Errorf("text outside predicate declarations")
			}
		}
	}
	if depth != 0 || inString {
		return fmt.Errorf("unbalanced predicate declarations")
	}
	return nil
}

func smtAnd(expressions ...string) string { return "(and " + strings.Join(expressions, " ") + ")" }
func smtOr(expressions ...string) string  { return "(or " + strings.Join(expressions, " ") + ")" }

// validatePredicateExpression accepts exactly one SMT expression and rejects
// top-level commands. Compiler predicates may contain arbitrary solver terms,
// but never assert/check/declare commands or trailing forms.
func validatePredicateExpression(formula []byte) error {
	text := strings.TrimSpace(string(formula))
	if text == "" {
		return fmt.Errorf("empty formula")
	}
	if text[0] != '(' {
		for _, character := range text {
			if unicode.IsSpace(character) || character == '(' || character == ')' || character == ';' {
				return fmt.Errorf("atom contains invalid syntax")
			}
		}
		return nil
	}
	depth := 0
	inString := false
	escaped := false
	end := -1
	for index, character := range text {
		if inString {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case ';':
			return fmt.Errorf("comments are forbidden inside predicate expressions")
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return fmt.Errorf("unbalanced formula")
			}
			if depth == 0 {
				end = index
			}
		}
	}
	if inString || depth != 0 || end != len(text)-1 {
		return fmt.Errorf("formula must contain exactly one balanced expression")
	}
	head := strings.Fields(strings.TrimSpace(text[1:]))
	if len(head) == 0 {
		return fmt.Errorf("formula has no operator")
	}
	operator := strings.Trim(head[0], "()")
	for _, forbidden := range []string{"assert", "check-sat", "set-logic", "set-option", "declare-fun", "declare-const", "define-fun", "push", "pop", "exit"} {
		if operator == forbidden {
			return fmt.Errorf("top-level command %q is forbidden", operator)
		}
	}
	return nil
}
