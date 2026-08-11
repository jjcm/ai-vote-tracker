// Command mockrouter is a development-only stand-in for the OpenRouter chat
// completions API. It lets you exercise the full voting pipeline — parallel
// calls, JSON parsing, caching, refresh — without an API key or a bill.
//
// It is NOT a substitute for real model output: verdicts are derived from a
// hash of the bill and model name, so anything it produces is fake.
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
		if strings.Contains(system, "ideological direction") {
			score := float64(seed%201)/100 - 1
			content = fmt.Sprintf(`{"score": %.2f, "reason": "Mock ideology score."}`, score)
		} else {
			vote := "Yes"
			reasons := yesReasons
			if seed%100 < 38 {
				vote = "No"
				reasons = noReasons
			}
			content = fmt.Sprintf(`{"vote": %q, "reason": %q}`, vote, reasons[seed%uint32(len(reasons))])
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
