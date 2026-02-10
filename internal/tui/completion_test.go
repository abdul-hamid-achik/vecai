package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionEngineSlashTrigger(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// Typing "/" should activate with all commands
	engine.Update("/")
	if !engine.IsActive() {
		t.Fatal("Expected engine to be active after /")
	}
	if len(engine.items) == 0 {
		t.Fatal("Expected items after /")
	}
	if engine.ActiveTrigger() != TriggerSlash {
		t.Errorf("Expected TriggerSlash, got %v", engine.ActiveTrigger())
	}
}

func TestCompletionEngineSlashFiltering(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.Update("/mo")
	if !engine.IsActive() {
		t.Fatal("Expected engine to be active after /mo")
	}
	// Should match /mode
	found := false
	for _, item := range engine.items {
		if item.InsertText == "/mode " {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected /mode in results for /mo")
	}
}

func TestCompletionEngineSlashArgCompletion(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// After command + space, should show arguments
	engine.Update("/mode ")
	if !engine.IsActive() {
		t.Fatal("Expected engine to be active for /mode args")
	}

	// Should have fast, smart, genius
	labels := make(map[string]bool)
	for _, item := range engine.items {
		labels[item.Label] = true
	}
	for _, expected := range []string{"fast", "smart", "genius"} {
		if !labels[expected] {
			t.Errorf("Expected %q in arg completions, got %v", expected, labels)
		}
	}
}

func TestCompletionEngineSlashArgFiltering(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.Update("/mode sm")
	if !engine.IsActive() {
		t.Fatal("Expected engine to be active for /mode sm")
	}
	if len(engine.items) != 1 {
		t.Fatalf("Expected 1 item for /mode sm, got %d", len(engine.items))
	}
	if engine.items[0].Label != "smart" {
		t.Errorf("Expected smart, got %s", engine.items[0].Label)
	}
}

func TestCompletionEngineDismiss(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.Update("/")
	if !engine.IsActive() {
		t.Fatal("Expected active")
	}

	engine.Dismiss()
	if engine.IsActive() {
		t.Error("Expected inactive after dismiss")
	}
}

func TestCompletionEngineAccept(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.Update("/hel")
	if !engine.IsActive() {
		t.Fatal("Expected active")
	}

	item := engine.Accept()
	if item.InsertText != "/help" {
		t.Errorf("Expected /help, got %q", item.InsertText)
	}
	if item.Kind != KindCommand {
		t.Errorf("Expected KindCommand, got %v", item.Kind)
	}
	if engine.IsActive() {
		t.Error("Expected inactive after accept")
	}
}

func TestCompletionEngineNavigation(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.Update("/")
	initial := engine.selected

	engine.MoveDown()
	if engine.selected != initial+1 {
		t.Errorf("Expected selected=%d after MoveDown, got %d", initial+1, engine.selected)
	}

	engine.MoveUp()
	if engine.selected != initial {
		t.Errorf("Expected selected=%d after MoveUp, got %d", initial, engine.selected)
	}

	// Wrap around up
	engine.MoveUp()
	if engine.selected != len(engine.items)-1 {
		t.Errorf("Expected wrap to last item, got %d", engine.selected)
	}
}

func TestCompletionEngineNoTrigger(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.Update("hello world")
	if engine.IsActive() {
		t.Error("Expected inactive for plain text")
	}

	engine.Update("")
	if engine.IsActive() {
		t.Error("Expected inactive for empty input")
	}
}

func TestCompletionEngineRender(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// Inactive
	if engine.Render(80) != "" {
		t.Error("Expected empty render when inactive")
	}

	// Active
	engine.Update("/hel")
	rendered := engine.Render(80)
	if rendered == "" {
		t.Error("Expected non-empty render when active")
	}
}

func TestCompletionEngineCommandHasArgs(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// /mode has args, so InsertText should include trailing space
	engine.Update("/mode")
	if !engine.IsActive() {
		t.Fatal("Expected active")
	}
	item := engine.Accept()
	if item.InsertText != "/mode " {
		t.Errorf("Expected '/mode ' with trailing space, got %q", item.InsertText)
	}
}

func TestCompletionEngineCommandNoArgs(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// /help has no args, so InsertText should NOT include trailing space
	engine.Update("/hel")
	item := engine.Accept()
	if item.InsertText != "/help" {
		t.Errorf("Expected '/help' without trailing space, got %q", item.InsertText)
	}
}

func TestSlashProviderAddCommands(t *testing.T) {
	p := NewSlashCommandProvider()
	initial := len(p.commands)

	p.AddCommands([]CommandDef{
		{Name: "/custom", Description: "Custom command"},
	})

	if len(p.commands) != initial+1 {
		t.Errorf("Expected %d commands, got %d", initial+1, len(p.commands))
	}

	// Should be findable
	items := p.Complete("cust")
	if len(items) != 1 {
		t.Fatalf("Expected 1 item for 'cust', got %d", len(items))
	}
	if items[0].InsertText != "/custom" {
		t.Errorf("Expected /custom, got %s", items[0].InsertText)
	}
}

func TestFindActiveTrigger(t *testing.T) {
	tests := []struct {
		input   string
		trigger byte
		want    int
	}{
		{"@file", '@', 0},
		{"hello @file", '@', 6},
		{"hello @file done", '@', -1}, // space after query = dismissed
		{"", '@', -1},
		{"no trigger", '@', -1},
		{"hello @", '@', 6},
		{"@@double", '@', 0}, // first @ at start-of-string matches
	}

	for _, tt := range tests {
		got := findActiveTrigger(tt.input, tt.trigger)
		if got != tt.want {
			t.Errorf("findActiveTrigger(%q, %q) = %d, want %d", tt.input, string(tt.trigger), got, tt.want)
		}
	}
}

func TestFileLangBadge(t *testing.T) {
	// Just ensure no panics and reasonable output
	tests := []string{"go", "python", "javascript", "typescript", "rust", "yaml", "markdown", "", "unknown", "c"}
	for _, lang := range tests {
		badge := fileLangBadge(lang)
		if badge == "" {
			t.Errorf("Expected non-empty badge for %q", lang)
		}
	}
}

// --- Phase 2: FileMentionProvider, FileCache, ParseFileTags tests ---

func TestParseFileTags(t *testing.T) {
	tests := []struct {
		input     string
		wantClean string
		wantTags  []string // RelPath values
	}{
		{"hello @main.go world", "hello world", []string{"main.go"}},
		{"@router.go explain this", "explain this", []string{"router.go"}},
		{"@foo.go @bar.py do stuff", "do stuff", []string{"foo.go", "bar.py"}},
		{"no mentions here", "no mentions here", nil},
		{"@internal/tui/model.go how does this work", "how does this work", []string{"internal/tui/model.go"}},
		{"", "", nil},
		{"@nodot should not match", "@nodot should not match", nil}, // no extension = no match
	}

	for _, tt := range tests {
		result := ParseFileTags(tt.input, "")
		if result.CleanQuery != tt.wantClean {
			t.Errorf("ParseFileTags(%q).CleanQuery = %q, want %q", tt.input, result.CleanQuery, tt.wantClean)
		}
		if len(result.NewTags) != len(tt.wantTags) {
			t.Errorf("ParseFileTags(%q).NewTags = %d tags, want %d", tt.input, len(result.NewTags), len(tt.wantTags))
			continue
		}
		for i, tag := range result.NewTags {
			if tag.RelPath != tt.wantTags[i] {
				t.Errorf("ParseFileTags(%q).NewTags[%d].RelPath = %q, want %q", tt.input, i, tag.RelPath, tt.wantTags[i])
			}
		}
	}
}

func TestParseFileTagsLanguageDetection(t *testing.T) {
	result := ParseFileTags("@main.go test", "")
	if len(result.NewTags) != 1 {
		t.Fatalf("Expected 1 tag, got %d", len(result.NewTags))
	}
	if result.NewTags[0].Language != "Go" {
		t.Errorf("Expected language Go, got %q", result.NewTags[0].Language)
	}

	result = ParseFileTags("@script.py test", "")
	if len(result.NewTags) != 1 {
		t.Fatalf("Expected 1 tag, got %d", len(result.NewTags))
	}
	if result.NewTags[0].Language != "Python" {
		t.Errorf("Expected language Python, got %q", result.NewTags[0].Language)
	}
}

func TestMatchScore(t *testing.T) {
	tests := []struct {
		path  string
		query string
		min   float64 // minimum expected score
		max   float64 // maximum expected score
	}{
		{"main.go", "main", 0.9, 1.1},          // exact name match
		{"main.go", "mai", 0.85, 0.95},          // prefix match
		{"some_main.go", "main", 0.75, 0.85},    // base contains
		{"internal/agent/main.go", "agent", 0.5, 0.7}, // path contains
		{"model.go", "mdl", 0.3, 0.5},           // fuzzy match
		{"model.go", "xyz", -0.1, 0.1},          // no match
	}

	for _, tt := range tests {
		score := matchScore(tt.path, tt.query)
		if score < tt.min || score > tt.max {
			t.Errorf("matchScore(%q, %q) = %.2f, want [%.2f, %.2f]", tt.path, tt.query, score, tt.min, tt.max)
		}
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		target string
		query  string
		want   bool
	}{
		{"model.go", "mdl", true},
		{"model.go", "mod", true},
		{"model.go", "xyz", false},
		{"router_test.go", "rtg", true},
		{"a", "ab", false}, // query longer than target
	}

	for _, tt := range tests {
		got := fuzzyMatch(tt.target, tt.query)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.target, tt.query, got, tt.want)
		}
	}
}

