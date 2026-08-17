#!/bin/sh
# Сборка пакета .ipk для OpenWrt (Ф-0.2).
#
# Почему обычный shell, а не система сборки OpenWrt. Пакет .ipk — это архив ar
# из трёх частей: служебной записи debian-binary, архива с описанием control и
# архива с файлами data. Собрать его можно чем угодно, а SDK OpenWrt весит
# гигабайты и требует своей среды. Второго языка сборки в проекте нет (Ф-0.3):
# программа собирается одной командой go build, пакет — этим скриптом.
#
# Запуск:
#   packaging/make-ipk.sh <архитектура> <версия> <путь к собранному бинарю>
# Пример:
#   packaging/make-ipk.sh aarch64_cortex-a53 0.2.0 build/pier-arm64
set -eu

ARCH="${1:?укажите архитектуру пакета, например aarch64_cortex-a53}"
VERSION="${2:?укажите версию, например 0.2.0}"
BINARY="${3:?укажите путь к собранному бинарю}"

[ -f "$BINARY" ] || { echo "не найден бинарь: $BINARY" >&2; exit 1; }

HERE=$(cd "$(dirname "$0")" && pwd)
OUT=${OUT:-$HERE/../build}
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$OUT"

# --- файлы пакета ---------------------------------------------------------
# Каталога данных (/etc/pier) в пакете нет намеренно. Он принадлежит
# пользователю, а не пакету: настройки, источники, группы и снимки подписок
# обязаны пережить и обновление, и удаление панели. Создаёт его служба при
# запуске, а opkg о нём ничего не знает и потому не трогает.
mkdir -p "$WORK/data/usr/bin" "$WORK/data/etc/init.d"
cp "$BINARY" "$WORK/data/usr/bin/pier"
cp "$HERE/init.d/pier" "$WORK/data/etc/init.d/pier"
chmod 0755 "$WORK/data/usr/bin/pier" "$WORK/data/etc/init.d/pier"

# Права не предполагаются, а проверяются, и есть запасной путь.
#
# В пакете лежат только исполняемые файлы — бинарь панели и служба procd.
# Права на них обязаны быть 0755: иначе установка пройдёт молча, а служба
# откажется стартовать с невразумительным «Permission denied». Между тем
# сборка идёт и с рабочей машины на Windows, где файловая система бита
# выполнения не хранит, и chmod там бессилен. Тогда режим задаётся самим
# архиватором — благо все файлы пакета исполняемые, и общий режим для них
# верен.
MODE_OPT=""
if [ ! -x "$WORK/data/usr/bin/pier" ]; then
	MODE_OPT="--mode=0755"
	echo "файловая система не хранит права: режим 0755 задаётся при упаковке" >&2
fi

# --- описание -------------------------------------------------------------
mkdir -p "$WORK/control"
cat > "$WORK/control/control" <<EOF
Package: pier
Version: $VERSION
Depends: libc, kmod-tun
Architecture: $ARCH
Section: net
Priority: optional
Maintainer: wolfram0108
Description: Панель управления подключениями для роутера.
 Принимает подписки и ссылки на прокси-ноды, следит за их работоспособностью
 и держит в ядре sing-box набор рабочих нод. Трафик в свои TUN-интерфейсы
 направляет внешний механизм маршрутизации.
EOF

cat > "$WORK/control/postinst" <<'EOF'
#!/bin/sh
[ -n "${IPKG_INSTROOT:-}" ] && exit 0
/etc/init.d/pier enable
/etc/init.d/pier start
exit 0
EOF

cat > "$WORK/control/prerm" <<'EOF'
#!/bin/sh
[ -n "${IPKG_INSTROOT:-}" ] && exit 0
/etc/init.d/pier stop
/etc/init.d/pier disable
exit 0
EOF

chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

# --- сборка архива --------------------------------------------------------
# Время файлов зафиксировано: одинаковый вход обязан давать одинаковый пакет,
# иначе контрольные суммы релиза меняются от одной пересборки.
STAMP=${SOURCE_DATE_EPOCH:-0}
TAR_OPTS="--numeric-owner --owner=0 --group=0 --mtime=@$STAMP --sort=name"

( cd "$WORK/data" && tar czf "$WORK/data.tar.gz" $TAR_OPTS $MODE_OPT . )
( cd "$WORK/control" && tar czf "$WORK/control.tar.gz" $TAR_OPTS . )
echo "2.0" > "$WORK/debian-binary"

PKG="$OUT/pier_${VERSION}_${ARCH}.ipk"
( cd "$WORK" && tar czf "$PKG" $TAR_OPTS ./debian-binary ./control.tar.gz ./data.tar.gz )

echo "$PKG"
