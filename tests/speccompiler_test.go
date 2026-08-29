package tests

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/HyperMarble/hyperray/internal/speccompiler"
	"github.com/HyperMarble/hyperray/internal/specparser"
)

const strictHeader = "| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |\n" +
	"|---|---|---|---|---|---|---|---|---|---|---|\n"

type testOperationDomains struct {
	domains     map[string][]string
	domainOrder []string
}

func frozenRef(id string, kind semanticir.ArtifactKind, path string, content []byte) semanticir.ArtifactRef {
	return semanticir.ArtifactRef{ID: id, Kind: kind, Path: path, Digest: semanticir.DigestBytes(content)}
}

func TestSpecCompilerQuotedFiniteValuesRoundTrip(t *testing.T) {
	values := `"/api/v1" / "https://example/x/y?q=1/2" / "2026/08/27" / "left / right" / "quote: \" and slash: \\" / "雪/☃"`
	source := "# Quoted finite values\n\nParameters: `resource` (" + values + ").\n\n" +
		"| resource | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|\n" +
		"| " + values + " | req-resource | choose-resource | reachable | return true | return false; other outcome | none | none | none | 1 | — |\n"
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) || task == nil {
		t.Fatalf("Compile task=%v diagnostics=%+v", task, diagnostics)
	}
	want := []string{"/api/v1", "https://example/x/y?q=1/2", "2026/08/27", "left / right", `quote: " and slash: \`, "雪/☃"}
	var got []string
	for _, value := range task.Domains[0].Values {
		if value.Value != nil {
			t.Fatalf("lexical label was reinterpreted as a runtime literal: %+v", value)
		}
		got = append(got, value.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compiled domain=%q, want %q", got, want)
	}
	if len(task.Requirements) != len(want) {
		t.Fatalf("requirements=%d, want %d expanded quoted values", len(task.Requirements), len(want))
	}
	seen := map[string]bool{}
	for _, requirement := range task.Requirements {
		seen[requirement.Conditions["resource"]] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Errorf("compiled conditions lost %q", value)
		}
	}
}

func TestSpecCompilerQuotedAnyIsLiteral(t *testing.T) {
	source := `# Quoted reserved value

Parameters: ` + "`mode`" + ` ("any" / other).

` + strictHeader +
		`| "any" | req-any | choose | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |
| other | req-other | choose | reachable | return 1 | return 0; other outcome | none | none | none | 1 | — |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) || task == nil || len(task.Requirements) != 2 {
		t.Fatalf("quoted any task=%v diagnostics=%+v", task, diagnostics)
	}
	if task.Requirements[0].Conditions["mode"] != "any" || task.Requirements[1].Conditions["mode"] != "other" {
		t.Fatalf("quoted any conditions=%+v", task.Requirements)
	}

	ambiguous := strings.Replace(source, `| "any" | req-any |`, `| any | req-any |`, 1)
	compiled, rejected := compileSpec(t, ambiguous)
	if compiled != nil || !diagnosticHasCode(rejected, semanticir.DiagnosticOverlapping) {
		t.Fatalf("unquoted any did not act as wildcard: task=%v diagnostics=%+v", compiled, rejected)
	}
}

func compileSpec(t *testing.T, source string) (*semanticir.Task, []semanticir.Diagnostic) {
	t.Helper()
	source = withTestGroundings(t, source)
	instruction := []byte("Return zero in zero mode and one in one mode.\n")
	return speccompiler.Compile(context.Background(), speccompiler.Request{
		TaskID:            "task",
		Artifact:          frozenRef("spec", semanticir.ArtifactSpec, "spec.md", []byte(source)),
		Source:            []byte(source),
		Instruction:       frozenRef("instruction", semanticir.ArtifactInstruction, "instruction.md", instruction),
		InstructionSource: instruction,
	})
}

