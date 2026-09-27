package cli

// `nav-pilot alpha local setup`: guided local_endpoint. It looks for servers
// already running on this machine, picks the model that suits nav-pilot best,
// offers the fixes for the traps it knows (a model that is not pulled,
// Ollama's small context), checks the result with doctor, and saves it.
//
// It asks before anything that costs: a download, a new Ollama model, a config
// write. Without a terminal it does none of those unless a flag says so
// (--pull, --fix-context, --yes), and it never starts a server.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// setupCandidate is a place a local server usually listens, by default port.
type setupCandidate struct {
	Kind string // ollama, llama-server, lmstudio, vllm
	Addr string // host:port, loopback only
}

var defaultSetupCandidates = []setupCandidate{
	{"ollama", "127.0.0.1:11434"},
	{"llama-server", "127.0.0.1:8080"},
	{"llama-server", "127.0.0.1:8081"},
	{"lmstudio", "127.0.0.1:1234"},
	{"vllm", "127.0.0.1:8000"},
}

var kindName = map[string]string{"ollama": "Ollama", "llama-server": "llama-server", "lmstudio": "LM Studio", "vllm": "vLLM"}

// setupCandidates is where detection looks. NAV_PILOT_SETUP_CANDIDATES
// (kind=host:port,...) replaces the list, for tests and for a server on a
// port of its own; loopback addresses only, since detection runs unasked.
func setupCandidates() ([]setupCandidate, error) {
	raw := os.Getenv("NAV_PILOT_SETUP_CANDIDATES")
	if raw == "" {
		return defaultSetupCandidates, nil
	}
	var out []setupCandidate
	for _, part := range strings.Split(raw, ",") {
		kind, addr, ok := strings.Cut(strings.TrimSpace(part), "=")
		host, _, err := net.SplitHostPort(addr)
		ip := net.ParseIP(host)
		if !ok || kindName[kind] == "" || err != nil || ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("NAV_PILOT_SETUP_CANDIDATES: %q is not kind=127.0.0.1:port (kinds: ollama, llama-server, lmstudio, vllm)", part)
		}
		out = append(out, setupCandidate{kind, addr})
	}
	return out, nil
}

// foundServer is a candidate that answered, with the models it lists.
type foundServer struct {
	setupCandidate
	Base   string
	Models []string
}

// detectServers asks every candidate for its model list at once, with a short
// timeout: a GET of /v1/models and nothing else, so no prompt or file leaves
// the process. A port that answers with something other than an OpenAI model
// list is not a server of ours and is skipped.
func detectServers(ctx context.Context, cands []setupCandidate) []foundServer {
	found := make([]*foundServer, len(cands))
	var wg sync.WaitGroup
	for i, c := range cands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			base := "http://" + c.Addr
			ids, err := listModels(ctx, base, 2*time.Second)
			if err == nil {
				found[i] = &foundServer{c, base, ids}
			}
		}()
	}
	wg.Wait()
	var out []foundServer
	for _, f := range found {
		if f != nil {
			out = append(out, *f)
		}
	}
	return out
}

