// Пакет fail — ошибки панели: устойчивый код, готовый русский текст, ответ HTTP.
//
// Три требования сходятся в одной точке, и потому у ошибок отдельный пакет:
//
//	Ф-8.2  ошибка содержит устойчивый код и готовый к показу текст на русском;
//	Ф-8.3  «можно, но подтвердите» отличимо по коду ответа от «нельзя вовсе»;
//	И-7    текст описывает действительную причину, а не предполагаемую;
//	Т-1    сообщение называет причину и следующее действие.
//
// Если бы текст сочинялся в обработчике, одна и та же беда получала бы разные
// формулировки в разных местах, а интерфейс начал бы их переписывать — и
// нарушил § 9.3.1, где сказано показывать текст сервера как есть.
//
// Поэтому: код, текст и ответ HTTP объявляются вместе, одной записью каталога
// (catalog.go). Добавить ошибку — добавить строку туда, больше нигде.
package fail

import (
	"errors"
	"fmt"
)

// Kind — устойчивый машинный код ошибки. Не меняется между версиями панели:
// по нему принимает решения интерфейс (§ 8.1).
type Kind string

// Error — ошибка панели. Несёт всё, что нужно ответу API, и ничего сверх.
type Error struct {
	Kind    Kind           // устойчивый код
	Message string         // готовый к показу текст на русском
	Details map[string]any // необязательные подробности для показа рядом
	Cause   error          // исходная ошибка: для журнала, наружу не уходит
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return string(e.Kind) + ": " + e.Message + ": " + e.Cause.Error()
	}
	return string(e.Kind) + ": " + e.Message
}

// Unwrap открывает исходную ошибку для errors.Is и errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// New создаёт ошибку по коду из каталога.
//
// Подстановки идут в текст записи каталога: там, где причина зависит от
// обстоятельств, текст содержит места для значений (Т-2 запрещает подменять
// такую причину общей формулировкой).
func New(kind Kind, args ...any) *Error {
	rec, ok := catalog[kind]
	if !ok {
		// Неизвестный код — дефект программы, а не пользователя. Отдаём
		// внутреннюю ошибку с честным текстом: врать про причину нельзя (И-7).
		return &Error{
			Kind:    Internal,
			Message: catalog[Internal].message,
			Details: map[string]any{"unknown_code": string(kind)},
		}
	}
	msg := rec.message
	if len(args) > 0 {
		msg = fmt.Sprintf(rec.message, args...)
	}
	return &Error{Kind: kind, Message: msg}
}

// Wrap создаёт ошибку по коду каталога, сохранив исходную причину для журнала.
// Наружу исходная причина не уходит: пользователю нужен текст каталога.
func Wrap(cause error, kind Kind, args ...any) *Error {
	e := New(kind, args...)
	e.Cause = cause
	return e
}

// With добавляет подробности, которые интерфейс покажет рядом с текстом
// (поле details ответа, § 8.1). Клиент вправе их игнорировать.
func (e *Error) With(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any, 2)
	}
	e.Details[key] = value
	return e
}

// Status — код ответа HTTP для этой ошибки. Берётся из каталога, а не
// назначается в обработчике: иначе одна и та же беда отвечала бы по-разному.
func (e *Error) Status() int {
	if rec, ok := catalog[e.Kind]; ok {
		return rec.status
	}
	return 500
}

// As достаёт ошибку панели из цепочки. Удобство поверх errors.As.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Is сообщает, что ошибка в цепочке имеет указанный код.
func Is(err error, kind Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == kind
}
