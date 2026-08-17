// Пакет event — поток живых данных в интерфейс.
//
// Требование Ф-8.4: состояние ядра, ход проверки, смена состояний нод, авария и
// трафик доставляются потоком событий; браузер ничего не опрашивает по кругу.
// Ф-8.13 уточняет исполнение: поток событий сервера поверх обычного HTTP, один
// на клиента, односторонний — команды идут обычными запросами.
//
// Что поток НЕ везёт (§ 8.3): списки нод, содержимое источников и всё, что
// нужно листать. Событие сообщает, что́ изменилось, а не везёт изменённое
// целиком — иначе поток превращается в опрос наоборот.
//
// Устройство: одна шина, у каждого подписчика свой буфер. Медленный подписчик
// не задерживает остальных: его буфер переполняется, и он получает признак
// «перечитай состояние заново» (resync) вместо накопленной очереди. Это то же
// самое, что панель отвечает после обрыва потока (§ 8.3), поэтому и обработка
// в интерфейсе одна.
package event

import (
	"sync"
)

// Event — одно событие потока. Поля объявлены в kinds.go вместе с видами.
type Event struct {
	Kind Kind           // что произошло
	Data map[string]any // поля события; состав задан каталогом видов
}

// Bus — шина событий панели. Одна на панель; подписчиков сколько угодно.
type Bus struct {
	mu     sync.RWMutex
	subs   map[int]*Subscription
	nextID int
	// Размер буфера подписчика. Небольшой намеренно: события описывают
	// изменения, а не везут данные, и отставший клиент должен перечитать
	// состояние, а не догонять историю.
	buffer int
}

// Subscription — подписка одного клиента.
type Subscription struct {
	id   int
	ch   chan Event
	bus  *Bus
	once sync.Once

	mu     sync.Mutex
	missed bool // переполнялся ли буфер: клиенту нужен resync
}

// NewBus заводит шину. Размер буфера подписчика — сколько событий держать,
// пока клиент их не забрал.
func NewBus(buffer int) *Bus {
	if buffer < 1 {
		buffer = 64
	}
	return &Bus{subs: make(map[int]*Subscription), buffer: buffer}
}

// Subscribe заводит подписку. Закрывать её обязан тот, кто завёл.
func (b *Bus) Subscribe() *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	s := &Subscription{id: b.nextID, ch: make(chan Event, b.buffer), bus: b}
	b.subs[s.id] = s
	return s
}

// Publish рассылает событие всем подписчикам.
//
// Рассылка не блокируется никогда: если буфер подписчика полон, событие для
// него теряется, а сам он помечается как отставший. Иначе один зависший
// браузер останавливал бы работу всей панели.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs {
		select {
		case s.ch <- e:
		default:
			s.markMissed()
		}
	}
}

// Emit — короткая запись публикации: вид и поля.
func (b *Bus) Emit(kind Kind, data map[string]any) { b.Publish(Event{Kind: kind, Data: data}) }

// Subscribers — сколько сейчас подписчиков. Нужно для показа в состоянии
// системы и для проверок.
func (b *Bus) Subscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// Events — канал событий подписки.
func (s *Subscription) Events() <-chan Event { return s.ch }

// Missed сообщает и сбрасывает признак отставания. Истина означает: клиент
// пропустил события, ему нужно перечитать состояние заново (§ 8.3, resync).
func (s *Subscription) Missed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	was := s.missed
	s.missed = false
	return was
}

func (s *Subscription) markMissed() {
	s.mu.Lock()
	s.missed = true
	s.mu.Unlock()
}

// Close отписывает клиента. Повторный вызов безопасен.
func (s *Subscription) Close() {
	s.once.Do(func() {
		s.bus.mu.Lock()
		delete(s.bus.subs, s.id)
		s.bus.mu.Unlock()
		close(s.ch)
	})
}