func listModels(ctx context.Context, base string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := local.ServerClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var list struct {
		Object string `json:"object"`
		Data   *[]struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &list) != nil || list.Object != "list" || list.Data == nil {
		return nil, errors.New("not an OpenAI model list")
	}
	var ids []string
	for _, m := range *list.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// knownGood maps model names to the models nav-pilot's own manifest runs,
// best first. A name matches when it contains every part, case-insensitively,
// so an Ollama tag (qwen3.6:35b), a GGUF file name and an HF repo all do.
var knownGood = []struct {
	Match *regexp.Regexp
	Why   string
}{
	{regexp.MustCompile(`qwen3\.6.*(^|[^0-9])35b`), "Qwen3.6-35B-A3B, the model nav-pilot's own MLX setup runs by default"},
	{regexp.MustCompile(`qwen3\.8.*(^|[^0-9])27b`), "Qwen3.8-27B, in nav-pilot's MLX manifest; dense, so slower on most machines"},
}

// ollamaRecommended is what setup offers to pull when Ollama has nothing
// known-good: the library build, which has Ollama's own tool-call parser (a
// hf.co pull does not).
const (
	ollamaRecommended   = "qwen3.6:35b"
	ollamaRecommendedGB = 23
	ggufRecommended     = "unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL"
	contextTokens       = 65536
)

// modelRank scores a model id: 0 unknown, higher is better. A copy setup made
// with a raised context ranks above the model it was made from, known or not.
func modelRank(id string) (int, string) {
	l := strings.ToLower(id)
	copied := 0
	if strings.HasSuffix(l, "-navpilot") || strings.HasSuffix(l, "-navpilot:latest") {
		copied = 1
	}
	for i, k := range knownGood {
		if k.Match.MatchString(l) {
			return 2*(len(knownGood)-i) + copied, k.Why
		}
	}
	return copied, ""
}

// setupChoice is one server and one of its models.
type setupChoice struct {
	Server foundServer
	Model  string
	Rank   int
	Why    string
}

// choices lists every server and model, best first. Ties keep detection
// order, which is the candidates' order: Ollama first.
func choices(servers []foundServer) []setupChoice {
	var out []setupChoice
	for _, s := range servers {
		for _, m := range s.Models {
			// llama-server without --alias lists its -m path, which cannot be
			// a model id. Said here, since otherwise it is simply missing.
			if err := validateModelValue(m); err != nil {
				fmt.Printf("  %s %s on %s cannot be used as a model id. Start it with a name: %s\n",
					yellow("⚠"), m, kindName[s.Kind], bold("--alias qwen3.6-35b"))
				continue
			}
			rank, why := modelRank(m)
			out = append(out, setupChoice{s, m, rank, why})
		}
	}
	slices.SortStableFunc(out, func(a, b setupChoice) int { return b.Rank - a.Rank })
	return out
}

type setupFlags struct {
	yes, pull, fixContext bool
	endpoint, model       string
}

func parseSetupFlags(args []string) (setupFlags, error) {
	var f setupFlags
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--yes":
			f.yes = true
		case "--pull":
			f.pull = true
		case "--fix-context":
			f.fixContext = true
		case "--endpoint", "--model":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s needs a value", a)
			}
			i++
			if a == "--endpoint" {
				f.endpoint = args[i]
			} else {
				f.model = args[i]
			}
		default:
			return f, fmt.Errorf("unknown flag %s. setup takes --yes, --pull, --fix-context, --endpoint <url> and --model <id>", a)
		}
	}
	return f, nil
}

// confirm asks in a terminal, and without one answers with the flag that
// stands for yes. def is the answer Enter gives: no for anything that
// downloads or creates.
func confirm(title string, flag, def bool) bool {
	if flag || !isInteractive() {
		return flag
	}
	ok := def
	if err := huh.NewConfirm().Title(title).Value(&ok).WithTheme(navTheme()).Run(); err != nil {
		return false
	}
	return ok
}

func cmdLocalSetup(args []string) error {
	f, err := parseSetupFlags(args)
	if err != nil {
		return &exitCode{code: 2, err: err}
	}
	// Before any request: --pull or --fix-context without --yes would spend
	// the download and then refuse to save.
	if !isInteractive() && !f.yes && (f.pull || f.fixContext) {
		return &exitCode{code: 2, err: errors.New("without a terminal, --pull and --fix-context go with --yes, which saves the result")}
	}
	ctx := context.Background()
	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local setup"), dim("(alpha, unsupported, unmeasured)"))
	mac := runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
	if mac {
		fmt.Printf("  %s\n", dim("This Mac can also run a measured MLX model that nav-pilot manages itself: nav-pilot alpha local init"))
		fmt.Printf("  %s\n\n", dim("Below: a server you run yourself (Ollama, llama-server, LM Studio) instead."))
	}

	servers, err := findServers(ctx, f)
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return noServerFound(mac)
	}

	choice, err := pickChoice(ctx, servers, f)
	if err != nil {
		return err
	}
	fmt.Printf("\n  Using  %s on %s %s\n\n", bold(choice.Model), kindName[choice.Server.Kind], dim(choice.Server.Base+"/v1"))
	// Refused before the checks, not after: the context probe can take
	// minutes on a CPU, and a script would otherwise wait through it twice.
	if !f.yes && !isInteractive() {
		again := "nav-pilot alpha local setup --model " + choice.Model + " --yes"
		if f.endpoint != "" {
			again = "nav-pilot alpha local setup --endpoint " + choice.Server.Base + "/v1 --model " + choice.Model + " --yes"
		}
		if choice.Rank == 0 && choice.Server.Kind == "ollama" {
			return &exitCode{code: 2, err: fmt.Errorf("without a terminal to ask, setup checks and saves only with --yes. With the recommended model: %s. With %s anyway: %s",
				bold("nav-pilot alpha local setup --pull --yes"), choice.Model, bold(again))}
		}
		return &exitCode{code: 2, err: fmt.Errorf("without a terminal to ask, setup checks and saves only with --yes: %s", bold(again))}
	}

	checks := runDoctor(ctx, choice.Server.Base, choice.Model)
	if choice.Server.Kind == "ollama" && failed(checks, "context") {
		fixed, err := fixOllamaContext(ctx, choice, f.fixContext)
		if err != nil {
			return err
		}
		if fixed != "" {
			choice.Model = fixed
			fmt.Printf("\n  Checking %s again:\n\n", bold(fixed))
			checks = runDoctor(ctx, choice.Server.Base, choice.Model)
		}
	}
	if slices.ContainsFunc(checks, func(c doctorCheck) bool { return c.Level == levelFail }) {
		again := "nav-pilot alpha local setup"
		if choice.Server.Kind == "ollama" && failed(checks, "context") {
			again += " --fix-context --yes"
		}
		return &exitCode{code: 1, err: fmt.Errorf("nothing was saved. Fix the FAIL lines above, then run %s", bold(again))}
	}
	return saveEndpoint(choice, checks, f.yes)
}