func compileSpecRaw(t *testing.T, source string) (*semanticir.Task, []semanticir.Diagnostic) {
	t.Helper()
	instruction := []byte("Return zero in zero mode and one in one mode.\n")
	return speccompiler.Compile(context.Background(), speccompiler.Request{
		TaskID:            "task",
		Artifact:          frozenRef("spec", semanticir.ArtifactSpec, "spec.md", []byte(source)),
		Source:            []byte(source),
		Instruction:       frozenRef("instruction", semanticir.ArtifactInstruction, "instruction.md", instruction),
		InstructionSource: instruction,
	})
}

func relationalGroundingSpec() string {
	return `# Relational grounding

Inputs: fits(start: integer, blocks: integer, limit: integer).
Universe: fits.start = values [0].
Universe: fits.blocks = values [1,2].
Universe: fits.limit = values [1].
Grounding: fits.case."fits" = when blocks > 0 and start >= 0 and limit >= start and blocks <= limit - start; witness {"blocks":1,"limit":1,"start":0}.
Grounding: fits.case."too-large" = when blocks > 0 and start >= 0 and limit >= start and blocks > limit - start; witness {"blocks":2,"limit":1,"start":0}.
Parameters: ` + "`case`" + ` ("fits" / "too-large").

| case | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| "fits" | fits-yes | fits | reachable | return true | return false; timeout; other outcome | none | none | [{"blocks":1,"limit":1,"start":0}] | none | 1 | — |
| "too-large" | fits-no | fits | reachable | return false | return true; timeout; other outcome | none | none | [{"blocks":2,"limit":1,"start":0}] | none | 1 | — |
`
}

func compoundGroundingSpec() string {
	return `# Compound grounding

Inputs: choose(x: integer).
Universe: choose.x = values [0,1].
Grounding: choose.mode."low" = when x <= 0; witness {"x":0}.
Grounding: choose.mode."high" = when x > 0; witness {"x":1}.
Parameters: ` + "`mode`" + ` ("low" / "high").

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| any | choose-all | choose | reachable | return true | return false; timeout; other outcome | none | none | [{"x":0},{"x":1}] | none | 1 | — |
`
}

func TestSpecCompilerTypedRelationalGroundingAndJointWitnesses(t *testing.T) {
	task, diagnostics := compileSpecRaw(t, relationalGroundingSpec())
	if task == nil || semanticir.HasErrors(diagnostics) {
		t.Fatalf("relational grounding rejected: task=%v diagnostics=%+v", task, diagnostics)
	}
	if len(task.Operations) != 1 || len(task.Operations[0].Inputs) != 3 || len(task.Groundings) != 2 || len(task.Requirements) != 2 {
		t.Fatalf("unexpected relational grounding shape: %+v", task)
	}
	for _, value := range task.Domains[0].Values {
		if value.Value != nil || len(value.Groundings) != 1 || value.Groundings[0].Kind != semanticir.GroundingMembership || value.Groundings[0].Membership == nil {
			t.Fatalf("label lost semantic-ID/membership separation: %+v", value)
		}
	}
	for _, requirement := range task.Requirements {
		if requirement.GroundingID == "" || requirement.GroundingID != semanticir.AssignmentGroundingID(requirement.OperationID, requirement.Conditions) {
			t.Fatalf("requirement has stale assignment grounding: %+v", requirement)
		}
	}
}

