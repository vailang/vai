package locker

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "helloworld"},
		{"  spaces  tabs\t", "spacestabs"},
		{"Hello, World!", "helloworld"},
		{"Build a TODO app.", "buildatodoapp"},
		{"UPPER lower MiXeD", "upperlowermixed"},
		{"no-change", "nochange"},
		{"line\nbreak\n", "linebreak"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalize(tt.input)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeStability(t *testing.T) {
	// Minor formatting differences should produce the same result.
	variants := []string{
		"Build a todo list application",
		"  Build  a  todo  list  application  ",
		"Build a todo list application.",
		"build a todo list application",
		"BUILD A TODO LIST APPLICATION",
	}
	first := normalize(variants[0])
	for _, v := range variants[1:] {
		got := normalize(v)
		if got != first {
			t.Errorf("normalize(%q) = %q, want %q (same as first variant)", v, got, first)
		}
	}
}

func TestHashPlan(t *testing.T) {
	h1 := HashPlan("myplan", []string{"main.rs"}, []string{"Build a todo app"}, nil, nil)
	h2 := HashPlan("myplan", []string{"main.rs"}, []string{"Build a todo app"}, nil, nil)
	if h1 != h2 {
		t.Error("identical inputs should produce identical hashes")
	}

	// Different spec text should produce different hash.
	h3 := HashPlan("myplan", []string{"main.rs"}, []string{"Build a chat app"}, nil, nil)
	if h1 == h3 {
		t.Error("different spec text should produce different hashes")
	}

	// Different plan name should produce different hash.
	h4 := HashPlan("other", []string{"main.rs"}, []string{"Build a todo app"}, nil, nil)
	if h1 == h4 {
		t.Error("different plan name should produce different hashes")
	}
}

func TestHashPlanWithImpls(t *testing.T) {
	impls := []ImplEntry{
		{Name: "add", BodyText: "add a new todo item"},
		{Name: "remove", BodyText: "remove a todo item by id"},
	}
	h1 := HashPlan("plan", []string{"main.rs"}, []string{"spec"}, nil, impls)

	// Reorder impls — should produce different hash (order matters).
	impls2 := []ImplEntry{
		{Name: "remove", BodyText: "remove a todo item by id"},
		{Name: "add", BodyText: "add a new todo item"},
	}
	h2 := HashPlan("plan", []string{"main.rs"}, []string{"spec"}, nil, impls2)
	if h1 == h2 {
		t.Error("different impl order should produce different hashes")
	}
}

func TestHashImpl(t *testing.T) {
	h1 := HashImpl("add", "implement function to add a new todo", "src/main.rs")
	h2 := HashImpl("add", "implement function to add a new todo", "src/main.rs")
	if h1 != h2 {
		t.Error("identical inputs should produce identical hashes")
	}

	// Minor formatting difference should produce same hash.
	h3 := HashImpl("add", "  Implement function to add a new todo. ", "src/main.rs")
	if h1 != h3 {
		t.Errorf("minor formatting differences should produce same hash: %s vs %s", h1, h3)
	}

	// Different body text should produce different hash.
	h4 := HashImpl("add", "implement delete function", "src/main.rs")
	if h1 == h4 {
		t.Error("different body text should produce different hashes")
	}
}

func TestComputeHashSeparation(t *testing.T) {
	// Ensure that "ab" + "cd" != "a" + "bcd" (null-byte separator prevents this).
	h1 := computeHash("ab", "cd")
	h2 := computeHash("a", "bcd")
	if h1 == h2 {
		t.Error("different part boundaries should produce different hashes")
	}
}
