package lexer

import "testing"

func TestLexer(t *testing.T) {

	source := `
		constraint life_handler {
			work hard, stay with family
		}

		prompt greet {
			Hello, World!

			[match user.language]{
				[case "en"] {
					Hello, World!
				}
				[case "es"] {
					¡Hola, Mundo!
				}
				[case "fr"] {
					Bonjour, le monde!
				}
				[case "zh"] {
					你好，世界！
				}
				[case _] {
					Hello, World!
				}
			}
		}


		inject greet

		plan my_hero_plan {
			target "src/hero.c"
			target "src/life.c"

			spec {
				[inject greet]
				Answer the question about the meaning of life in a concise way.
			}

			prompt handler {
				see good film!
			}

			impl "int main()" {
				[target "life.c"]
				[inject handler]
				[use life]
			}
		}

		inject my_hero_plan

	`

	l := New(source)

	// Collect all tokens
	var tokens []TokenInfo
	for {
		token := l.NextToken()
		tokens = append(tokens, token)
		if token.Type == EOF || token.Type == ILLEGAL {
			break
		}
	}

	// Define expected token sequence
	expected := []struct {
		typ Token
		val string
	}{
		// constraint life_handler { ... }
		{CONSTRAINT, "constraint"},
		{IDENT, "life_handler"},
		{LBRACE, "{"},
		{TEXT, "work hard, stay with family"},
		{RBRACE, "}"},

		// prompt greet { ... [match]...[/match] }
		{PROMPT, "prompt"},
		{IDENT, "greet"},
		{LBRACE, "{"},
		{TEXT, "Hello, World!"},
		{MATCH_REF, "user.language"},
		{LBRACE, "{"},
		{CASE_REF, "en"},
		{LBRACE, "{"},
		{TEXT, "Hello, World!"},
		{RBRACE, "}"},
		{CASE_REF, "es"},
		{LBRACE, "{"},
		{TEXT, "¡Hola, Mundo!"},
		{RBRACE, "}"},
		{CASE_REF, "fr"},
		{LBRACE, "{"},
		{TEXT, "Bonjour, le monde!"},
		{RBRACE, "}"},
		{CASE_REF, "zh"},
		{LBRACE, "{"},
		{TEXT, "你好，世界！"},
		{RBRACE, "}"},
		{CASE_REF, "_"},
		{LBRACE, "{"},
		{TEXT, "Hello, World!"},
		{RBRACE, "}"},
		{RBRACE, "}"},  // close match
		{RBRACE, "}"},  // close prompt

		// inject greet
		{INJECT, "inject"},
		{IDENT, "greet"},

		// plan my_hero_plan { ... }
		{PLAN, "plan"},
		{IDENT, "my_hero_plan"},
		{LBRACE, "{"},

		// target "src/hero.c"
		{TARGET, "target"},
		{STRING, `"src/hero.c"`},

		// target "src/life.c"
		{TARGET, "target"},
		{STRING, `"src/life.c"`},

		// spec { ... }
		{SPEC, "spec"},
		{LBRACE, "{"},
		{INJECT_REF, "greet"},
		{TEXT, "Answer the question about the meaning of life in a concise way."},
		{RBRACE, "}"},

		// prompt handler { ... }
		{PROMPT, "prompt"},
		{IDENT, "handler"},
		{LBRACE, "{"},
		{TEXT, "see good film!"},
		{RBRACE, "}"},

		// impl "int main()" { ... }
		{IMPL, "impl"},
		{STRING, `"int main()"`},
		{LBRACE, "{"},
		{TARGET_REF, "life.c"},
		{INJECT_REF, "handler"},
		{USE_REF, "life"},
		{RBRACE, "}"},

		// close plan
		{RBRACE, "}"},

		// inject my_hero_plan
		{INJECT, "inject"},
		{IDENT, "my_hero_plan"},

		{EOF, ""},
	}

	for i, exp := range expected {
		if i >= len(tokens) {
			t.Fatalf("token %d: expected %s %q, but ran out of tokens", i, exp.typ, exp.val)
		}
		got := tokens[i]
		if got.Type != exp.typ {
			t.Errorf("token %d: type = %s, want %s (val=%q)", i, got.Type, exp.typ, got.Val)
		}
		if exp.val != "" && got.Val != exp.val {
			t.Errorf("token %d: val = %q, want %q (type=%s)", i, got.Val, exp.val, got.Type)
		}
	}

	if len(tokens) != len(expected) {
		t.Errorf("got %d tokens, expected %d", len(tokens), len(expected))
		for i, item := range tokens {
			t.Logf("  [%d] %s %q", i, item.Type, item.Val)
		}
	}
}
