#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_BASENAME="wdtmon4"
APP_FILENAME="${APP_BASENAME}.app"
DISPLAY_NAME="wdtmon4"
BUNDLE_ID="ru.unitx.wdtmon4"
MIN_MACOS_VERSION="12.0"
DIST_DIR="${1:-${SCRIPT_DIR}/dist}"
ICON_SOURCE="${SCRIPT_DIR}/web/public/android-chrome-512x512.png"

APP_VERSION="$(
	awk -F '"' '/^[[:space:]]*var[[:space:]]+VERSION[[:space:]]*=/ { print $2; exit }' \
		"${SCRIPT_DIR}/main.go"
)"
if [[ ! "$APP_VERSION" =~ ^[0-9]+([.][0-9]+){1,2}$ ]]; then
	echo "Ошибка: не удалось определить версию приложения: ${APP_VERSION:-<пусто>}" >&2
	exit 1
fi

DMG_FILENAME="${APP_BASENAME}-${APP_VERSION}-macos12-universal.dmg"

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "Ошибка: DMG можно собрать только на macOS" >&2
	exit 1
fi

for tool in go node npm clang lipo otool sips iconutil plutil codesign hdiutil ditto shasum; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "Ошибка: не найден инструмент: $tool" >&2
		exit 1
	fi
done

if ! node -e '
	const [major, minor] = process.versions.node.split(".").map(Number);
	process.exit((major === 20 && minor >= 19) || major > 22 || (major === 22 && minor >= 12) ? 0 : 1);
'; then
	echo "Ошибка: Node.js $(node --version) не поддерживается; выполните: cd web && nvm use" >&2
	exit 1
fi

if [[ ! -f "$ICON_SOURCE" ]]; then
	echo "Ошибка: не найдена иконка: $ICON_SOURCE" >&2
	exit 1
fi
if [[ ! -x "${SCRIPT_DIR}/web/node_modules/.bin/vite" ]]; then
	echo "Ошибка: не установлены зависимости web; выполните: cd web && npm ci" >&2
	exit 1
fi

BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/wdtmon4-dmg.XXXXXX")"
MOUNT_DIR="${BUILD_DIR}/mounted"
MOUNTED=0

detach_image() {
	local attempt
	for attempt in 1 2 3; do
		if hdiutil detach "$MOUNT_DIR" -quiet; then
			MOUNTED=0
			return 0
		fi
		sleep 1
	done
	if hdiutil detach "$MOUNT_DIR" -force -quiet; then
		MOUNTED=0
		return 0
	fi
	return 1
}

cleanup() {
	if [[ "$MOUNTED" -eq 1 ]]; then
		detach_image || true
	fi
	rm -rf "$BUILD_DIR"
}
trap cleanup EXIT

AMD64_BIN="${BUILD_DIR}/${APP_BASENAME}-amd64"
ARM64_BIN="${BUILD_DIR}/${APP_BASENAME}-arm64"
UNIVERSAL_BIN="${BUILD_DIR}/${APP_BASENAME}"
APP_BUNDLE="${DIST_DIR}/${APP_FILENAME}"
APP_EXE="${APP_BUNDLE}/Contents/MacOS/${APP_BASENAME}"
RESOURCES_DIR="${APP_BUNDLE}/Contents/Resources"
DMG_ROOT="${BUILD_DIR}/dmg-root"
DMG_PATH="${DIST_DIR}/${DMG_FILENAME}"

build_arch() {
	local goarch="$1"
	local clang_arch="$2"
	local output="$3"

	(
		cd "$SCRIPT_DIR"
		env \
			MACOSX_DEPLOYMENT_TARGET="$MIN_MACOS_VERSION" \
			CGO_ENABLED=1 \
			GOOS=darwin \
			GOARCH="$goarch" \
			CC="clang -arch ${clang_arch}" \
			go build -trimpath -buildvcs=false \
				-ldflags "-s -w -X main.VERSION=${APP_VERSION}" \
				-o "$output" .
	)
}

echo "Сборка веб-интерфейса..."
(
	cd "${SCRIPT_DIR}/web"
	npm run build
)

echo "Сборка ${DISPLAY_NAME} ${APP_VERSION} для Intel и Apple Silicon..."
build_arch amd64 x86_64 "$AMD64_BIN"
build_arch arm64 arm64 "$ARM64_BIN"
lipo -create -output "$UNIVERSAL_BIN" "$AMD64_BIN" "$ARM64_BIN"

ARCHS="$(lipo -archs "$UNIVERSAL_BIN")"
for required_arch in x86_64 arm64; do
	if [[ " $ARCHS " != *" $required_arch "* ]]; then
		echo "Ошибка: в универсальном бинарнике нет архитектуры ${required_arch}: ${ARCHS}" >&2
		exit 1
	fi
done

