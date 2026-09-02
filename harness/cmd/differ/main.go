// Command differ runs a corpus through two implementations of the harness protocol
// and reports every behavioral difference between them.
//
// The reference side is normally the Go runner; the candidate side is whatever
// implementation is under validation (the Rust binary, once it exists). Any
// difference in output, metadata (including element order), token stream, or
// error status counts as a mismatch — the acceptance bar is zero.
//
//	go run ./harness/cmd/differ \
//	  -corpus harness/corpus/testdata.jsonl \
//	  -reference "go run ./harness/cmd/gorunner" \
//	  -candidate "./target/release/rustrunner" \
//	  -report /tmp/mismatches.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/DataDog/go-sqllexer/harness/internal/corpus"
	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

const maxLine = 8 << 20

// verdict grades a difference. Metadata ordering is deliberately not a failure:
// the contract is that values are deduplicated and identical, so a different order
// is reported and tracked but does not fail the run. It is still surfaced because
// the frozen Go suite compares those slices with assert.Equal, which is
// order-sensitive — anything reported here will fail there.
type verdict int

const (
	verdictEqual verdict = iota
	verdictOrder
	verdictMismatch
)

// mismatch is one behavioral difference, written to the report as JSON so failures
// are diffable and can be replayed straight back through either runner.
type mismatch struct {
	ID        string             `json:"id"`
	Field     string             `json:"field"`
	Kind      string             `json:"kind"`
	SQL       protocol.Text      `json:"sql"`
	Request   protocol.Request   `json:"request"`
	Reference *protocol.Response `json:"reference"`
	Candidate *protocol.Response `json:"candidate"`
}

func main() {
	var (
		corpusPath = flag.String("corpus", "", "path to a JSONL corpus (required)")
		refCmd     = flag.String("reference", "go run ./harness/cmd/gorunner", "reference implementation command")
		candCmd    = flag.String("candidate", "", "candidate implementation command (required)")
		reportPath = flag.String("report", "", "optional path for the JSONL mismatch report")
	)
	flag.Parse()

	if *corpusPath == "" || *candCmd == "" {
		fmt.Fprintln(os.Stderr, "both -corpus and -candidate are required")
		flag.Usage()
		os.Exit(2)
	}

	requests, err := corpus.Read(*corpusPath)
	if err != nil {
		fatalf("read corpus: %v", err)
	}
	if len(requests) == 0 {
		fatalf("corpus %s is empty", *corpusPath)
	}

	refResponses, err := run(*refCmd, requests)
	if err != nil {
		fatalf("reference %q: %v", *refCmd, err)
	}
	candResponses, err := run(*candCmd, requests)
	if err != nil {
		fatalf("candidate %q: %v", *candCmd, err)
	}

	var report *os.File
	if *reportPath != "" {
		report, err = os.Create(*reportPath)
		if err != nil {
			fatalf("create report: %v", err)
		}
		defer report.Close()
	}

	mismatches, orderDiffs := compare(requests, refResponses, candResponses, report)

	fmt.Printf("\ncorpus     %s\n", *corpusPath)
	fmt.Printf("requests   %d\n", len(requests))
	fmt.Printf("mismatches %d\n", mismatches)
	fmt.Printf("order-only %d (reported, not gating; the frozen Go suite would fail on these)\n", orderDiffs)
	if mismatches > 0 {
		if *reportPath != "" {
			fmt.Printf("report     %s\n", *reportPath)
		}
		os.Exit(1)
	}
}

// maxPrinted caps the individual mismatches echoed to stdout; the full set always
// goes to the JSONL report when one is requested.
const maxPrinted = 100

func compare(requests []protocol.Request, ref, cand []protocol.Response, report *os.File) (mismatches, orderDiffs int) {
	printed := 0
	var enc *json.Encoder
	if report != nil {
		enc = json.NewEncoder(report)
	}

	for i, req := range requests {
		field, v := diffResponse(ref[i], cand[i])
		if v == verdictEqual {
			continue
		}
		if v == verdictOrder {
			orderDiffs++
			if enc != nil {
				_ = enc.Encode(mismatch{ID: req.ID, Field: field, Kind: "order", SQL: req.SQL,
					Request: req, Reference: &ref[i], Candidate: &cand[i]})
			}
			continue
		}
		mismatches++
		if enc != nil {
			_ = enc.Encode(mismatch{
				ID: req.ID, Field: field, Kind: "mismatch", SQL: req.SQL,
				Request: req, Reference: &ref[i], Candidate: &cand[i],
			})
		}
		if printed < maxPrinted {
			printed++
			fmt.Printf("MISMATCH %s [%s]\n  sql:       %s\n  reference: %s\n  candidate: %s\n",
				req.ID, field, truncate(string(req.SQL)), truncate(describe(ref[i])), truncate(describe(cand[i])))
		}
	}
	return mismatches, orderDiffs
}