func TestFileMentionProviderComplete(t *testing.T) {
	// Create a temp dir with some test files
	tmpDir := t.TempDir()
	for _, name := range []string{"main.go", "router.go", "model.py", "README.md"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	provider := NewFileMentionProvider(tmpDir)

	// Force fallback walk (no git in temp dir)
	items := provider.Complete("main")
	if len(items) == 0 {
		t.Fatal("Expected completions for 'main'")
	}
	if items[0].Kind != KindFile {
		t.Errorf("Expected KindFile, got %v", items[0].Kind)
	}
	if items[0].Language != "Go" {
		t.Errorf("Expected Go language, got %q", items[0].Language)
	}

	// Empty query returns nil
	items = provider.Complete("")
	if items != nil {
		t.Error("Expected nil for empty query")
	}
}

func TestFileMentionProviderTrigger(t *testing.T) {
	provider := NewFileMentionProvider("")
	if provider.Trigger() != TriggerAt {
		t.Errorf("Expected TriggerAt, got %v", provider.Trigger())
	}
	if provider.IsAsync() {
		t.Error("Expected IsAsync=false")
	}
}

func TestFileCacheRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewFileCache(tmpDir)
	files := cache.Files()
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	if files[0] != "a.go" {
		t.Errorf("Expected a.go, got %q", files[0])
	}

	// Invalidate and add another file
	if err := os.WriteFile(filepath.Join(tmpDir, "b.py"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	cache.Invalidate()
	files = cache.Files()
	if len(files) != 2 {
		t.Fatalf("Expected 2 files after invalidation, got %d", len(files))
	}
}

func TestFileCacheTTL(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewFileCache(tmpDir)
	files1 := cache.Files()

	// Same reference returned within TTL
	files2 := cache.Files()
	if len(files1) != len(files2) {
		t.Error("Expected same files within TTL")
	}
}

func TestShouldSkipDir(t *testing.T) {
	skip := []string{".git", "node_modules", "vendor", "__pycache__", ".hidden"}
	keep := []string{"src", "internal", "pkg", "cmd"}

	for _, name := range skip {
		if !shouldSkipDir(name) {
			t.Errorf("Expected skip for %q", name)
		}
	}
	for _, name := range keep {
		if shouldSkipDir(name) {
			t.Errorf("Expected keep for %q", name)
		}
	}
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want bool
	}{
		{"main.go", 100, false},
		{"script.py", 100, false},
		{"image.png", 100, true},
		{"big.go", 2 * 1024 * 1024, true}, // > 1MB
		{"package-lock.json", 100, true},
		{"go.sum", 100, true},
	}

	for _, tt := range tests {
		got := shouldSkipFile(tt.name, tt.size)
		if got != tt.want {
			t.Errorf("shouldSkipFile(%q, %d) = %v, want %v", tt.name, tt.size, got, tt.want)
		}
	}
}

func TestRenderFileTagChips(t *testing.T) {
	// Empty case
	if got := renderFileTagChips(nil, 80); got != "" {
		t.Errorf("Expected empty for nil, got %q", got)
	}

	// Single file
	tags := []TaggedFile{{RelPath: "main.go", Language: "Go"}}
	got := renderFileTagChips(tags, 80)
	if got == "" {
		t.Error("Expected non-empty for single tag")
	}

	// Multiple files with overflow
	tags = []TaggedFile{
		{RelPath: "main.go"},
		{RelPath: "router.go"},
		{RelPath: "pipeline.go"},
		{RelPath: "model.go"},
		{RelPath: "view.go"},
	}
	got = renderFileTagChips(tags, 40) // Narrow width forces "+N more"
	if got == "" {
		t.Error("Expected non-empty for multiple tags")
	}
}

func TestExtFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"internal/tui/model.py", "py"},
		{"no-ext", ""},
		{"path/no-ext", ""},
		{".hidden", "hidden"},
	}
	for _, tt := range tests {
		got := extFromPath(tt.path)
		if got != tt.want {
			t.Errorf("extFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestLangFromExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".go", "Go"},
		{".py", "Python"},
		{".js", "JavaScript"},
		{".ts", "TypeScript"},
		{".rs", "Rust"},
		{".unknown", ""},
	}
	for _, tt := range tests {
		got := langFromExt(tt.ext)
		if got != tt.want {
			t.Errorf("langFromExt(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestShortName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "main.go"},
		{"internal/tui/model.go", "model.go"},
		{"a/b/c.py", "c.py"},
	}
	for _, tt := range tests {
		got := shortName(tt.path)
		if got != tt.want {
			t.Errorf("shortName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{-5, "-5"},
	}
	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCompletionEngineFileProvider(t *testing.T) {
	// Create temp dir with test files
	tmpDir := t.TempDir()
	for _, name := range []string{"agent.go", "pipeline.go", "router.go"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	provider := NewFileMentionProvider(tmpDir)
	engine := NewCompletionEngine(NewSlashCommandProvider(), provider)

	// @ trigger should activate file completion
	engine.Update("hello @ag")
	if !engine.IsActive() {
		t.Fatal("Expected engine active after @ag")
	}
	if engine.ActiveTrigger() != TriggerAt {
		t.Errorf("Expected TriggerAt, got %v", engine.ActiveTrigger())
	}

	found := false
	for _, item := range engine.items {
		if item.Label == "agent.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected agent.go in file completions")
	}
}

func TestModelTaggedFilesDedup(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))

	m.AddTaggedFile(TaggedFile{RelPath: "main.go", Language: "Go"})
	m.AddTaggedFile(TaggedFile{RelPath: "main.go", Language: "Go"}) // duplicate
	m.AddTaggedFile(TaggedFile{RelPath: "router.go", Language: "Go"})

	files := m.GetTaggedFiles()
	if len(files) != 2 {
		t.Errorf("Expected 2 files (deduped), got %d", len(files))
	}
}

func TestModelTaggedFilesRemove(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))

	m.AddTaggedFile(TaggedFile{RelPath: "a.go"})
	m.AddTaggedFile(TaggedFile{RelPath: "b.go"})
	m.RemoveTaggedFile("a.go")

	files := m.GetTaggedFiles()
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	if files[0].RelPath != "b.go" {
		t.Errorf("Expected b.go, got %q", files[0].RelPath)
	}
}

func TestModelClearTaggedFiles(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))

	m.AddTaggedFile(TaggedFile{RelPath: "a.go"})
	m.AddTaggedFile(TaggedFile{RelPath: "b.go"})
	m.ClearTaggedFiles()

	if len(m.GetTaggedFiles()) != 0 {
		t.Error("Expected 0 files after clear")
	}
}

