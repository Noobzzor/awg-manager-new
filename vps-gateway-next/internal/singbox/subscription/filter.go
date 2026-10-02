package subscription

import (
	"fmt"
	"regexp"
	"strings"
)

// MemberFilter — скомпилированная пара regex-фильтров подписки в стиле
// mihomo `filter:` / `exclude-filter:`. Матчинг идёт по человекочитаемому
// имени сервера (MemberInfo.Label / ParsedOutbound.Label). Пустое поле =
// нет ограничения. nil-фильтр (*MemberFilter)(nil) пропускает всё.
type MemberFilter struct {
	include *regexp.Regexp // nil = включать всё
	exclude *regexp.Regexp // nil = ничего не исключать
}

// lookaroundTokens — конструкции PCRE, которые Go (RE2) не поддерживает.
// Детектируем их ДО компиляции, чтобы вместо невнятного
// "invalid or unsupported Perl syntax" выдать пользователю подсказку
// про пару фильтров «включить» + «исключить».
var lookaroundTokens = []string{"(?=", "(?!", "(?<=", "(?<!"}

// containsLookaround сообщает, встречается ли в шаблоне lookahead/lookbehind.
func containsLookaround(pattern string) bool {
	for _, tok := range lookaroundTokens {
		if strings.Contains(pattern, tok) {
			return true
		}
	}
	return false
}

// FilterError — user-facing ошибка валидации regex-фильтра. Отдельный тип,
// чтобы HTTP-обработчики через errors.As отвечали 400 (ошибка пользователя),
// а не 500.
type FilterError struct{ Msg string }

func (e *FilterError) Error() string { return e.Msg }

// errLookaround — единое user-facing сообщение про lookahead/lookbehind.
func errLookaround(field string) error {
	return &FilterError{Msg: fmt.Sprintf("%s: регулярные выражения Go (RE2) не поддерживают lookahead/lookbehind; используйте пару фильтров: «включить» + «исключить»", field)}
}

// CompileMemberFilter валидирует и компилирует пару фильтров подписки.
// Ошибки — user-facing (русский текст), пригодны для 400 в API.
func CompileMemberFilter(include, exclude string) (*MemberFilter, error) {
	f := &MemberFilter{}
	if include != "" {
		if containsLookaround(include) {
			return nil, errLookaround("фильтр «включать только»")
		}
		re, err := regexp.Compile(include)
		if err != nil {
			return nil, &FilterError{Msg: fmt.Sprintf("фильтр «включать только»: некорректное регулярное выражение: %v", err)}
		}
		f.include = re
	}
	if exclude != "" {
		if containsLookaround(exclude) {
			return nil, errLookaround("фильтр «исключать»")
		}
		re, err := regexp.Compile(exclude)
		if err != nil {
			return nil, &FilterError{Msg: fmt.Sprintf("фильтр «исключать»: некорректное регулярное выражение: %v", err)}
		}
		f.exclude = re
	}
	return f, nil
}

// Allows сообщает, проходит ли сервер с данным именем через фильтр:
// include пуст → проходит, иначе имя обязано совпасть; exclude пуст →
// проходит, иначе имя НЕ должно совпасть. Оба условия применяются вместе.
func (f *MemberFilter) Allows(label string) bool {
	if f == nil {
		return true
	}
	if f.include != nil && !f.include.MatchString(label) {
		return false
	}
	if f.exclude != nil && f.exclude.MatchString(label) {
		return false
	}
	return true
}

// IsEmpty сообщает, что фильтр ничего не ограничивает (оба поля пустые).
func (f *MemberFilter) IsEmpty() bool {
	return f == nil || (f.include == nil && f.exclude == nil)
}