func TestSpecCompilerRejectsInvalidGroundingAndInputWitnesses(t *testing.T) {
	valid := compoundGroundingSpec()
	tests := []struct {
		name   string
		source string
		code   semanticir.DiagnosticCode
	}{
		{name: "missing-inputs", source: strings.Replace(valid, "Inputs: choose(x: integer).\n", "", 1), code: semanticir.DiagnosticIncomplete},
		{name: "missing-label-grounding", source: strings.Replace(valid, "Grounding: choose.mode.\"high\" = when x > 0; witness {\"x\":1}.\n", "", 1), code: semanticir.DiagnosticIncomplete},
		{name: "call-expression", source: strings.Replace(valid, "x <= 0", "is_low(x)", 1), code: semanticir.DiagnosticInvalidInput},
		{name: "division-expression", source: strings.Replace(valid, "x <= 0", "x / 2 <= 0", 1), code: semanticir.DiagnosticInvalidInput},
		{name: "overflow-expression", source: strings.Replace(valid, `x <= 0; witness {"x":0}`, `x + 1 > 0; witness {"x":9223372036854775807}`, 1), code: semanticir.DiagnosticUnreachable},
		{name: "partial-label-witness", source: strings.Replace(valid, `witness {"x":0}`, `witness {}`, 1), code: semanticir.DiagnosticInvalidInput},
		{name: "noncanonical-label-witness", source: strings.Replace(valid, `witness {"x":0}`, `witness { "x":0}`, 1), code: semanticir.DiagnosticInvalidInput},
		{name: "unsatisfied-label-witness", source: strings.Replace(valid, `x <= 0; witness {"x":0}`, `x < 0; witness {"x":0}`, 1), code: semanticir.DiagnosticUnreachable},
		{name: "missing-row-witness-column", source: strings.ReplaceAll(strings.Replace(valid, " | Input witnesses |", "", 1), "| none | none | [{\"x\":0},{\"x\":1}] | none | 1 |", "| none | none | none | 1 |"), code: semanticir.DiagnosticInvalidInput},
		{name: "extra-row-witness", source: strings.Replace(valid, `[{"x":0},{"x":1}]`, `[{"x":0},{"x":1},{"x":2}]`, 1), code: semanticir.DiagnosticIncomplete},
		{name: "reordered-row-witness", source: strings.Replace(valid, `[{"x":0},{"x":1}]`, `[{"x":1},{"x":0}]`, 1), code: semanticir.DiagnosticUnreachable},
		{name: "partial-row-witness", source: strings.Replace(valid, `[{"x":0},{"x":1}]`, `[{}, {"x":1}]`, 1), code: semanticir.DiagnosticInvalidInput},
		{name: "noncanonical-row-witness", source: strings.Replace(valid, `[{"x":0},{"x":1}]`, `[{ "x":0},{"x":1}]`, 1), code: semanticir.DiagnosticInvalidInput},
		{name: "nonsatisfying-row-witness", source: strings.Replace(valid, `[{"x":0},{"x":1}]`, `[{"x":0},{"x":0}]`, 1), code: semanticir.DiagnosticUnreachable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task, diagnostics := compileSpecRaw(t, test.source)
			if task != nil || !diagnosticHasCode(diagnostics, test.code) {
				t.Fatalf("invalid grounding accepted: task=%v diagnostics=%+v", task, diagnostics)
			}
		})
	}
}

// withTestGroundings keeps the table-focused fixtures concise while ensuring
// they exercise the production compiler's mandatory authored grounding seam.
// It emits ordinary equality predicates; relational grounding has dedicated
// tests below.
func withTestGroundings(t *testing.T, source string) string {
	t.Helper()
	if strings.Contains(source, "\nInputs:") || strings.HasPrefix(source, "Inputs:") {
		return source
	}
	tables, err := specparser.Parse(source)
	if err != nil {
		return source
	}
	operations := map[string]*testOperationDomains{}
	for _, table := range tables {
		var parsed []specparser.Domain
		if !strings.Contains(table.Params, "Parameters: none") {
			parsed, _, err = specparser.ParseParams(table.Params)
			if err != nil {
				continue
			}
		}
		opIndex := -1
		for index, column := range table.Columns {
			if strings.EqualFold(strings.TrimSpace(column), "Operation") {
				opIndex = index
				break
			}
		}
		for _, row := range table.Rows {
			if opIndex < 0 || opIndex >= len(row) {
				continue
			}
			operationID := strings.Trim(strings.TrimSpace(row[opIndex]), "`")
			if operationID == "" || operationID == "—" || operationID == "-" || operationID == "none" {
				continue
			}
			item := operations[operationID]
			if item == nil {
				item = &testOperationDomains{domains: map[string][]string{}}
				operations[operationID] = item
			}
			for _, domain := range parsed {
				if _, exists := item.domains[domain.Name]; !exists {
					item.domainOrder = append(item.domainOrder, domain.Name)
				}
				item.domains[domain.Name] = append([]string(nil), domain.Values...)
			}
		}
	}
	if len(operations) == 0 {
		return source
	}
	var operationIDs []string
	for operationID := range operations {
		operationIDs = append(operationIDs, operationID)
	}
	sort.Strings(operationIDs)
	var declarations strings.Builder
	declarations.WriteString("\n")
	for _, operationID := range operationIDs {
		item := operations[operationID]
		domainIDs := item.domainOrder
		declarations.WriteString("Inputs: " + operationID + "(")
		for index, domainID := range domainIDs {
			if index > 0 {
				declarations.WriteString(", ")
			}
			declarations.WriteString(domainID + ": string")
		}
		declarations.WriteString(").\n")
		for _, domainID := range domainIDs {
			for _, valueID := range item.domains[domainID] {
				encoded, _ := json.Marshal(valueID)
				witness, _ := json.Marshal(map[string]any{})
				assignment := map[string]any{}
				for _, inputID := range domainIDs {
					values := item.domains[inputID]
					assignment[inputID] = values[0]
				}
				assignment[domainID] = valueID
				witness, _ = json.Marshal(assignment)
				declarations.WriteString("Grounding: " + operationID + "." + domainID + "." + string(encoded) + " = when " + domainID + " == " + string(encoded) + "; witness " + string(witness) + ".\n")
			}
		}
	}
	source = injectTestWitnesses(t, source, tables, operations)
	return source + declarations.String()
}

