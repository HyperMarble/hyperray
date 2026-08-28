package semanticir

import (
	"fmt"
	"strings"
)

// OutcomeID returns the sole canonical identifier for observable outcome
// semantics. All compilers/frontends must use it; language-local ID schemes
// would make proof sets incomparable.
func OutcomeID(outcome ObservableOutcome) string {
	type effectSemantics struct {
		Kind   EffectKind `json:"kind"`
		Target string     `json:"target"`
		Value  any        `json:"value,omitempty"`
	}
	effects := make([]effectSemantics, 0, len(outcome.Effects))
	for _, effect := range outcome.Effects {
		effects = append(effects, effectSemantics{Kind: effect.Kind, Target: effect.Target, Value: expressionSemanticsOf(effect.Value)})
	}
	semantic := struct {
		Kind          OutcomeKind       `json:"kind"`
		Value         *Literal          `json:"value,omitempty"`
		ExceptionType string            `json:"exception_type"`
		Message       string            `json:"message"`
		OperationID   string            `json:"operation_id"`
		Effects       []effectSemantics `json:"effects"`
	}{outcome.Kind, outcome.Value, outcome.ExceptionType, outcome.Message, outcome.OperationID, effects}
	digest, err := Digest(semantic)
	if err != nil {
		panic(err) // the closed semantic struct contains no fallible JSON value
	}
	return "outcome-" + strings.TrimPrefix(digest, "sha256:")[:16]
}

// OtherOutcome returns the canonical complement of all named exact terminal
// and ordered-effect traces for one operation.
func OtherOutcome(operationID string, provenance Provenance) ObservableOutcome {
	outcome := ObservableOutcome{Kind: OutcomeOther, OperationID: operationID, Provenance: provenance}
	outcome.ID = OutcomeID(outcome)
	return outcome
}

// ClassifyOutcome maps a concrete raw terminal/effect trace to its exact
// named operation outcome, or to the operation's canonical complement.
func ClassifyOutcome(operation Operation, raw ObservableOutcome) string {
	exact := OutcomeID(raw)
	for _, id := range operation.OutcomeIDs {
		if id == exact {
			return exact
		}
	}
	return OtherOutcome(operation.ID, raw.Provenance).ID
}

// ObservableOutcomeFromTrace adds only frozen Ray-owned identity/provenance
// to raw runtime facts. Effect values become literal expressions so the
// existing canonical outcome semantics remain the single comparison format.
func ObservableOutcomeFromTrace(operationID string, trace RawOutcomeTrace, provenance Provenance) (ObservableOutcome, error) {
	if err := ValidateRawOutcomeTrace(trace); err != nil {
		return ObservableOutcome{}, err
	}
	outcome := ObservableOutcome{
		Kind: trace.Kind, Value: trace.Value, ExceptionType: trace.ExceptionType,
		Message: trace.Message, OperationID: operationID, Provenance: provenance,
	}
	for index, rawEffect := range trace.Effects {
		var value *Expression
		if rawEffect.Value != nil {
			literal := *rawEffect.Value
			value = &Expression{Kind: ExprLiteral, Type: literal.Type, Literal: &literal, Provenance: provenance}
		}
		outcome.Effects = append(outcome.Effects, Effect{
			ID: fmt.Sprintf("runtime-effect-%d", index+1), Kind: rawEffect.Kind,
			Target: rawEffect.Target, Value: value, Provenance: provenance,
		})
	}
	outcome.ID = OutcomeID(outcome)
	return outcome, nil
}

// ValidateRawOutcomeTrace rejects any terminal/effect shape outside the
// closed raw protocol. In particular OutcomeOther is semantic classification,
// never a runtime-reported terminal.
func ValidateRawOutcomeTrace(trace RawOutcomeTrace) error {
	if trace.Value != nil {
		if err := ValidateLiteral(*trace.Value); err != nil {
			return fmt.Errorf("raw return value: %w", err)
		}
	}
	switch trace.Kind {
	case OutcomeReturn:
		if trace.Value == nil || trace.ExceptionType != "" || trace.Message != "" {
			return fmt.Errorf("raw return requires value and no raise fields")
		}
	case OutcomeRaise:
		if trace.Value != nil || trace.ExceptionType == "" {
			return fmt.Errorf("raw raise requires exception type and no return value")
		}
	case OutcomeSuccess:
		if trace.Value != nil || trace.ExceptionType != "" || trace.Message != "" {
			return fmt.Errorf("raw success has return/raise fields")
		}
	case OutcomeTimeout:
		if trace.Value != nil || trace.ExceptionType != "" || trace.Message != "" || len(trace.Effects) != 0 {
			return fmt.Errorf("raw timeout has return/raise/effect fields")
		}
	default:
		return fmt.Errorf("raw trace has unsupported terminal kind %q", trace.Kind)
	}
	for index, effect := range trace.Effects {
		if effect.Target == "" {
			return fmt.Errorf("raw effect %d has empty target", index)
		}
		switch effect.Kind {
		case EffectRead, EffectWrite, EffectCall, EffectOutput:
		default:
			return fmt.Errorf("raw effect %d has unsupported kind %q", index, effect.Kind)
		}
		if effect.Value != nil {
			if err := ValidateLiteral(*effect.Value); err != nil {
				return fmt.Errorf("raw effect %d value: %w", index, err)
			}
		}
	}
	return nil
}

// ValidateExhaustiveRawOutcomeTrace restricts observation-only exhaustive
// evidence to pure terminals. A harness-reported effect is not trusted code
// semantics; effectful behavior requires the compiler semantic graph.
func ValidateExhaustiveRawOutcomeTrace(trace RawOutcomeTrace) error {
	if err := ValidateRawOutcomeTrace(trace); err != nil {
		return err
	}
	if trace.Kind == OutcomeTimeout {
		return fmt.Errorf("solution harness cannot self-report timeout; the trusted supervisor must produce it")
	}
	if len(trace.Effects) != 0 {
		return fmt.Errorf("exhaustive execution cannot establish effects; compiler semantic graph evidence is required")
	}
	return nil
}

// ClassifyRawOutcome centrally maps raw facts to a named exact outcome or the
// canonical operation-scoped complement.
func ClassifyRawOutcome(operation Operation, trace RawOutcomeTrace, provenance Provenance) (string, error) {
	outcome, err := ObservableOutcomeFromTrace(operation.ID, trace, provenance)
	if err != nil {
		return "", err
	}
	return ClassifyOutcome(operation, outcome), nil
}

func expressionSemanticsOf(expression *Expression) any {
	if expression == nil {
		return nil
	}
	operands := make([]any, 0, len(expression.Operands))
	for index := range expression.Operands {
		operands = append(operands, expressionSemanticsOf(&expression.Operands[index]))
	}
	return struct {
		Kind     ExpressionKind `json:"kind"`
		Type     ValueType      `json:"type"`
		Name     string         `json:"name"`
		Operator Operator       `json:"operator"`
		Literal  *Literal       `json:"literal,omitempty"`
		Operands []any          `json:"operands"`
	}{expression.Kind, expression.Type, expression.Name, expression.Operator, expression.Literal, operands}
}
