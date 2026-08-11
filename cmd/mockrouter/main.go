// Command mockrouter is a development-only stand-in for the OpenRouter chat
// completions API. It lets you exercise the full pipeline — section digests,
// pros and cons memos, parallel votes, JSON parsing, caching, refresh —
// without an API key or a bill.
//
// It is NOT a substitute for real model output: everything it returns is
// derived from a hash of the bill and model name, so anything it produces is
// fake.
//
//	go run ./cmd/mockrouter &
//	OPENROUTER_KEY=dev OPENROUTER_BASE_URL=http://127.0.0.1:8500/api/v1 go run ./cmd/server
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

var yesReasons = []string{
	"Delivers a needed public benefit with reasonable guardrails and a credible funding path.",
	"The compliance burden is proportionate to the harm the bill actually prevents.",
	"Modernizes an outdated framework without displacing existing state authority.",
	"Targets a well-documented gap and pairs the mandate with real implementation funding.",
	"On balance the measurable gains outweigh the modest administrative cost.",
}

var noReasons = []string{
	"The mandates outrun the evidence base and the enforcement provisions are too blunt.",
	"Costs fall on the parties least able to absorb them while the benefits stay speculative.",
	"Preempts state experimentation before anyone has shown the federal approach works.",
	"The drafting leaves key terms undefined, which invites litigation rather than compliance.",
	"Worthwhile goals, but the authorities granted here are broader than the problem requires.",
}

var pros = []string{
	"Sec. 2 pairs the mandate with real implementation funding.",
	"Sec. 4 leaves existing state authority intact.",
	"Sec. 6 sets deadlines an agency can actually meet.",
	"Sec. 7 requires an annual report Congress can audit.",
	"Sec. 8 sunsets the new authority rather than making it permanent.",
}

var cons = []string{
	"Sec. 3 leaves the key threshold undefined, which invites litigation.",
	"Sec. 5 preempts state experimentation before the federal approach is tested.",
	"The authorization in Sec. 2 is unoffset and grows every year.",
	"Compliance costs in Sec. 4 fall hardest on the smallest covered entities.",
	"Sec. 6 sets a deadline with no consequence for missing it.",
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8500", "listen address")
	delay := flag.Duration("delay", 400*time.Millisecond, "simulated per-call latency")
	flag.Parse()

	http.HandleFunc("/api/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var system, user string
		for _, m := range req.Messages {
			switch m.Role {
			case "system":
				system = m.Content
			case "user":
				user = m.Content
			}
		}

		time.Sleep(time.Duration(rand.Int63n(int64(*delay) + 1)))

		seed := hash(req.Model + "|" + firstLine(user))
		var content string
		// The stage is read off the system prompt, and the floor vote is
		// checked first because its prompt also refers to the memo.
		switch {
		case strings.Contains(system, "ideological direction"):
			score := float64(seed%201)/100 - 1
			content = fmt.Sprintf(`{"score": %.2f, "reason": "Mock ideology score."}`, score)

		case strings.Contains(system, "SECTION NOTE"):
			content = fmt.Sprintf(`{"note": %q}`, fmt.Sprintf(
				"Mock note on %s: authorizes appropriations, sets a compliance deadline, and directs an annual report.",
				partLabel(user)))

		case strings.Contains(system, "recorded floor vote"):
			vote := "Yes"
			reasons := yesReasons
			if seed%100 < 38 {
				vote = "No"
				reasons = noReasons
			}
			content = fmt.Sprintf(`{"vote": %q, "reason": %q}`, vote, reasons[seed%uint32(len(reasons))])

		case strings.Contains(system, "pros and cons memo"):
			content = fmt.Sprintf(`{"pros": [%q, %q, %q], "cons": [%q, %q]}`,
				pros[seed%uint32(len(pros))],
				pros[(seed+1)%uint32(len(pros))],
				pros[(seed+2)%uint32(len(pros))],
				cons[seed%uint32(len(cons))],
				cons[(seed+1)%uint32(len(cons))])

		default:
			content = `{"error": "mockrouter: unrecognised prompt"}`
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "mock-" + req.Model,
			"model":   req.Model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": content}}},
		})
	})

	log.Printf("mock OpenRouter listening on http://%s/api/v1 (development only)", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// partLabel echoes back which piece of a split bill the note is about, so the
// digest reads like notes on a real bill rather than five identical lines.
func partLabel(user string) string {
	for _, line := range strings.Split(user, "\n") {
		if strings.HasPrefix(line, "PART ") {
			return strings.TrimSpace(line)
		}
	}
	return "this part"
}