func injectTestWitnesses(t *testing.T, source string, tables []specparser.Table, operations map[string]*testOperationDomains) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	for _, table := range tables {
		enforced := -1
		reachability := -1
		operationColumn := -1
		forbiddenOutcomes := -1
		for index, column := range table.Columns {
			switch strings.ToLower(strings.TrimSpace(column)) {
			case "enforced by":
				enforced = index
			case "reachability":
				reachability = index
			case "operation":
				operationColumn = index
			case "forbidden outcomes":
				forbiddenOutcomes = index
			}
		}
		if enforced < 0 || reachability < 0 || operationColumn < 0 {
			continue
		}
		header := append([]string(nil), table.Columns...)
		header = insertTestCell(header, enforced, "Input witnesses")
		lines[table.Line-1] = renderTestRow(header)
		separator := make([]string, len(header))
		for index := range separator {
			separator[index] = "---"
		}
		lines[table.Line] = renderTestRow(separator)

		parsedDomains, _, _ := specparser.ParseParams(table.Params)
		byDomain := map[string]specparser.Domain{}
		columnByDomain := map[string]int{}
		for _, domain := range parsedDomains {
			byDomain[domain.Name] = domain
			for index, column := range table.Columns {
				if strings.EqualFold(strings.TrimSpace(column), domain.Name) {
					columnByDomain[domain.Name] = index
				}
			}
		}
		for rowIndex, original := range table.Rows {
			original = append([]string(nil), original...)
			witnessCell := "none"
			if strings.EqualFold(strings.Trim(strings.TrimSpace(original[reachability]), "`"), "reachable") {
				if forbiddenOutcomes >= 0 && forbiddenOutcomes < len(original) && !strings.Contains(original[forbiddenOutcomes], "timeout") {
					original[forbiddenOutcomes] = strings.TrimSpace(original[forbiddenOutcomes]) + "; timeout"
				}
				operationID := strings.Trim(strings.TrimSpace(original[operationColumn]), "`")
				item := operations[operationID]
				assignments := []map[string]string{{}}
				if item != nil {
					for _, domainID := range item.domainOrder {
						values := item.domains[domainID]
						if domain, exists := byDomain[domainID]; exists {
							values = specparser.CellValues(original[columnByDomain[domainID]], domain)
						}
						var next []map[string]string
						for _, assignment := range assignments {
							for _, value := range values {
								copy := map[string]string{}
								for name, existing := range assignment {
									copy[name] = existing
								}
								copy[domainID] = value
								next = append(next, copy)
							}
						}
						assignments = next
					}
				}
				witnesses := make([]map[string]any, 0, len(assignments))
				for _, assignment := range assignments {
					witness := map[string]any{}
					for name, value := range assignment {
						witness[name] = value
					}
					witnesses = append(witnesses, witness)
				}
				encoded, _ := json.Marshal(witnesses)
				witnessCell = string(encoded)
			}
			row := insertTestCell(append([]string(nil), original...), enforced, witnessCell)
			lines[table.Line+1+rowIndex] = renderTestRow(row)
		}
	}
	return strings.Join(lines, "\n")
}