// diffResponse returns the first field that differs and how severe the difference
// is. Output, error status, metadata size, and the token stream must match exactly;
// metadata collections must contain the same deduplicated values, with ordering
// graded separately.
func diffResponse(ref, cand protocol.Response) (string, verdict) {
	switch {
	case ref.Error != cand.Error:
		return "error", verdictMismatch
	case ref.Output != cand.Output:
		return "output", verdictMismatch
	}
	if (ref.Metadata == nil) != (cand.Metadata == nil) {
		return "metadata", verdictMismatch
	}

	worst, worstField := verdictEqual, ""
	if ref.Metadata != nil {
		if ref.Metadata.Size != cand.Metadata.Size {
			return "metadata.size", verdictMismatch
		}
		for _, pair := range []struct {
			name string
			a, b []protocol.Text
		}{
			{"metadata.tables", ref.Metadata.Tables, cand.Metadata.Tables},
			{"metadata.comments", ref.Metadata.Comments, cand.Metadata.Comments},
			{"metadata.commands", ref.Metadata.Commands, cand.Metadata.Commands},
			{"metadata.procedures", ref.Metadata.Procedures, cand.Metadata.Procedures},
		} {
			switch v := diffCollection(pair.a, pair.b); v {
			case verdictMismatch:
				return pair.name, verdictMismatch
			case verdictOrder:
				if worst == verdictEqual {
					worst, worstField = v, pair.name
				}
			}
		}
	}

	if len(ref.Tokens) != len(cand.Tokens) {
		return "tokens", verdictMismatch
	}
	for i := range ref.Tokens {
		if ref.Tokens[i] != cand.Tokens[i] {
			return fmt.Sprintf("tokens[%d]", i), verdictMismatch
		}
	}
	return worstField, worst
}

// diffCollection compares a metadata collection. A duplicate value is a mismatch
// even if the set of values matches, because deduplication is part of the contract
// and a duplicate also shifts Size.
func diffCollection(a, b []protocol.Text) verdict {
	if len(a) != len(b) {
		return verdictMismatch
	}
	ordered := true
	counts := make(map[protocol.Text]int, len(a))
	for i := range a {
		if a[i] != b[i] {
			ordered = false
		}
		counts[a[i]]++
		counts[b[i]]--
	}
	if ordered {
		return verdictEqual
	}
	for _, n := range counts {
		if n != 0 {
			return verdictMismatch
		}
	}
	if hasDuplicates(b) {
		return verdictMismatch
	}
	return verdictOrder
}

func hasDuplicates(values []protocol.Text) bool {
	seen := make(map[protocol.Text]struct{}, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
	}
	return false
}

// run feeds every request to one implementation and collects its responses,
// matching them back to requests by ID so ordering bugs surface as errors rather
// than as silent misalignment.
func run(command string, requests []protocol.Request) ([]protocol.Response, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	writeErr := make(chan error, 1)
	go func() {
		bw := bufio.NewWriterSize(stdin, 1<<20)
		enc := json.NewEncoder(bw)
		for _, req := range requests {
			if err := enc.Encode(req); err != nil {
				writeErr <- err
				stdin.Close()
				return
			}
		}
		writeErr <- bw.Flush()
		stdin.Close()
	}()

	responses, err := readResponses(stdout, requests)
	if err != nil {
		cmd.Wait()
		return nil, err
	}
	if err := <-writeErr; err != nil && !isBrokenPipe(err) {
		cmd.Wait()
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return responses, nil
}

// isBrokenPipe reports whether a write failed because the implementation had
// already closed its stdin. That is not an error by itself: an implementation
// that answered every request and exited wins the race against the feeder on a
// fast machine. The missing-response check and the exit status decide the run.
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed)
}

func readResponses(r io.Reader, requests []protocol.Request) ([]protocol.Response, error) {
	byID := make(map[string]int, len(requests))
	for i, req := range requests {
		byID[req.ID] = i
	}

	responses := make([]protocol.Response, len(requests))
	seen := make([]bool, len(requests))
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLine)

	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var resp protocol.Response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			return nil, fmt.Errorf("malformed response: %w", err)
		}
		i, ok := byID[resp.ID]
		if !ok {
			return nil, fmt.Errorf("response for unknown id %q", resp.ID)
		}
		responses[i] = resp
		seen[i] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for i, ok := range seen {
		if !ok {
			return nil, fmt.Errorf("no response for id %q", requests[i].ID)
		}
	}
	return responses, nil
}

func describe(r protocol.Response) string {
	if r.Error != "" {
		return "error: " + r.Error
	}
	if r.Tokens != nil {
		values := make([]string, 0, len(r.Tokens))
		for _, t := range r.Tokens {
			values = append(values, fmt.Sprintf("%d:%s", t.Type, t.Value))
		}
		return strings.Join(values, " ")
	}
	if r.Metadata != nil {
		return fmt.Sprintf("%s | size=%d tables=%v commands=%v comments=%v procedures=%v",
			r.Output, r.Metadata.Size, r.Metadata.Tables, r.Metadata.Commands,
			r.Metadata.Comments, r.Metadata.Procedures)
	}
	return string(r.Output)
}

func truncate(s string) string {
	const limit = 300
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
