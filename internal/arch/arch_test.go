// Пакет arch — проверки устройства проекта.
//
// Правила раскладки (doc/КАРТА.md) обычно живут в документе и нарушаются
// тихо: кто-то импортирует пакет уровнем выше, и через полгода слои
// перепутаны. Здесь они проверяются кодом и падают сборкой.
package arch_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Уровни пакетов. Импортировать разрешено только вниз: пакет уровня N видит
// уровни от 0 до N-1 и никогда не смотрит вверх или вбок на своём уровне,
// если это не оговорено особо.
var level = map[string]int{
	"clock": 0,
	"fail":  0,

	"model": 1,

	"event":    2,
	"settings": 2,
	"parse":    2,

	"flash":   3,
	"confgen": 3,

	"core":    4,
	"sysbind": 4,
	"probe":   4,

	"state": 5,

	"api": 6,

	"webui": 7,
}

// Запреты сверх уровней — прямо из ТЗ.
var forbidden = map[string][]string{
	// § 4.2: движок проверки не знает ни о панели, ни о ядре напрямую.
	// Всё, что ему нужно, он объявляет своими интерфейсами.
	"probe": {"core", "state", "api"},
	// API — дверь наружу, а не библиотека для своих.
	"": {"api"},
	// Интерфейс — клиент собственного API и только: своей логики не имеет.
	"webui": {"state", "core", "probe", "flash", "api", "model"},
}

const modulePath = "github.com/wolfram0108/pier/internal/"

func TestИмпортыИдутТолькоВниз(t *testing.T) {
	root := internalRoot(t)

	for pkg, lvl := range level {
		dir := filepath.Join(root, pkg)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // пакет ещё не написан — это не нарушение
		}
		for _, imp := range imports(t, dir) {
			dep, ok := strings.CutPrefix(imp, modulePath)
			if !ok {
				continue // внешний или стандартный пакет
			}
			dep, _, _ = strings.Cut(dep, "/")

			depLvl, known := level[dep]
			if !known {
				t.Errorf("%s импортирует %s, которого нет в карте уровней "+
					"(doc/КАРТА.md): либо опечатка, либо карту не обновили", pkg, dep)
				continue
			}
			if depLvl >= lvl {
				t.Errorf("%s (уровень %d) импортирует %s (уровень %d): "+
					"импорт вверх или вбок запрещён", pkg, lvl, dep, depLvl)
			}
		}
	}
}

func TestЗапретыИзТЗСоблюдены(t *testing.T) {
	root := internalRoot(t)

	for pkg, banned := range forbidden {
		if pkg == "" {
			continue // общий запрет проверяется ниже, по всем пакетам
		}
		dir := filepath.Join(root, pkg)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		for _, imp := range imports(t, dir) {
			dep, ok := strings.CutPrefix(imp, modulePath)
			if !ok {
				continue
			}
			dep, _, _ = strings.Cut(dep, "/")
			for _, b := range banned {
				if dep == b {
					t.Errorf("%s импортирует %s — запрещено правилом ТЗ "+
						"(см. doc/КАРТА.md, «два запрета сверх уровней»)", pkg, dep)
				}
			}
		}
	}

	// Никто, кроме cmd/, не импортирует api.
	for pkg := range level {
		if pkg == "api" {
			continue
		}
		dir := filepath.Join(root, pkg)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		for _, imp := range imports(t, dir) {
			if imp == modulePath+"api" {
				t.Errorf("%s импортирует api: наружная дверь не библиотека для своих", pkg)
			}
		}
	}
}

// Н-5.6: время в панели монотонное. Единственный, кому позволено спрашивать
// календарь у системы, — пакет clock; остальные берут время у него.
func TestВремяТолькоЧерезПакетClock(t *testing.T) {
	root := internalRoot(t)

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("не прочитан каталог internal: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "clock" || e.Name() == "arch" {
			continue
		}
		filepath.WalkDir(filepath.Join(root, e.Name()), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, banned := range []string{"time.Now()", "time.Since(", "time.Until("} {
				if strings.Contains(string(body), banned) {
					rel, _ := filepath.Rel(root, path)
					t.Errorf("%s: вызов %s. На роутере нет часов реального времени (Н-5.6): "+
						"время берётся у пакета clock, а не у системы", rel, banned)
				}
			}
			return nil
		})
	}
}

// imports собирает импорты всех файлов пакета, включая тестовые: тест,
// заглядывающий вверх, тянет туда же и сам пакет.
func imports(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			out = append(out, strings.Trim(imp.Path.Value, `"`))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("не разобран пакет %s: %v", dir, err)
	}
	return out
}

func internalRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("не определён рабочий каталог: %v", err)
	}
	return filepath.Dir(wd) // .../internal/arch → .../internal
}