// findServers is detection, or with --endpoint the one server named there.
func findServers(ctx context.Context, f setupFlags) ([]foundServer, error) {
	if f.endpoint != "" {
		base, err := local.ValidateEndpoint(f.endpoint)
		if err != nil {
			return nil, err
		}
		ids, err := listModels(ctx, base, 5*time.Second)
		if err != nil {
			return nil, fmt.Errorf("%s does not answer with a model list (%v). Start the server, then run setup again", base+"/v1", err)
		}
		// Only Ollama answers /api/version; the others behave alike here.
		kind := "llama-server"
		if isOllama(ctx, base) {
			kind = "ollama"
		}
		return []foundServer{{setupCandidate{kind, strings.TrimPrefix(strings.TrimPrefix(base, "http://"), "https://")}, base, ids}}, nil
	}
	cands, err := setupCandidates()
	if err != nil {
		return nil, err
	}
	fmt.Printf("  %s\n", dim("Looking for servers on this machine (a model list request to each usual port, nothing else)…"))
	servers := detectServers(ctx, cands)
	for _, c := range cands {
		i := slices.IndexFunc(servers, func(s foundServer) bool { return s.Addr == c.Addr })
		if i < 0 {
			fmt.Printf("    %s %-12s %s\n", dim("·"), kindName[c.Kind], dim(c.Addr+" not answering"))
			continue
		}
		models := strings.Join(servers[i].Models, ", ")
		if models == "" {
			models = "no models"
		}
		fmt.Printf("    %s %-12s %s  %s\n", green("✓"), kindName[c.Kind], c.Addr, models)
	}
	return servers, nil
}

func isOllama(ctx context.Context, base string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := local.ServerClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var v struct {
		Version string `json:"version"`
	}
	return resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&v) == nil && v.Version != ""
}

// noServerFound says how to start one, with the exact commands.
func noServerFound(mac bool) error {
	fmt.Printf("\n  No local server is running. Start one, then run %s again:\n\n", bold("nav-pilot alpha local setup"))
	fmt.Printf("  %s\n", bold("Ollama")+dim(" (easiest; https://ollama.com/download)"))
	fmt.Printf("    OLLAMA_CONTEXT_LENGTH=%d ollama serve\n\n", contextTokens)
	fmt.Printf("  %s\n", bold("llama-server")+dim(" (llama.cpp; faster on a small GPU)"))
	fmt.Printf("    llama-server --jinja -c %d --port 8080 -hf %s\n", contextTokens, ggufRecommended)
	fmt.Printf("    %s\n\n", dim("On a GPU with 8 GB or less, add --n-cpu-moe 999 to keep the experts in RAM."))
	if mac {
		fmt.Printf("  Or let nav-pilot run MLX on this Mac: %s\n\n", bold("nav-pilot alpha local init"))
	}
	return &exitCode{code: 1}
}