func insertTestCell(values []string, index int, value string) []string {
	values = append(values, "")
	copy(values[index+1:], values[index:])
	values[index] = value
	return values
}

func renderTestRow(values []string) string { return "| " + strings.Join(values, " | ") + " |" }

func validStrictSpec() string {
	return "# Strict spec\n\nParameters: `mode` (zero / one).\n\n" + strictHeader +
		"| zero | req-zero | choose | reachable | return 0 | return 1; other outcome | write:result | none | test_choose#assert-1 | 1 | — |\n" +
		"| one | req-one | choose | reachable | return 1 | return 0; other outcome | none | none | none | 1 | — |\n"
}

func diagnosticHasCode(diagnostics []semanticir.Diagnostic, code semanticir.DiagnosticCode) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestSpecCompilerCompilesStrictFiniteTable(t *testing.T) {
	task, diagnostics := compileSpec(t, validStrictSpec())
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Compile diagnostics: %+v", diagnostics)
	}
	if task == nil {
		t.Fatal("Compile returned nil task")
	}
	if len(task.Domains) != 1 || len(task.Requirements) != 2 || len(task.Outcomes) != 4 || len(task.Operations) != 1 {
		t.Fatalf("unexpected compiled shape: domains=%d requirements=%d outcomes=%d operations=%d", len(task.Domains), len(task.Requirements), len(task.Outcomes), len(task.Operations))
	}
	if got := task.Operations[0].DomainIDs; len(got) != 1 || got[0] != "mode" {
		t.Fatalf("operation domain scope = %v", got)
	}
	if len(task.Operations[0].OutcomeIDs) != 4 {
		t.Fatalf("operation outcomes = %v", task.Operations[0].OutcomeIDs)
	}
	for _, value := range task.Domains[0].Values {
		if value.Value != nil || (value.ID != "zero" && value.ID != "one") {
			t.Fatalf("bare semantic label was reinterpreted as runtime data: %+v", value)
		}
	}
	if len(task.Requirements[0].InstructionClauseIDs) != 1 || len(task.InstructionModel.Clauses) != 1 {
		t.Fatalf("instruction linkage missing: requirement=%v model=%+v", task.Requirements[0].InstructionClauseIDs, task.InstructionModel)
	}
	if task.Requirements[1].TestIDs != nil {
		t.Fatalf("Enforced by none compiled to %v, want nil", task.Requirements[1].TestIDs)
	}
	if issues := task.ValidateSpec(); semanticir.HasErrors(issues) {
		t.Fatalf("compiled task does not validate: %+v", issues)
	}
}

func TestSpecCompilerRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name   string
		source string
		code   semanticir.DiagnosticCode
	}{
		{name: "empty", source: " \n", code: semanticir.DiagnosticInvalidInput},
		{name: "missing-domain", source: "# Spec\n\n" + strictHeader + "| zero | req | choose | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |\n", code: semanticir.DiagnosticMissingDomain},
		{name: "duplicate-id", source: "# Spec\n\nParameters: `mode` (zero / one).\n\n" + strictHeader +
			"| zero | duplicate | choose | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |\n" +
			"| one | duplicate | choose | reachable | return 1 | return 0; other outcome | none | none | none | 1 | — |\n", code: semanticir.DiagnosticDuplicateID},
		{name: "unreachable", source: "# Spec\n\nParameters: `mode` (zero / one).\n\n" + strictHeader +
			"| impossible | req-bad | choose | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |\n" +
			"| any | req-all | choose | reachable | return 1 | return 0; other outcome | none | none | none | 1 | — |\n", code: semanticir.DiagnosticUnreachable},
		{name: "incomplete", source: "# Spec\n\nParameters: `mode` (zero / one).\n\n" + strictHeader +
			"| zero | req-zero | choose | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |\n", code: semanticir.DiagnosticIncomplete},
		{name: "overlapping", source: "# Spec\n\nParameters: `mode` (zero / one).\n\n" + strictHeader +
			"| any | req-all | choose | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |\n" +
			"| zero | req-zero | choose | reachable | return 1 | return 0; other outcome | none | none | none | 1 | — |\n", code: semanticir.DiagnosticOverlapping},
		{name: "prose-only", source: "# Spec\n\nThe implementation must return zero.\n\n" + strings.TrimPrefix(validStrictSpec(), "# Strict spec\n\n"), code: semanticir.DiagnosticProseRequirement},
		{name: "opaque-invariant", source: strings.Replace(validStrictSpec(), "| none | test_choose#assert-1 |", "| result_is_finite | test_choose#assert-1 |", 1), code: semanticir.DiagnosticProseRequirement},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task, diagnostics := compileSpec(t, test.source)
			if task != nil {
				t.Fatalf("Compile returned partial task: %+v", task)
			}
			if !diagnosticHasCode(diagnostics, test.code) {
				t.Fatalf("diagnostics = %+v, want code %q", diagnostics, test.code)
			}
		})
	}
}

func TestSpecCompilerOperationScopedDomains(t *testing.T) {
	source := `# Multiple operations

Parameters: ` + "`x`" + ` (zero / one).

| x | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|
| zero | f-zero | f | reachable | return 0 | return 1; other outcome | none | none | none | 1 | — |
| one | f-one | f | reachable | return 1 | return 0; other outcome | none | none | none | 1 | — |

Parameters: ` + "`mode`" + ` (p / q / r).

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|
| p / q | g-ok | g | reachable | success | raise ValueError containing "bad"; other outcome | none | none | none | 1 | — |
| r | g-bad | g | reachable | raise ValueError containing "bad" | success; other outcome | none | none | none | 1 | — |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Compile diagnostics: %+v", diagnostics)
	}
	if task == nil || len(task.Requirements) != 5 {
		t.Fatalf("requirements=%d, want operation-scoped 2+3", len(task.Requirements))
	}
	if got := len(semanticir.EnumerateAssignments(task.Domains)); got != 6 {
		// The registry product exists for serialization, but it is deliberately
		// not the proof universe; operation scoping above yields five behaviors.
		if got == 0 {
			t.Fatal("domain registry unexpectedly empty")
		}
	}
}

func TestSpecCompilerEffectValuedOutcomePartition(t *testing.T) {
	source := `# Effect-valued outcomes

Parameters: ` + "`mode`" + ` (scheduled / immediate).

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|
| scheduled | scheduled | drive | reachable | return unit | return unit with write:result=2; other outcome | write:result=1 | none | none | 1 | — |
| immediate | immediate | drive | reachable | return unit | return unit with write:result=1; other outcome | write:result=2 | none | none | 1 | — |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) {
		t.Fatalf("Compile diagnostics: %+v", diagnostics)
	}
	if len(task.Outcomes) != 4 {
		t.Fatalf("effect-valued outcomes collapsed: %+v", task.Outcomes)
	}
	for _, outcome := range task.Outcomes {
		if outcome.Kind == semanticir.OutcomeOther || outcome.Kind == semanticir.OutcomeTimeout {
			continue
		}
		if len(outcome.Effects) != 1 || outcome.Effects[0].Value == nil || outcome.ID != semanticir.OutcomeID(outcome) {
			t.Fatalf("outcome lacks exact effect value/identity: %+v", outcome)
		}
	}
	for _, requirement := range task.Requirements {
		if len(requirement.RequiredOutcomes) != 1 || len(requirement.ForbiddenOutcomes) != 3 || requirement.RequiredOutcomes[0] == requirement.ForbiddenOutcomes[0] {
			t.Fatalf("requirement partition is not exact: %+v", requirement)
		}
	}
}