// --- Phase 3: Vecgrep async completion tests ---

func TestMergeAsyncResultsEmpty(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// Merging empty results should just clear loading
	engine.loading = true
	engine.MergeAsyncResults(nil)
	if engine.loading {
		t.Error("Expected loading=false after empty merge")
	}
}

func TestMergeAsyncResultsDedup(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	// Set up existing items (simulating sync results)
	engine.active = true
	engine.activeTrigger = TriggerAt
	engine.items = []CompletionItem{
		{Label: "main.go", InsertText: "@main.go ", FilePath: "/proj/main.go", Score: 0.9},
		{Label: "router.go", InsertText: "@router.go ", FilePath: "/proj/router.go", Score: 0.8},
	}
	engine.loading = true

	// Merge async results (one duplicate, one new)
	engine.MergeAsyncResults([]CompletionItem{
		{Label: "main.go", InsertText: "@main.go ", FilePath: "/proj/main.go", Score: 0.7},  // duplicate
		{Label: "agent.go", InsertText: "@agent.go ", FilePath: "/proj/agent.go", Score: 0.85}, // new
	})

	if engine.loading {
		t.Error("Expected loading=false after merge")
	}
	if len(engine.items) != 3 {
		t.Errorf("Expected 3 items (2 original + 1 new), got %d", len(engine.items))
	}

	// Should be sorted by score descending
	for i := 1; i < len(engine.items); i++ {
		if engine.items[i].Score > engine.items[i-1].Score {
			t.Errorf("Items not sorted by score: items[%d].Score=%f > items[%d].Score=%f",
				i, engine.items[i].Score, i-1, engine.items[i-1].Score)
		}
	}
}

