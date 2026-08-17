package settings

import (
	"fmt"

	"github.com/wolfram0108/pier/internal/fail"
)

// Каталог настроек — таблица § 7.6 ТЗ целиком, одна запись на настройку.
//
// В записи всё, что о настройке нужно знать: смысл человеческими словами,
// умолчание для двух типовых роутеров, пределы и проверка. Отсюда же берутся
// умолчания при первом запуске и проверка при сохранении — второго списка,
// который мог бы разойтись с этим, в программе нет.
//
// Почему умолчаний два. Предел нод и квота тяжёлых выводятся из баланса памяти
// (Н-2.8, Н-2.9): на 128 МБ роутера рабочий запас другой, чем на 256 МБ.
// Значение выбирается по факту обнаруженной памяти при первом запуске.

type field struct {
	name  string // имя поля в JSON — оно же в API и в интерфейсе
	about string // смысл: короткая фраза, как в § 7.6
	check func(*Settings) error
}

// limits — общий вид проверки числового поля.
func limits(name string, get func(*Settings) int, min, max int, unit string) func(*Settings) error {
	return func(s *Settings) error {
		v := get(s)
		if v < min || v > max {
			return fail.New(fail.BadRequest,
				fmt.Sprintf("%s: допустимо от %d до %d %s, получено %d", name, min, max, unit, v))
		}
		return nil
	}
}

var fields = []field{
	// --- пределы и поведение канала ---------------------------------------
	{"core_node_limit", "предел нод в ядре, общий бюджет на все группы",
		limits("предел нод в ядре", func(s *Settings) int { return s.CoreNodeLimit }, 1, 1000, "нод")},

	{"no_working_node", "при отсутствии рабочих нод: сохранить интернет либо не выпускать",
		func(s *Settings) error {
			switch s.NoWorkingNode {
			case KeepInternet, Block:
				return nil
			}
			return fail.New(fail.BadRequest, "поведение при отсутствии рабочих нод: "+
				"допустимо keep_internet или block")
		}},

	{"switch_policy", "политика смены активной ноды: мягкая, с ожиданием, немедленная",
		func(s *Settings) error {
			switch s.SwitchPolicy {
			case Soft, Wait, Immediate:
				return nil
			}
			return fail.New(fail.BadRequest, "политика смены активной ноды: "+
				"допустимо soft, wait или immediate")
		}},

	{"switch_wait", "срок ожидания при смене активной ноды",
		limits("срок ожидания при смене", func(s *Settings) int { return s.SwitchWait }, 1, 3600, "с")},

	// --- проверка нод ------------------------------------------------------
	{"probe_concurrency", "сколько замеров выполняется параллельно внутри одной порции",
		limits("одновременность замеров", func(s *Settings) int { return s.ProbeConcurrency }, 1, 128, "")},

	{"probe_timeout", "сколько ждать ответа ноды, прежде чем счесть замер неудачным",
		limits("время ожидания замера", func(s *Settings) int { return s.ProbeTimeout }, 1, 60, "с")},

	{"core_poll_interval", "как часто панель спрашивает у ядра, отвечает ли оно",
		limits("частота проверки ядра", func(s *Settings) int { return s.CorePollInterval }, 1, 60, "с")},

	{"composition_interval", "как часто перепроверяются ноды, через которые идёт трафик",
		limits("интервал опроса состава", func(s *Settings) int { return s.CompositionInterval }, 10, 3600, "с")},

	{"reserve_interval", "как часто перепроверяются ноды запаса",
		limits("интервал опроса запаса", func(s *Settings) int { return s.ReserveInterval }, 10, 3600, "с")},

	{"fresh_candidates", "срок годности результата: поиск кандидатов и сканирование по команде",
		limits("срок годности результата (поиск и сканирование)", func(s *Settings) int { return s.FreshCandidates }, 10, 3600, "с")},

	{"fresh_background", "срок годности результата: набор запаса",
		limits("срок годности результата (набор запаса)", func(s *Settings) int { return s.FreshBackground }, 10, 86400, "с")},

	{"background_budget", "бюджет фонового набора, замеров в минуту",
		limits("бюджет фонового набора", func(s *Settings) int { return s.BackgroundBudget }, 1, 600, "замеров в минуту")},

	{"batch_candidates", "размер порции: поиск кандидатов",
		limits("размер порции (поиск кандидатов)", func(s *Settings) int { return s.BatchCandidates }, 1, 200, "нод")},
	{"batch_reserve", "размер порции: опрос запаса",
		limits("размер порции (опрос запаса)", func(s *Settings) int { return s.BatchReserve }, 1, 200, "нод")},
	{"batch_background", "размер порции: набор запаса",
		limits("размер порции (набор запаса)", func(s *Settings) int { return s.BatchBackground }, 1, 200, "нод")},
	{"batch_scan", "размер порции: сканирование по команде, очередь одна на все источники",
		limits("размер порции (сканирование)", func(s *Settings) int { return s.BatchScan }, 1, 500, "нод")},

	{"heavy_quota", "квота на одновременно поднятые тяжёлые ноды, суммарно по всем процессам",
		limits("квота тяжёлых нод", func(s *Settings) int { return s.HeavyQuota }, 1, 200, "нод")},

	{"reserve_size", "потолок числа живых нод сверх состава, на каждую группу",
		limits("размер запаса", func(s *Settings) int { return s.ReserveSize }, 0, 100, "нод")},

	// --- источники и хранение ---------------------------------------------
	// Тумблеры проверять нечего: у булева значения нет недопустимых состояний.
	// Запись всё равно обязательна — она объясняет, что значит переключатель,
	// и не даёт настройке появиться в API без объяснения.
	{"refresh_auto", "обновлять подписки автоматически или только вручную", nil},
	{"persist_registry", "хранить ли на флеше статистику по всем проверенным нодам; " +
		"по умолчанию выключено: за неё платит флеш, а восстанавливается она сама", nil},

	{"refresh_interval", "интервал обновления подписок",
		limits("интервал обновления подписок", func(s *Settings) int { return s.RefreshInterval }, 300, 604800, "с")},

	{"persist_interval", "как часто накопленное уходит на флеш, если менялось",
		limits("интервал сохранения состава и запаса", func(s *Settings) int { return s.PersistInterval }, 60, 86400, "с")},

	// --- порты -------------------------------------------------------------
	{"panel_port", "порт панели",
		limits("порт панели", func(s *Settings) int { return s.PanelPort }, 1, 65535, "")},
	{"core_port", "управляющий интерфейс ядра",
		limits("порт управляющего интерфейса ядра", func(s *Settings) int { return s.CorePort }, 1, 65535, "")},
	{"probe_port", "служебный вход замера",
		limits("порт служебного входа замера", func(s *Settings) int { return s.ProbePort }, 1, 65535, "")},

	// Порты не должны совпадать: иначе ядро не поднимется, а причина будет
	// выглядеть как что угодно, только не как совпадение портов.
	{"ports_distinct", "порты панели, ядра и служебного входа различны",
		func(s *Settings) error {
			if s.PanelPort == s.CorePort || s.PanelPort == s.ProbePort || s.CorePort == s.ProbePort {
				return fail.New(fail.BadRequest, fmt.Sprintf(
					"порты должны быть разными: панель %d, ядро %d, служебный вход %d",
					s.PanelPort, s.CorePort, s.ProbePort))
			}
			return nil
		}},

	// --- страховочные сроки ------------------------------------------------
	{"core_ready_deadline", "сколько ждать ответа управляющего интерфейса после перезапуска",
		limits("страховочный срок готовности ядра", func(s *Settings) int { return s.CoreReadyDeadline }, 1, 300, "с")},

	{"busy_deadline", "через сколько нода возвращается в очередь, если результат так и не пришёл",
		limits("страховочный срок пометки занятости", func(s *Settings) int { return s.BusyDeadline }, 5, 600, "с")},

	// --- выход наружу ------------------------------------------------------
	{"outbound_family", "по какому семейству адресов нода выпускает трафик наружу",
		func(s *Settings) error {
			switch s.OutboundFamily {
			case IPv4, IPv6:
				return nil
			}
			return fail.New(fail.BadRequest, "семейство адресов на выходе: допустимо ipv4 или ipv6")
		}},

	{"probe_targets", "адреса для замера, берутся по кругу",
		func(s *Settings) error {
			if len(s.ProbeTargets) == 0 {
				return fail.New(fail.BadRequest, "список адресов замера пуст: "+
					"без него проверить ноды нечем")
			}
			if len(s.ProbeTargets) > maxProbeTargets {
				return fail.New(fail.BadRequest, fmt.Sprintf(
					"адресов замера больше допустимого: %d при пределе %d",
					len(s.ProbeTargets), maxProbeTargets))
			}
			return nil
		}},
}