func TestSpecCompilerAllowsSingleOutcomeUniverse(t *testing.T) {
	source := `# One possible behavior

Parameters: ` + "`mode`" + ` ("only").

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|
| only | only-case | f | reachable | return unit | other outcome | none | none | none | 1 | — |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) || task == nil {
		t.Fatalf("single-outcome spec rejected: task=%v diagnostics=%+v", task, diagnostics)
	}
	if len(task.Outcomes) != 3 || len(task.Requirements) != 1 || len(task.Requirements[0].ForbiddenOutcomes) != 2 {
		t.Fatalf("single-outcome partition is wrong: outcomes=%+v requirements=%+v", task.Outcomes, task.Requirements)
	}
}

func TestSpecCompilerAcceptanceDecodersRequireCanonicalClosedJSON(t *testing.T) {
	record := speccompiler.AcceptanceRecordV1{Schema: semanticir.SpecAuthoringRecordSchemaV1, TaskID: "task", Complete: true}
	canonical, err := semanticir.CanonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = speccompiler.DecodeAcceptanceRecord(canonical); err != nil {
		t.Fatalf("canonical acceptance record rejected: %v", err)
	}
	if _, err = speccompiler.DecodeAcceptanceRecord(append(canonical, '\n')); err == nil {
		t.Fatal("noncanonical acceptance record was accepted")
	}
	unknown := append(append([]byte(nil), canonical[:len(canonical)-1]...), []byte(`,"unknown":true}`)...)
	if _, err = speccompiler.DecodeAcceptanceRecord(unknown); err == nil {
		t.Fatal("unknown acceptance field was accepted")
	}
	phase := semanticir.PhaseAEnvironmentModel{Schema: semanticir.PhaseAEnvironmentSchemaV1, Identity: "env", ConfigurationDigest: semanticir.DigestBytes([]byte("config")), Complete: true}
	phaseJSON, _ := semanticir.CanonicalJSON(phase)
	if _, err = speccompiler.DecodePhaseAEnvironment(phaseJSON); err != nil {
		t.Fatalf("canonical Phase-A environment rejected: %v", err)
	}
	ledger := speccompiler.AcceptanceLedgerV1{Schema: speccompiler.AcceptanceLedgerSchemaV1, Entries: []speccompiler.AcceptanceLedgerEntry{}}
	ledgerJSON, _ := semanticir.CanonicalJSON(ledger)
	if _, err = speccompiler.DecodeAcceptanceLedger(ledgerJSON); err != nil {
		t.Fatalf("canonical ledger rejected: %v", err)
	}
}

func TestSpecCompilerExplicitOtherOutcomeAndNull(t *testing.T) {
	source := `# Optional result

Parameters: ` + "`mode`" + ` ("empty").

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|
| empty | empty-case | choose | reachable | return null | other outcome | none | none | none | 1 | — |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) || task == nil {
		t.Fatalf("explicit complement/null spec rejected: task=%v diagnostics=%+v", task, diagnostics)
	}
	if len(task.Outcomes) != 3 || len(task.Operations) != 1 || len(task.Operations[0].OutcomeIDs) != 3 {
		t.Fatalf("closed optional outcome shape is wrong: %+v", task)
	}
	var foundNull, foundOther bool
	for _, outcome := range task.Outcomes {
		switch outcome.Kind {
		case semanticir.OutcomeReturn:
			foundNull = outcome.Value != nil && outcome.Value.Type == semanticir.TypeOptional && outcome.Value.Null
		case semanticir.OutcomeOther:
			foundOther = outcome.OperationID == "choose" && outcome.Value == nil && len(outcome.Effects) == 0
		}
		if outcome.ID != semanticir.OutcomeID(outcome) {
			t.Errorf("outcome ID is not canonical: %+v", outcome)
		}
	}
	if !foundNull || !foundOther {
		t.Fatalf("null/complement semantics missing: %+v", task.Outcomes)
	}
}