func TestMergeAsyncResultsCap(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())
	engine.active = true
	engine.activeTrigger = TriggerAt

	// Start with 10 items
	for i := 0; i < 10; i++ {
		engine.items = append(engine.items, CompletionItem{
			Label:      "file" + itoa(i) + ".go",
			InsertText: "@file" + itoa(i) + ".go ",
			FilePath:   "/proj/file" + itoa(i) + ".go",
			Score:      float64(10-i) / 10.0,
		})
	}

	// Merge 5 more — should cap at 12
	var async []CompletionItem
	for i := 10; i < 15; i++ {
		async = append(async, CompletionItem{
			Label:      "file" + itoa(i) + ".go",
			InsertText: "@file" + itoa(i) + ".go ",
			FilePath:   "/proj/file" + itoa(i) + ".go",
			Score:      0.5,
		})
	}
	engine.MergeAsyncResults(async)

	if len(engine.items) > 12 {
		t.Errorf("Expected max 12 items, got %d", len(engine.items))
	}
}

func TestMergeAsyncResultsActivatesDropdown(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())
	engine.active = false
	engine.activeTrigger = TriggerAt

	// Even if dropdown was dismissed, async results reactivate it
	engine.MergeAsyncResults([]CompletionItem{
		{Label: "found.go", InsertText: "@found.go ", FilePath: "/proj/found.go", Score: 0.9},
	})

	if !engine.active {
		t.Error("Expected active=true after merge with results")
	}
}