// pickChoice picks the server and model: --model when given, otherwise the
// best-ranked one, which a terminal can change. With nothing known-good on
// Ollama it offers the pull first.
func pickChoice(ctx context.Context, servers []foundServer, f setupFlags) (setupChoice, error) {
	all := choices(servers)
	if f.model != "" {
		for _, c := range all {
			if c.Model == f.model {
				return c, nil
			}
		}
		return setupChoice{}, fmt.Errorf("no server here lists %s. Pick one of: %s", f.model, choiceList(all))
	}
	if len(all) == 0 || all[0].Rank == 0 {
		if i := slices.IndexFunc(servers, func(s foundServer) bool { return s.Kind == "ollama" }); i >= 0 {
			pulled, err := offerPull(ctx, servers[i], f.pull)
			if err != nil {
				return setupChoice{}, err
			}
			if pulled {
				return setupChoice{servers[i], ollamaRecommended, 1, knownGood[0].Why}, nil
			}
		} else {
			fmt.Printf("\n  %s None of these is a model nav-pilot knows. The closest to its own is:\n", yellow("⚠"))
			fmt.Printf("    llama-server --jinja -c %d --port 8080 -hf %s --alias qwen3.6-35b\n", contextTokens, ggufRecommended)
		}
		if len(all) > 0 {
			fmt.Printf("  %s Continuing with %s, which nav-pilot knows nothing about.\n", yellow("⚠"), bold(all[0].Model))
		}
	}
	if len(all) == 0 {
		fmt.Printf("\n  The server here lists no model nav-pilot can use. Load or pull one (on Ollama: %s), then run %s again.\n\n",
			bold("ollama pull "+ollamaRecommended), bold("nav-pilot alpha local setup"))
		return setupChoice{}, &exitCode{code: 1}
	}
	if all[0].Rank > 0 {
		fmt.Printf("\n  Recommended  %s on %s: %s\n", bold(all[0].Model), kindName[all[0].Server.Kind], all[0].Why)
	}
	if len(all) == 1 || !isInteractive() {
		return all[0], nil
	}
	var opts []huh.Option[int]
	for i, c := range all {
		label := fmt.Sprintf("%s  (%s)", c.Model, kindName[c.Server.Kind])
		if c.Rank > 0 && i == 0 {
			label += "  recommended"
		}
		opts = append(opts, huh.NewOption(label, i))
	}
	pick := 0
	if err := huh.NewSelect[int]().Title("Which model should nav-pilot use?").Options(opts...).Value(&pick).WithTheme(navTheme()).Run(); err != nil {
		return setupChoice{}, cancelledError{nothingWritten: true}
	}
	return all[pick], nil
}

func choiceList(all []setupChoice) string {
	var s []string
	for _, c := range all {
		s = append(s, c.Model)
	}
	return strings.Join(s, ", ")
}

// offerPull offers to pull the recommended model through Ollama's own API,
// which is what `ollama pull` does, showing the size first.
func offerPull(ctx context.Context, s foundServer, flag bool) (bool, error) {
	cmd := "ollama pull " + ollamaRecommended
	fmt.Printf("\n  %s Ollama has no model nav-pilot knows. Recommended: %s (%s), about %d GB:\n    %s\n",
		yellow("⚠"), bold(ollamaRecommended), knownGood[0].Why, ollamaRecommendedGB, bold(cmd))
	if !confirm(fmt.Sprintf("Download %s now (about %d GB)?", ollamaRecommended, ollamaRecommendedGB), flag, false) {
		if !isInteractive() {
			fmt.Printf("  %s\n", dim("Not downloaded. Run it yourself, or pass --pull to have setup run it."))
		}
		return false, nil
	}
	fmt.Printf("%s Pulling %s…\n", dim("→"), ollamaRecommended)
	if err := ollamaStream(ctx, s.Base+"/api/pull", map[string]any{"model": ollamaRecommended}); err != nil {
		return false, fmt.Errorf("%s failed: %w", cmd, err)
	}
	fmt.Printf("\r%s\r%s Pulled %s.\n", strings.Repeat(" ", 80), green("✓"), ollamaRecommended)
	return true, nil
}

