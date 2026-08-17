// Программа pier — панель управления подключениями для роутера.
//
// Здесь только сборка зависимостей и запуск: логики нет и быть не должно.
// Каждый слой панели живёт своим пакетом (doc/КАРТА.md), а этот файл лишь
// соединяет их и передаёт управление.
//
// Такое разделение не ради красоты: слои связаны интерфейсами, и подставить
// вместо настоящих часов управляемые, а вместо настоящего ядра — заглушку
// можно только тогда, когда связывание собрано в одном месте.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/wolfram0108/pier/internal/clock"
	"github.com/wolfram0108/pier/internal/event"
	"github.com/wolfram0108/pier/internal/settings"
)

// version подставляется при сборке: -ldflags "-X main.version=v0.2.0".
var version = "dev"

func main() {
	var (
		root    = flag.String("root", "/etc/pier", "каталог состояния на флеше")
		tmp     = flag.String("tmp", "/tmp/pier", "каталог файлов провайдеров (tmpfs)")
		listen  = flag.String("listen", "", "адрес и порт панели; пусто — из настроек")
		verbose = flag.Bool("verbose", false, "подробные сообщения в системный журнал")
		showVer = flag.Bool("version", false, "напечатать версию и выйти")
	)
	flag.Parse()

	if *showVer {
		fmt.Println(version)
		return
	}

	// Журнал панели уходит в системный (Ф-1.9, § 4.3): своих файлов на флеше
	// панель не ведёт — за объём хранения отвечает система, а не она.
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	log.Info("панель запускается", "версия", version, "состояние", *root, "провайдеры", *tmp)

	// Часы — единственный источник времени (Н-5.6). Передаются слоям явно,
	// чтобы в опытах их можно было заменить управляемыми.
	clk := clock.NewSystem()
	_ = clk

	// Шина событий: один поток на клиента (Ф-8.13). Размер буфера подписчика
	// невелик намеренно — отставший клиент перечитывает состояние, а не
	// догоняет историю (§ 8.3).
	bus := event.NewBus(64)
	_ = bus

	// Настройки: при первом запуске берутся умолчания § 7.6, выбранные по
	// объёму памяти роутера (Н-2.8, Н-2.9).
	cfg := settings.Defaults(memoryMB())
	if err := cfg.Validate(0); err != nil {
		log.Error("умолчания настроек не прошли проверку", "ошибка", err)
		os.Exit(1)
	}
	log.Info("настройки по умолчанию",
		"предел нод", cfg.CoreNodeLimit,
		"квота тяжёлых", cfg.HeavyQuota,
		"порт панели", cfg.PanelPort)

	_ = *listen

	// Остановка по сигналу: служба procd шлёт SIGTERM. Панель обязана успеть
	// сбросить накопленное на флеш (§ 6.5) — этим займётся слой хранилища.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Info("каркас поднят; слои состояния, ядра и API появятся по этапам роадмапа")
	<-stop
	log.Info("панель останавливается")
}

// memoryMB — объём памяти роутера. От него зависят два умолчания: предел нод в
// ядре и квота тяжёлых протоколов (Н-2.8, Н-2.9).
//
// Значение читается из системы; когда прочитать нечего — предполагается
// меньшая конфигурация. Ошибиться в меньшую сторону безопасно: панель займёт
// меньше, чем могла бы, и роутер продолжит работать. Ошибка в большую сторону
// означает нехватку памяти под нагрузкой.
func memoryMB() int {
	const fallback = 128

	body, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return fallback
	}
	var kb int
	if _, err := fmt.Sscanf(string(body), "MemTotal: %d kB", &kb); err != nil || kb <= 0 {
		return fallback
	}
	return kb / 1024
}
