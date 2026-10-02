package subscription

import (
	"errors"
	"strings"
	"testing"
)

func TestCompileMemberFilter_AllowsMatrix(t *testing.T) {
	cases := []struct {
		name    string
		include string
		exclude string
		label   string
		want    bool
	}{
		{"пустой фильтр пропускает всё", "", "", "🇷🇺 Moscow-1", true},
		{"пустой фильтр пропускает пустое имя", "", "", "", true},
		{"include only: совпало", "DE|NL", "", "NL-Amsterdam", true},
		{"include only: не совпало", "DE|NL", "", "US-Dallas", false},
		{"include only: пустое имя не совпадает", "DE", "", "", false},
		{"exclude only: совпало → скрыт", "", "RU", "RU-Moscow", false},
		{"exclude only: не совпало → пропущен", "", "RU", "DE-Berlin", true},
		{"exclude only: пустое имя пропускается", "", "RU", "", true},
		{"оба: include совпал, exclude нет", "Europe", "RU", "Europe DE-1", true},
		{"оба: include совпал, но exclude тоже", "Europe", "RU", "Europe RU-1", false},
		{"оба: include не совпал", "Europe", "RU", "Asia JP-1", false},
		{"(?i) регистронезависимость include", "(?i)de", "", "DE-Frankfurt", true},
		{"(?i) регистронезависимость exclude", "", "(?i)(ru|russia)", "Best Russia Server", false},
		{"кириллица", "", "Россия|Москва", "🚀 Москва-1", false},
		{"кириллица: не совпало", "", "Россия", "Германия-1", true},
		{"эмодзи-флаг в exclude", "", "🇷🇺", "🇷🇺 SPB", false},
		{"эмодзи-флаг: другой флаг проходит", "", "🇷🇺", "🇩🇪 Berlin", true},
		{"пример из issue (эквивалент mihomo)", "", "(?i)(🇷🇺|Россия|RU|BRIDGE|LTE)", "🇩🇪 Frankfurt", true},
		{"пример из issue: BRIDGE скрыт", "", "(?i)(🇷🇺|Россия|RU|BRIDGE|LTE)", "bridge-node-1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := CompileMemberFilter(tc.include, tc.exclude)
			if err != nil {
				t.Fatalf("CompileMemberFilter(%q, %q): %v", tc.include, tc.exclude, err)
			}
			if got := f.Allows(tc.label); got != tc.want {
				t.Errorf("Allows(%q) = %v, want %v (include=%q exclude=%q)", tc.label, got, tc.want, tc.include, tc.exclude)
			}
		})
	}
}

func TestCompileMemberFilter_NilFilterAllowsAll(t *testing.T) {
	var f *MemberFilter
	if !f.Allows("anything") {
		t.Error("nil *MemberFilter must allow everything")
	}
	if !f.IsEmpty() {
		t.Error("nil *MemberFilter must be empty")
	}
}

func TestCompileMemberFilter_LookaroundDetected(t *testing.T) {
	// Пример из issue: mihomo-стиль negative lookahead, RE2 такое не умеет.
	cases := []struct {
		name    string
		include string
		exclude string
	}{
		{"lookahead в include (пример из issue)", `(?i)^(?!.*(RU|Russia)).*$`, ""},
		{"lookahead в exclude", "", `(?!RU)`},
		{"positive lookahead", `(?=DE)`, ""},
		{"lookbehind", "", `(?<=DE)`},
		{"negative lookbehind", `(?<!RU)`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileMemberFilter(tc.include, tc.exclude)
			if err == nil {
				t.Fatal("expected lookaround error, got nil")
			}
			var fe *FilterError
			if !errors.As(err, &fe) {
				t.Fatalf("expected *FilterError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), "lookahead/lookbehind") {
				t.Errorf("error must mention lookahead/lookbehind, got: %v", err)
			}
			if !strings.Contains(err.Error(), "«включить» + «исключить»") {
				t.Errorf("error must suggest the include+exclude pair, got: %v", err)
			}
		})
	}
}

func TestCompileMemberFilter_InvalidRegex(t *testing.T) {
	cases := []struct {
		name    string
		include string
		exclude string
	}{
		{"незакрытая скобка в include", "(DE", ""},
		{"незакрытая скобка в exclude", "", "[RU"},
		{"битый квантификатор", "*DE", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileMemberFilter(tc.include, tc.exclude)
			if err == nil {
				t.Fatal("expected compile error, got nil")
			}
			var fe *FilterError
			if !errors.As(err, &fe) {
				t.Fatalf("expected *FilterError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), "некорректное регулярное выражение") {
				t.Errorf("error must be user-facing Russian, got: %v", err)
			}
		})
	}
}