// fixOllamaContext offers the num_ctx fix: a copy of the model with a 64k
// context, made by Ollama from the weights it already has. That holds across
// server restarts, which an OLLAMA_CONTEXT_LENGTH set in one shell does not.
// It returns the new model's name, or "" when declined.
func fixOllamaContext(ctx context.Context, c setupChoice, flag bool) (string, error) {
	name := strings.NewReplacer(":", "-", "/", "-").Replace(strings.TrimSuffix(c.Model, ":latest")) + "-navpilot"
	fmt.Printf("\n  %s Ollama cut the prompt to its default context. nav-pilot can make %s, a copy of %s with a %d-token context. It downloads nothing and shares the weights. By hand:\n",
		yellow("⚠"), bold(name), c.Model, contextTokens)
	fmt.Printf("    printf 'FROM %s\\nPARAMETER num_ctx %d\\n' > Modelfile\n    ollama create %s -f Modelfile\n", c.Model, contextTokens, name)
	fmt.Printf("  %s\n", dim(fmt.Sprintf("Or restart Ollama with OLLAMA_CONTEXT_LENGTH=%d ollama serve, which raises it for every model.", contextTokens)))
	if !confirm(fmt.Sprintf("Create %s now?", name), flag, false) {
		if !isInteractive() {
			fmt.Printf("  %s\n", dim("Not created. Pass --fix-context to have setup create it."))
		}
		return "", nil
	}
	err := ollamaStream(ctx, c.Server.Base+"/api/create", map[string]any{
		"model": name, "from": c.Model, "parameters": map[string]any{"num_ctx": contextTokens},
	})
	if err != nil {
		return "", fmt.Errorf("ollama create %s failed: %w", name, err)
	}
	fmt.Printf("%s Created %s.\n", green("✓"), name)
	return name, nil
}

// ollamaStream posts to one of Ollama's streaming endpoints (/api/pull,
// /api/create) and follows its progress lines until "success" or an error.
func ollamaStream(ctx context.Context, url string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	resp, err := local.ServerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s: %s", resp.Status, firstLineOf(raw))
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var line struct {
			Status    string `json:"status"`
			Error     string `json:"error"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		switch {
		case line.Error != "":
			return errors.New(line.Error)
		case line.Status == "success":
			return nil
		case line.Total > 0 && providerpkg.IsTerminal(os.Stdout):
			progressLine(fmt.Sprintf("%s %.1f/%.1f GB", line.Status, float64(line.Completed)/1e9, float64(line.Total)/1e9))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("the server stopped before it said success")
}

func failed(checks []doctorCheck, name string) bool {
	return slices.ContainsFunc(checks, func(c doctorCheck) bool { return c.Name == name && c.Level == levelFail })
}

// saveEndpoint writes the three keys through the normal config path, after a
// yes, and says what now works.
func saveEndpoint(c setupChoice, checks []doctorCheck, yes bool) error {
	endpoint := c.Server.Base + "/v1"
	fmt.Printf("  Save to %s:\n    local_endpoint       = %s\n    local_endpoint_model = %s\n    local_enabled        = true\n", configPath(), endpoint, c.Model)
	if !confirm("Save and turn local dispatch on?", yes, true) {
		return cancelledError{nothingWritten: true}
	}
	for _, kv := range [][2]string{{"local_endpoint", endpoint}, {"local_endpoint_model", c.Model}, {"local_enabled", "true"}} {
		if _, err := writeConfigKey(kv[0], kv[1]); err != nil {
			return err
		}
	}
	fmt.Printf("\n%s Saved. Local dispatch is on, to %s on %s.\n", green("✓"), bold(c.Model), kindName[c.Server.Kind])
	fmt.Printf("  %s Launch with a local worker: %s\n", dim("→"), bold("nav-pilot --client opencode"))
	if slices.ContainsFunc(checks, func(d doctorCheck) bool { return d.Name == "logprobs" && d.Level != levelPass }) {
		fmt.Printf("  %s alpha decide will refuse this server: see logprobs above\n", yellow("⚠"))
	} else {
		fmt.Printf("  %s nav-pilot alpha decide answers from it\n", green("✓"))
	}
	start := "ollama serve"
	if c.Server.Kind != "ollama" {
		start = "the command you started " + kindName[c.Server.Kind] + " with"
	}
	fmt.Printf("  %s\n\n", dim("nav-pilot starts nothing: after a reboot, start it again with "+start+". Check it: nav-pilot alpha local doctor"))
	return nil
}
