package arch_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wolfram0108/pier/internal/event"
	"github.com/wolfram0108/pier/internal/fail"
	"github.com/wolfram0108/pier/internal/settings"
)

// Каталоги — те места, где расширение стоит одной строки. Проверки ниже
// стерегут ровно это свойство: пополнить каталог наполовину не выйдет.

// У каждого кода ошибки есть текст, и текст этот годен к показу человеку:
// назван по-русски, кончается точкой, не пуст (Ф-8.2, Т-1).
func TestКаждаяОшибкаИмеетРусскийТекст(t *testing.T) {
	for _, code := range fail.Codes() {
		e := fail.New(code)
		if e.Kind != code {
			t.Errorf("код %q: New вернул чужой код %q — записи нет в каталоге", code, e.Kind)
			continue
		}
		msg := e.Message
		if strings.TrimSpace(msg) == "" {
			t.Errorf("код %q: пустой текст", code)
			continue
		}
		if !strings.ContainsFunc(msg, isCyrillic) {
			t.Errorf("код %q: текст не на русском: %q", code, msg)
		}
		if e.Status() < 400 || e.Status() >= 600 {
			t.Errorf("код %q: ответ HTTP %d — ошибка обязана отвечать кодом ошибки", code, e.Status())
		}
	}
}

// Каждое поле настроек описано в каталоге: иначе значение появится в API, но
// без проверки и без объяснения, что оно значит (§ 7.6).
func TestКаждаяНастройкаОписанаВКаталоге(t *testing.T) {
	described := map[string]bool{}
	for _, d := range settings.Describe() {
		if strings.TrimSpace(d.About) == "" {
			t.Errorf("настройка %q описана пустой строкой", d.Name)
		}
		described[d.Name] = true
	}

	typ := reflect.TypeFor[settings.Settings]()
	for i := range typ.NumField() {
		tag := typ.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		switch name {
		case "", "-", "version":
			continue // version не настройка, а признак изменения (Ф-8.8)
		}
		if !described[name] {
			t.Errorf("поле %q есть в Settings, но не описано в каталоге: "+
				"значит, оно не проверяется при сохранении", name)
		}
	}
}

// Умолчания § 7.6 обязаны проходить собственную проверку. Иначе панель при
// первом запуске стартует с набором, который сама же считает недопустимым.
func TestУмолчанияПроходятПроверку(t *testing.T) {
	for _, mem := range []int{128, 256} {
		s := settings.Defaults(mem)
		if err := s.Validate(0); err != nil {
			t.Errorf("умолчания для %d МБ не проходят проверку: %v", mem, err)
		}
	}

	// Н-2.8 и Н-2.9: предел нод и квота тяжёлых выводятся из баланса памяти,
	// а не назначаются по вкусу — на 256 МБ они обязаны быть больше.
	small, big := settings.Defaults(128), settings.Defaults(256)
	if big.CoreNodeLimit <= small.CoreNodeLimit {
		t.Errorf("предел нод не растёт с памятью: %d при 128 МБ, %d при 256 МБ",
			small.CoreNodeLimit, big.CoreNodeLimit)
	}
	if big.HeavyQuota <= small.HeavyQuota {
		t.Errorf("квота тяжёлых нод не растёт с памятью: %d при 128 МБ, %d при 256 МБ",
			small.HeavyQuota, big.HeavyQuota)
	}
}

// У каждого вида события объявлен состав полей (§ 8.3): по нему проверяется,
// что событие ушло полным, а не с половиной полей.
func TestКаждоеСобытиеИмеетСоставПолей(t *testing.T) {
	for _, k := range event.Kinds() {
		fields, ok := event.Fields(k)
		if !ok {
			t.Errorf("вид %q не описан в каталоге", k)
			continue
		}
		if k == event.Resync {
			continue // resync полей не несёт: это команда перечитать состояние
		}
		if len(fields) == 0 {
			t.Errorf("вид %q объявлен без полей: событие без данных обновить "+
				"показанное не может", k)
		}
	}
}

// Карта кода — не украшение: пакеты и карта обязаны соответствовать друг другу.
func TestКартаКодаОписываетВсеПакеты(t *testing.T) {
	root := internalRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("не прочитан каталог internal: %v", err)
	}

	карта, err := os.ReadFile(filepath.Join(filepath.Dir(root), "doc", "КАРТА.md"))
	if err != nil {
		t.Fatalf("не прочитана карта кода: %v", err)
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "arch" {
			continue
		}
		if !strings.Contains(string(карта), e.Name()+"/") {
			t.Errorf("пакет %s не описан в doc/КАРТА.md", e.Name())
		}
		if _, ok := level[e.Name()]; !ok {
			t.Errorf("пакет %s не назван в карте уровней arch_test.go", e.Name())
		}
	}
}

func isCyrillic(r rune) bool { return r >= 'А' && r <= 'я' }