func TestSetLoading(t *testing.T) {
	engine := NewCompletionEngine(NewSlashCommandProvider())

	engine.SetLoading(true)
	if !engine.IsLoading() {
		t.Error("Expected loading=true")
	}

	engine.SetLoading(false)
	if engine.IsLoading() {
		t.Error("Expected loading=false")
	}
}

func TestVecgrepDebounceIDIncrement(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))

	initial := *m.vecgrepDebounceID
	*m.vecgrepDebounceID++
	if *m.vecgrepDebounceID != initial+1 {
		t.Error("Expected debounce ID to increment")
	}
}

func TestCheckVecgrepAvailable(t *testing.T) {
	// Just verify it doesn't panic — actual availability depends on system
	_ = checkVecgrepAvailable()
}

func TestSearchVecgrepShortQuery(t *testing.T) {
	// Short queries should return nil immediately
	items := SearchVecgrep(context.TODO(), "x", "", "", 5)
	if items != nil {
		t.Error("Expected nil for short query")
	}
}

func TestModelSetProjectRoot(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))
	m.SetProjectRoot("/some/path")
	if m.projectRoot != "/some/path" {
		t.Errorf("Expected /some/path, got %q", m.projectRoot)
	}
}

// --- Phase 5: Smart Context Suggestions ---

