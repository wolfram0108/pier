// Пакет settings — настройки панели: значения, пределы, проверка.
//
// Всё, что человек может настроить, описано одной таблицей (catalog.go): имя
// поля, смысл, значение по умолчанию, пределы и проверка. Добавить настройку —
// добавить туда запись; ни обработчик API, ни хранилище, ни интерфейс при этом
// не правятся.
//
// Требования, которые держит этот пакет:
//
//	§ 7.6   состав настроек и значения по умолчанию;
//	Ф-8.7   настройки применяются целиком, одним действием, а не по полю:
//	        часть значений связана между собой и проверяется совместно;
//	Ф-4.17  общий предел нод не применяется молча ниже уже распределённого;
//	Н-5.6   все длительности — в секундах, отсчёт монотонный.
//
// Умолчания зависят от объёма памяти роутера (Н-2.8, Н-2.9): два значения на
// 128 и 256 МБ. Выбор делается один раз при первом запуске, по факту
// обнаруженной памяти, а не назначается по вкусу.
package settings

import (
	"time"

	"github.com/wolfram0108/pier/internal/fail"
)

// Settings — полный набор настроек панели. Поля названы по § 7.6; имена в JSON
// — через нижнее подчёркивание, как в конфигурации ядра (§ 8.1).
type Settings struct {
	// --- пределы и поведение канала ---
	CoreNodeLimit int      `json:"core_node_limit"` // предел нод в ядре, общий на все группы
	NoWorkingNode Fallback `json:"no_working_node"` // при отсутствии рабочих нод
	SwitchPolicy  Switch   `json:"switch_policy"`   // политика смены активной ноды
	SwitchWait    int      `json:"switch_wait"`     // срок ожидания при смене, с

	// --- проверка нод ---
	ProbeConcurrency int `json:"probe_concurrency"`  // одновременность замеров внутри порции
	ProbeTimeout     int `json:"probe_timeout"`      // время ожидания ответа ноды, с
	CorePollInterval int `json:"core_poll_interval"` // как часто панель спрашивает ядро, с

	CompositionInterval int `json:"composition_interval"` // интервал опроса состава, с
	ReserveInterval     int `json:"reserve_interval"`     // интервал опроса запаса, с

	FreshCandidates int `json:"fresh_candidates"` // срок годности: поиск и сканирование, с
	FreshBackground int `json:"fresh_background"` // срок годности: набор запаса, с

	BackgroundBudget int `json:"background_budget"` // бюджет фонового набора, замеров в минуту

	BatchCandidates int `json:"batch_candidates"` // размер порции: поиск кандидатов
	BatchReserve    int `json:"batch_reserve"`    // размер порции: опрос запаса
	BatchBackground int `json:"batch_background"` // размер порции: набор запаса
	BatchScan       int `json:"batch_scan"`       // размер порции: сканирование по команде

	HeavyQuota  int `json:"heavy_quota"`  // квота на одновременно поднятые тяжёлые ноды
	ReserveSize int `json:"reserve_size"` // размер запаса на каждую группу

	// --- источники и хранение ---
	RefreshAuto     bool `json:"refresh_auto"`     // обновлять подписки автоматически
	RefreshInterval int  `json:"refresh_interval"` // интервал обновления подписок, с
	PersistInterval int  `json:"persist_interval"` // интервал сохранения состава и запаса, с
	PersistRegistry bool `json:"persist_registry"` // сохранять статистику всего реестра

	// --- порты ---
	PanelPort int `json:"panel_port"` // порт панели
	CorePort  int `json:"core_port"`  // управляющий интерфейс ядра
	ProbePort int `json:"probe_port"` // служебный вход замера

	// --- страховочные сроки ---
	CoreReadyDeadline int `json:"core_ready_deadline"` // ожидание готовности ядра, с
	BusyDeadline      int `json:"busy_deadline"`       // страховочный срок пометки занятости, с

	// --- выход наружу ---
	OutboundFamily Family   `json:"outbound_family"` // семейство адресов на выходе через ноду
	ProbeTargets   []string `json:"probe_targets"`   // адреса для замера, по кругу

	// Version растёт при каждом изменении: по ней отклоняется правка поверх
	// устаревшего состояния (Ф-8.8) и уходит событие settings (Ф-8.9).
	Version int `json:"version"`
}

// Fallback — что делать, когда через группу выпустить трафик нечем (Ф-4.10).
type Fallback string

const (
	// KeepInternet — трафик идёт напрямую, мимо прокси. Значение по умолчанию:
	// остаться в сети важнее, чем ничего (§ 7.6). Пока режим действует, панель
	// говорит об этом непрерывно и прямым текстом (Ф-4.12).
	KeepInternet Fallback = "keep_internet"
	// Block — трафик не проходит.
	Block Fallback = "block"
)

// Switch — судьба уже открытых соединений при смене активной ноды (§ 7.4).
type Switch string

const (
	// Soft — не трогать, доживут сами. По умолчанию: механизм смены сам по
	// себе соединений не рвёт (И-4).
	Soft Switch = "soft"
	// Wait — дать время закончиться, затем оборвать.
	Wait Switch = "wait"
	// Immediate — оборвать сразу.
	Immediate Switch = "immediate"
)

// Family — семейство адресов, которым нода выпускает трафик наружу.
// Одно и то же для замера и для трафика (Ф-3.57).
type Family string

const (
	// IPv4 — по умолчанию: выход по IPv4 есть у всех прокси-серверов,
	// по IPv6 — у меньшинства (Ф-3.59).
	IPv4 Family = "ipv4"
	IPv6 Family = "ipv6"
)

// Duration переводит настройку-секунды в длительность.
func Duration(seconds int) time.Duration { return time.Duration(seconds) * time.Second }

// Validate проверяет набор целиком: и каждое значение по своим пределам, и
// связанные значения совместно.
//
// allocated — сколько мест уже распределено лимитами групп. Проверка Ф-4.17:
// общий предел нельзя опустить ниже распределённого молча.
func (s *Settings) Validate(allocated int) error {
	for _, f := range fields {
		if f.check == nil {
			continue // тумблер: недопустимых состояний у булева значения нет
		}
		if err := f.check(s); err != nil {
			return err
		}
	}
	if s.CoreNodeLimit < allocated {
		return fail.New(fail.CoreLimitExceeded, allocated)
	}
	return nil
}
