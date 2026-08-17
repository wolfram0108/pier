package clock

import (
	"sync"
	"time"
)

// Fake — часы, которыми управляют вручную. Живут в основном коде, а не в
// тестовом файле, намеренно: ими пользуются и опыты на стенде, и проверки
// поведения во времени, которых в этом проекте много — сроки годности,
// интервалы опроса, страховочные сроки.
//
// Без таких часов проверка «нода возвращается в очередь через 60 секунд»
// занимает минуту и зависит от загрузки машины. С ними — микросекунды и
// повторяемый результат.
type Fake struct {
	mu      sync.Mutex
	now     Moment
	cal     time.Time
	calOK   bool
	waiters []waiter
}

type waiter struct {
	at Moment
	ch chan time.Time
}

// NewFake заводит управляемые часы. Календарное время считается неустановленным
// до вызова SetCalendar — это же положение и на роутере сразу после включения.
func NewFake() *Fake { return &Fake{} }

// Now возвращает текущий момент управляемой шкалы.
func (f *Fake) Now() Moment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Calendar возвращает заданное календарное время и признак его установленности.
func (f *Fake) Calendar() (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cal, f.calOK
}

// SetCalendar задаёт календарное время: так проверяется поведение до и после
// синхронизации часов (Н-5.7, Н-5.9).
func (f *Fake) SetCalendar(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cal, f.calOK = t, true
}

// Advance двигает время вперёд и будит всех, чей срок наступил.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now += Moment(d)
	now := f.now
	var due []waiter
	rest := f.waiters[:0]
	for _, w := range f.waiters {
		if w.at <= now {
			due = append(due, w)
		} else {
			rest = append(rest, w)
		}
	}
	f.waiters = rest
	f.mu.Unlock()

	for _, w := range due {
		w.ch <- time.Time{}
	}
}

// Sleep у управляемых часов не ждёт по-настоящему: он ждёт, пока время
// подвинут вызовом Advance.
func (f *Fake) Sleep(d time.Duration) { <-f.After(d) }

// After возвращает канал, срабатывающий, когда время подвинут на d вперёд.
func (f *Fake) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	f.mu.Lock()
	f.waiters = append(f.waiters, waiter{at: f.now.Add(d), ch: ch})
	f.mu.Unlock()
	return ch
}
