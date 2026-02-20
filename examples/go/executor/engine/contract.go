package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Discovery is the response from /.well-known/covenant.
type Discovery struct {
	Version      string `json:"version"`
	Service      string `json:"service"`
	Description  string `json:"description"`
	ContractETag string `json:"contract_etag"`
	Persona      string `json:"persona"`
	Contracts    struct {
		Files []string `json:"files"`
	} `json:"contracts"`
}

// FetchDiscovery fetches and parses the discovery document.
func FetchDiscovery(serverURL string) (*Discovery, error) {
	resp, err := http.Get(serverURL + "/.well-known/covenant")
	if err != nil {
		return nil, fmt.Errorf("fetch discovery: %w", err)
	}
	defer resp.Body.Close()

	var disc Discovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, fmt.Errorf("decode discovery: %w", err)
	}
	return &disc, nil
}

// LoadContract fetches CUE files listed in the discovery doc, compiles them
// with the CUE Go SDK, and extracts a Contract struct.
func LoadContract(serverURL string, disc *Discovery) (*Contract, error) {
	ctx := cuecontext.New()

	var unified cue.Value
	for _, filePath := range disc.Contracts.Files {
		data, err := fetchFile(serverURL + filePath)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", filePath, err)
		}

		v := ctx.CompileBytes(data)
		if v.Err() != nil {
			return nil, fmt.Errorf("compile %s: %w", filePath, v.Err())
		}

		if !unified.Exists() {
			unified = v
		} else {
			unified = unified.Unify(v)
		}
	}

	if !unified.Exists() {
		return nil, fmt.Errorf("no contract files loaded")
	}
	if unified.Err() != nil {
		return nil, fmt.Errorf("unified contract error: %w", unified.Err())
	}

	return extractContract(unified)
}

func fetchFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extractContract walks the unified CUE value tree and populates a Contract.
func extractContract(v cue.Value) (*Contract, error) {
	c := &Contract{
		Facts:        make(map[string]FactDef),
		DerivedFacts: make(map[string]DerivedFactDef),
		Operations:   make(map[string]OperationDef),
		Entities:     make(map[string]EntityDef),
	}

	if err := extractFacts(v, c); err != nil {
		return nil, err
	}
	if err := extractDerivedFacts(v, c); err != nil {
		return nil, err
	}
	if err := extractRules(v, c); err != nil {
		return nil, err
	}
	if err := extractOperations(v, c); err != nil {
		return nil, err
	}
	if err := extractEntities(v, c); err != nil {
		return nil, err
	}

	return c, nil
}

func extractFacts(v cue.Value, c *Contract) error {
	factsVal := v.LookupPath(cue.ParsePath("facts"))
	if !factsVal.Exists() {
		return nil
	}

	iter, err := factsVal.Fields()
	if err != nil {
		return fmt.Errorf("iterate facts: %w", err)
	}

	for iter.Next() {
		name := iter.Selector().Unquoted()
		fv := iter.Value()

		def := FactDef{
			Required:  true,  // default
			OnMissing: "system_error", // default
		}

		if src, err := fv.LookupPath(cue.ParsePath("source")).String(); err == nil {
			def.Source = src
		}
		if req, err := fv.LookupPath(cue.ParsePath("required")).Bool(); err == nil {
			def.Required = req
		}
		if om, err := fv.LookupPath(cue.ParsePath("on_missing")).String(); err == nil {
			def.OnMissing = om
		}

		c.Facts[name] = def
	}
	return nil
}

func extractDerivedFacts(v cue.Value, c *Contract) error {
	dfVal := v.LookupPath(cue.ParsePath("derived_facts"))
	if !dfVal.Exists() {
		return nil
	}

	iter, err := dfVal.Fields()
	if err != nil {
		return fmt.Errorf("iterate derived_facts: %w", err)
	}

	for iter.Next() {
		name := iter.Selector().Unquoted()
		fv := iter.Value()

		derivVal := fv.LookupPath(cue.ParsePath("derivation"))
		jsonBytes, err := derivVal.MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal derivation for %s: %w", name, err)
		}

		var d Derivation
		if err := json.Unmarshal(jsonBytes, &d); err != nil {
			return fmt.Errorf("unmarshal derivation for %s: %w", name, err)
		}

		c.DerivedFacts[name] = DerivedFactDef{Derivation: d}
	}
	return nil
}

func extractRules(v cue.Value, c *Contract) error {
	rulesVal := v.LookupPath(cue.ParsePath("rules"))
	if !rulesVal.Exists() {
		return nil
	}

	jsonBytes, err := rulesVal.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}

	return json.Unmarshal(jsonBytes, &c.Rules)
}

func extractOperations(v cue.Value, c *Contract) error {
	opsVal := v.LookupPath(cue.ParsePath("operations"))
	if !opsVal.Exists() {
		return nil
	}

	iter, err := opsVal.Fields()
	if err != nil {
		return fmt.Errorf("iterate operations: %w", err)
	}

	for iter.Next() {
		name := iter.Selector().Unquoted()
		jsonBytes, err := iter.Value().MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal operation %s: %w", name, err)
		}
		var op OperationDef
		if err := json.Unmarshal(jsonBytes, &op); err != nil {
			return fmt.Errorf("unmarshal operation %s: %w", name, err)
		}
		c.Operations[name] = op
	}
	return nil
}

func extractEntities(v cue.Value, c *Contract) error {
	entVal := v.LookupPath(cue.ParsePath("entities"))
	if !entVal.Exists() {
		return nil
	}

	iter, err := entVal.Fields()
	if err != nil {
		return fmt.Errorf("iterate entities: %w", err)
	}

	for iter.Next() {
		name := iter.Selector().Unquoted()
		jsonBytes, err := iter.Value().MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal entity %s: %w", name, err)
		}
		var ent EntityDef
		if err := json.Unmarshal(jsonBytes, &ent); err != nil {
			return fmt.Errorf("unmarshal entity %s: %w", name, err)
		}
		c.Entities[name] = ent
	}
	return nil
}

// portName strips the "port:" prefix from a fact source, e.g. "port:customerRepo" → "customerRepo".
func portName(source string) string {
	return strings.TrimPrefix(source, "port:")
}