func TestSpecCompilerRequiresExplicitOtherOutcomeAndRowClassification(t *testing.T) {
	missingFromAlphabet := `# Open result alphabet

Parameters: ` + "`mode`" + ` (zero / one).

` + strictHeader +
		`| zero | zero-case | choose | reachable | return 0 | return 1 | none | none | none | 1 | — |
| one | one-case | choose | reachable | return 1 | return 0 | none | none | none | 1 | — |
`
	if task, diagnostics := compileSpec(t, missingFromAlphabet); task != nil || !diagnosticHasCode(diagnostics, semanticir.DiagnosticIncomplete) {
		t.Fatalf("missing explicit complement accepted: task=%v diagnostics=%+v", task, diagnostics)
	}

	unclassifiedOnOneRow := strings.Replace(missingFromAlphabet,
		"| zero | zero-case | choose | reachable | return 0 | return 1 |",
		"| zero | zero-case | choose | reachable | return 0 | return 1; other outcome |", 1)
	if task, diagnostics := compileSpec(t, unclassifiedOnOneRow); task != nil || !diagnosticHasCode(diagnostics, semanticir.DiagnosticIncomplete) {
		t.Fatalf("row that omits complement classification accepted: task=%v diagnostics=%+v", task, diagnostics)
	}

	qualifiedComplement := strings.Replace(unclassifiedOnOneRow, "other outcome", "other outcome with write:state=true", 1)
	if task, diagnostics := compileSpec(t, qualifiedComplement); task != nil || !diagnosticHasCode(diagnostics, semanticir.DiagnosticProseRequirement) {
		t.Fatalf("effect-qualified complement accepted: task=%v diagnostics=%+v", task, diagnostics)
	}

	complementWithRowEffects := `# Complement cannot inherit effects

Parameters: ` + "`mode`" + ` ("only").

` + strictHeader + `| only | only-case | choose | reachable | other outcome | return null | write:state=true | none | none | 1 | — |
`
	if task, diagnostics := compileSpec(t, complementWithRowEffects); task != nil || !diagnosticHasCode(diagnostics, semanticir.DiagnosticOverlapping) {
		t.Fatalf("row effects attached to complement: task=%v diagnostics=%+v", task, diagnostics)
	}
}

func TestSpecCompilerExcludedRowAllowsOnlySemanticNoneTestMapping(t *testing.T) {
	source := `# Constraint mapping

Parameters: ` + "`mode`" + ` (enabled / disabled).

| mode | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|
| enabled | enabled-case | f | reachable | return unit | other outcome | none | none | none | 1 | — |
| disabled | disabled-case | f | excluded | none | none | none | none | none | none | disabled state is outside this operation scope |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) || task == nil || len(task.Constraints) != 1 {
		t.Fatalf("excluded row with Enforced by none rejected: task=%v diagnostics=%+v", task, diagnostics)
	}

	withTestID := strings.Replace(source, "| none | none | disabled state", "| test_disabled | none | disabled state", 1)
	badTask, badDiagnostics := compileSpec(t, withTestID)
	if badTask != nil || !diagnosticHasCode(badDiagnostics, semanticir.DiagnosticInvalidInput) {
		t.Fatalf("excluded row with an actual test ID was accepted: task=%v diagnostics=%+v", badTask, badDiagnostics)
	}
}

func TestSpecCompilerZeroArgumentOperation(t *testing.T) {
	source := `# Zero argument operation

Parameters: none.

| ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|
| call | f | reachable | return unit | other outcome | none | none | none | 1 | — |
`
	task, diagnostics := compileSpec(t, source)
	if semanticir.HasErrors(diagnostics) || task == nil {
		t.Fatalf("zero-argument spec rejected: task=%v diagnostics=%+v", task, diagnostics)
	}
	if len(task.Domains) != 0 || len(task.Operations) != 1 || len(task.Operations[0].DomainIDs) != 0 || len(task.Requirements) != 1 || len(task.Requirements[0].Conditions) != 0 {
		t.Fatalf("zero-argument semantic shape is wrong: %+v", task)
	}
	assignments := semanticir.EnumerateAssignments(nil)
	if len(assignments) != 1 || len(assignments[0]) != 0 {
		t.Fatalf("zero-domain product=%+v, want one empty assignment", assignments)
	}
}