// maxProbeTargets — предел объявлен отдельно, потому что это единственное
// место, где клиент присылает много (Ф-8.11), и эти адреса уходят наружу через
// ноды, то есть пригодны и для чужого сканирования (Н-6.8).
const maxProbeTargets = 32

// Defaults возвращает значения по умолчанию § 7.6 для роутера с заданным
// объёмом памяти. Два значения зависят от памяти прямо (Н-2.8, Н-2.9):
// предел нод в ядре и квота тяжёлых.
func Defaults(memoryMB int) Settings {
	limit, heavy := 50, 20 // 128 МБ: худший случай оставляет системе запас
	if memoryMB >= 192 {
		limit, heavy = 100, 60 // 256 МБ
	}
	return Settings{
		CoreNodeLimit: limit,
		NoWorkingNode: KeepInternet,
		SwitchPolicy:  Soft,
		SwitchWait:    200,

		ProbeConcurrency: 16,
		ProbeTimeout:     3,
		CorePollInterval: 1,

		CompositionInterval: 60,
		ReserveInterval:     60,

		FreshCandidates: 60,
		FreshBackground: 60,

		BackgroundBudget: 10,

		BatchCandidates: 20,
		BatchReserve:    20,
		BatchBackground: 20,
		BatchScan:       50,

		HeavyQuota:  heavy,
		ReserveSize: 5,

		RefreshAuto:     true,
		RefreshInterval: 24 * 60 * 60,
		PersistInterval: 60 * 60,
		PersistRegistry: false,

		PanelPort: 8088,
		CorePort:  9090,
		ProbePort: 18085,

		CoreReadyDeadline: 15,
		BusyDeadline:      60,

		OutboundFamily: IPv4,
		// Адреса, отдающие пустой ответ с кодом «нет содержимого» (Ф-3.2, Ф-3.8).
		ProbeTargets: []string{
			"http://cp.cloudflare.com/generate_204",
			"http://connectivitycheck.gstatic.com/generate_204",
			"http://www.gstatic.com/generate_204",
		},

		Version: 1,
	}
}

// Describe перечисляет настройки со смыслом: этим живёт справка в интерфейсе и
// проверка полноты — что каждое поле § 7.6 действительно описано.
func Describe() []struct{ Name, About string } {
	out := make([]struct{ Name, About string }, 0, len(fields))
	for _, f := range fields {
		out = append(out, struct{ Name, About string }{f.name, f.about})
	}
	return out
}
