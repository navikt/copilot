package cli

import "testing"

func TestModelRankPrefersNavPilotsOwnModel(t *testing.T) {
	servers := []foundServer{
		{setupCandidate{"llama-server", "127.0.0.1:8080"}, "http://127.0.0.1:8080", []string{"Qwen3.8-27B-Q4_K_M.gguf", "gemma-3-4b"}},
		{setupCandidate{"ollama", "127.0.0.1:11434"}, "http://127.0.0.1:11434", []string{"llama3:8b", "qwen3.6:35b", "qwen3.6-35b-navpilot:latest"}},
	}
	got := choices(servers)
	want := []string{"qwen3.6-35b-navpilot:latest", "qwen3.6:35b", "Qwen3.8-27B-Q4_K_M.gguf"}
	for i, w := range want {
		if got[i].Model != w {
			t.Fatalf("choices[%d] = %s, want %s (all: %+v)", i, got[i].Model, w, got)
		}
	}
	for id, want := range map[string]int{"llama3:8b": 0, "qwen3.6:235b": 0, "llama3-8b-navpilot:latest": 1, "Qwen3.6-35B-A3B-UD-Q4_K_XL.gguf": 4} {
		if r, _ := modelRank(id); r != want {
			t.Errorf("modelRank(%s) = %d, want %d", id, r, want)
		}
	}
}

func TestSetupCandidatesAreLoopbackOnly(t *testing.T) {
	t.Setenv("NAV_PILOT_SETUP_CANDIDATES", "ollama=127.0.0.1:1,vllm=[::1]:2")
	if c, err := setupCandidates(); err != nil || len(c) != 2 {
		t.Fatalf("setupCandidates = %v, %v", c, err)
	}
	for _, bad := range []string{"ollama=10.0.0.1:11434", "ollama=localhost:11434", "nope=127.0.0.1:1", "ollama"} {
		t.Setenv("NAV_PILOT_SETUP_CANDIDATES", bad)
		if _, err := setupCandidates(); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