mkdir -p "$DIST_DIR"
rm -rf "$APP_BUNDLE"
rm -f "$DMG_PATH"
mkdir -p "${APP_BUNDLE}/Contents/MacOS" "$RESOURCES_DIR"
cp "$UNIVERSAL_BIN" "$APP_EXE"
chmod 755 "$APP_EXE"

cat >"${APP_BUNDLE}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>ru</string>
	<key>CFBundleDisplayName</key>
	<string>${DISPLAY_NAME}</string>
	<key>CFBundleExecutable</key>
	<string>${APP_BASENAME}</string>
	<key>CFBundleIconFile</key>
	<string>wdtmon4.icns</string>
	<key>CFBundleIdentifier</key>
	<string>${BUNDLE_ID}</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>${DISPLAY_NAME}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${APP_VERSION}</string>
	<key>CFBundleVersion</key>
	<string>${APP_VERSION}</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.utilities</string>
	<key>LSMinimumSystemVersion</key>
	<string>${MIN_MACOS_VERSION}</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSHumanReadableCopyright</key>
	<string>UnitX</string>
</dict>
</plist>
EOF
plutil -lint "${APP_BUNDLE}/Contents/Info.plist" >/dev/null

ICONSET="${BUILD_DIR}/wdtmon4.iconset"
mkdir -p "$ICONSET"
sips -z 16 16 "$ICON_SOURCE" --out "${ICONSET}/icon_16x16.png" >/dev/null
sips -z 32 32 "$ICON_SOURCE" --out "${ICONSET}/icon_16x16@2x.png" >/dev/null
sips -z 32 32 "$ICON_SOURCE" --out "${ICONSET}/icon_32x32.png" >/dev/null
sips -z 64 64 "$ICON_SOURCE" --out "${ICONSET}/icon_32x32@2x.png" >/dev/null
sips -z 128 128 "$ICON_SOURCE" --out "${ICONSET}/icon_128x128.png" >/dev/null
sips -z 256 256 "$ICON_SOURCE" --out "${ICONSET}/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "$ICON_SOURCE" --out "${ICONSET}/icon_256x256.png" >/dev/null
sips -z 512 512 "$ICON_SOURCE" --out "${ICONSET}/icon_256x256@2x.png" >/dev/null
sips -z 512 512 "$ICON_SOURCE" --out "${ICONSET}/icon_512x512.png" >/dev/null
sips -z 1024 1024 "$ICON_SOURCE" --out "${ICONSET}/icon_512x512@2x.png" >/dev/null
iconutil -c icns "$ICONSET" -o "${RESOURCES_DIR}/wdtmon4.icns"

for arch in x86_64 arm64; do
	ACTUAL_MIN_VERSION="$(
		otool -l -arch "$arch" "$APP_EXE" | \
			awk '$1 == "minos" { print $2; exit }'
	)"
	if [[ "$ACTUAL_MIN_VERSION" != "$MIN_MACOS_VERSION" ]]; then
		echo "Ошибка: минимальная версия macOS для ${arch}: ${ACTUAL_MIN_VERSION:-не определена}, ожидалась ${MIN_MACOS_VERSION}" >&2
		exit 1
	fi
done

if otool -L "$APP_EXE" | grep -E '/(usr/local|opt/homebrew)/' >/dev/null; then
	echo "Ошибка: приложение зависит от внешней библиотеки Homebrew" >&2
	exit 1
fi

codesign --force --sign - "$APP_BUNDLE"
codesign --verify --deep --strict --verbose=2 "$APP_BUNDLE"

mkdir -p "$DMG_ROOT"
ditto "$APP_BUNDLE" "${DMG_ROOT}/${APP_FILENAME}"
ln -s /Applications "${DMG_ROOT}/Applications"
hdiutil create -volname "$DISPLAY_NAME" -srcfolder "$DMG_ROOT" \
	-ov -format UDZO "$DMG_PATH"
codesign --force --sign - "$DMG_PATH"
codesign --verify --verbose=2 "$DMG_PATH"
hdiutil verify "$DMG_PATH"

mkdir -p "$MOUNT_DIR"
hdiutil attach "$DMG_PATH" -readonly -nobrowse -mountpoint "$MOUNT_DIR" -quiet
MOUNTED=1
MOUNTED_APP="${MOUNT_DIR}/${APP_FILENAME}"
codesign --verify --deep --strict --verbose=2 "$MOUNTED_APP"
VERSION_OUTPUT="$("${MOUNTED_APP}/Contents/MacOS/${APP_BASENAME}" --version)"
if [[ "$VERSION_OUTPUT" != "wdtmon4 v${APP_VERSION}" ]]; then
	echo "Ошибка: неожиданная версия приложения: ${VERSION_OUTPUT}" >&2
	exit 1
fi
detach_image

echo
echo "Готово:"
echo "  ${APP_BUNDLE}"
echo "  ${DMG_PATH}"
echo "  Архитектуры: ${ARCHS}"
LC_ALL=C shasum -a 256 "$DMG_PATH"