func TestRenderSuggestHint(t *testing.T) {
	tests := []struct {
		name      string
		files     []SuggestedFile
		maxWidth  int
		wantEmpty bool
	}{
		{"empty", nil, 80, true},
		{"too narrow", []SuggestedFile{{RelPath: "foo.go"}}, 15, true},
		{
			"single file",
			[]SuggestedFile{{RelPath: "internal/agent/router.go", Language: "go"}},
			80, false,
		},
		{
			"multiple files",
			[]SuggestedFile{
				{RelPath: "router.go", Language: "go"},
				{RelPath: "pipeline.go", Language: "go"},
			},
			80, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderSuggestHint(tt.files, tt.maxWidth)
			if tt.wantEmpty && result != "" {
				t.Errorf("Expected empty, got %q", result)
			}
			if !tt.wantEmpty && result == "" {
				t.Error("Expected non-empty hint")
			}
		})
	}
}

func TestRenderSuggestHintContainsFileNames(t *testing.T) {
	files := []SuggestedFile{
		{RelPath: "internal/agent/router.go"},
		{RelPath: "cmd/main.go"},
	}
	result := renderSuggestHint(files, 120)
	if result == "" {
		t.Fatal("Expected non-empty hint")
	}
	// shortName should extract base name
	if !strings.Contains(result, "router.go") {
		t.Errorf("Expected hint to contain 'router.go', got %q", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("Expected hint to contain 'main.go', got %q", result)
	}
}

func TestModelSuggestedFiles(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))

	// Initially empty
	if files := m.GetSuggestedFiles(); len(files) != 0 {
		t.Errorf("Expected empty suggestions, got %d", len(files))
	}

	// Set suggestions
	suggest := []SuggestedFile{
		{RelPath: "foo.go", Language: "go", Score: 0.9},
		{RelPath: "bar.py", Language: "python", Score: 0.8},
	}
	m.SetSuggestedFiles(suggest, "test query")

	got := m.GetSuggestedFiles()
	if len(got) != 2 {
		t.Fatalf("Expected 2 suggestions, got %d", len(got))
	}
	if got[0].RelPath != "foo.go" {
		t.Errorf("Expected foo.go, got %s", got[0].RelPath)
	}
	if m.suggestQuery != "test query" {
		t.Errorf("Expected query 'test query', got %q", m.suggestQuery)
	}

	// Clear
	m.ClearSuggestions()
	if files := m.GetSuggestedFiles(); len(files) != 0 {
		t.Errorf("Expected empty after clear, got %d", len(files))
	}
}

func TestSuggestDebounceIDIncrement(t *testing.T) {
	m := NewModel("test", make(chan StreamMsg, 1))

	initial := *m.suggestDebounceID
	*m.suggestDebounceID++
	if *m.suggestDebounceID != initial+1 {
		t.Errorf("Expected debounce ID to increment")
	}

	// Incrementing again should work (pointer shared across model copies)
	m2 := m // Bubbletea copies the struct
	*m2.suggestDebounceID++
	if *m.suggestDebounceID != initial+2 {
		t.Errorf("Expected pointer-shared increment, got %d", *m.suggestDebounceID)
	}
}

func TestSuggestRelPathConstruction(t *testing.T) {
	// Simulates what the SuggestDebounceMsg handler does
	tests := []struct {
		label  string
		detail string
		want   string
	}{
		{"main.go", "", "main.go"},
		{"router.go", "internal/agent", "internal/agent/router.go"},
		{"app.go", "internal/tui", "internal/tui/app.go"},
	}
	for _, tt := range tests {
		item := CompletionItem{Label: tt.label, Detail: tt.detail}
		relPath := item.Label
		if item.Detail != "" {
			relPath = item.Detail + "/" + item.Label
		}
		if relPath != tt.want {
			t.Errorf("label=%q detail=%q: got %q, want %q", tt.label, tt.detail, relPath, tt.want)
		}
	}
}

func TestSearchVecgrepWithContext(t *testing.T) {
	// Verify that passing queryContext doesn't panic
	items := SearchVecgrep(context.TODO(), "x", "some context about routing", "", 5)
	// Short query "x" should still return nil even with context
	if items != nil {
		t.Error("Expected nil for short query even with context")
	}
}

func TestSearchVecgrepContextThreshold(t *testing.T) {
	// If query is short but context is long enough (>=5), search should proceed
	// (though it will fail since vecgrep likely isn't installed in CI)
	// We're testing the threshold logic, not actual results
	items := SearchVecgrep(context.TODO(), "a", "hello world context", "", 5)
	// Can't assert non-nil since vecgrep may not be installed, but no panic
	_ = items
}
