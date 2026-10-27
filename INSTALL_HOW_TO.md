# Сборка Goink на Windows 11

Полный процесс сборки `goink.exe` и Inno Setup-инсталлятора на машине под Windows 11.
Проверено на практике 2026-10-27. Все команды помечены оболочкой:
**[PS]** — PowerShell, **[MSYS2]** — терминал MSYS2.

Результат:
- `build\bin\goink.exe` — портативное приложение;
- `build\dist\goink-v<версия>-windows-amd64.exe` — инсталлятор для раскатки по другим машинам.

---

## 1. Одноразовая подготовка окружения

### 1.1 MSYS2 (оболочка + утилиты + C-компилятор для cgo)

1. Скачать и установить с https://www.msys2.org (64-bit).
2. **[MSYS2]**:
   ```bash
   pacman -Syu          # обновить систему; после — закрыть и открыть терминал заново
   pacman -S git make curl unzip file jq tar
   pacman -S mingw-w64-x86_64-toolchain   # gcc для cgo (в группе 12 пакетов — Enter = all)
   ```
3. Проверка:
   ```bash
   make --version && jq --version && which gcc
   ```
   `which gcc` должен дать `/mingw64/bin/gcc`.

> Важно: использовать оболочку **MINGW64** (стандартный терминал MSYS2) — под неё
> и ставится `mingw-w64-x86_64-toolchain`. Окружение ucrt64 требует свой пакет
> (`mingw-w64-ucrt-x86_64-toolchain`).

### 1.2 Go 1.25.x

1. Скачать Windows-инсталлятор amd64 с https://go.dev/dl — версия **1.25.x**
   (`go.mod` требует `go 1.25.0`; версии новее, например 1.27.x, могут иметь
   регрессии компилятора — см. «Подводные камни»).
2. Установить (ставит в `C:\Program Files\Go`).
3. **Закрыть и открыть** терминал MSYS2.
4. **[MSYS2]**:
   ```bash
   go version    # go1.25.x
   go env GOROOT # C:\Program Files\Go
   ```

### 1.3 Node.js 22 LTS + npm 10

1. Скачать **Node 22 LTS** (Windows x64) с https://nodejs.org и установить
   (ставит в `C:\Program Files\nodejs`).
2. **Закрыть и открыть** терминал MSYS2.
3. **[MSYS2]**:
   ```bash
   node -v   # v22.x
   npm -v    # 10.9.x
   ```

> **Не ставить Node 24.** npm ≥ 11.5/12 по новой политике безопасности блокирует
> скачивание пакетов с прямых URL, а `package-lock.json` этого проекта содержит
> ~1085 ссылок на `registry.npmmirror.com` — `npm ci` падает с `EALLOWREMOTE`.

> **Не выполнять `npm install -g npm`.** Глобальная установка создаёт shim'ы
> в `%APPDATA%\npm`, которые затеняют штатный npm из Node — та же ошибка
> `EALLOWREMOTE` даже при Node 22. Если уже поставили — удалить файлы
> `%APPDATA%\npm\npm`, `npm.cmd`, `npx`, `npx.cmd` и `%APPDATA%\npm\node_modules`.

### 1.4 Wails CLI v2.12.0

```
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```
(выполнить в **[PS]** или **[MSYS2]** — `go` один и тот же).

Добавить `%USERPROFILE%\go\bin` в PATH (Панель управления → Система →
Дополнительные параметры системы → Переменные среды → системная/пользовательская
`Path` → «Новое»). **Закрыть и открыть** терминал.

Проверка:
```bash
wails version   # v2.12.0  (флаг -version не существует)
```

### 1.5 Inno Setup 6 (только для инсталлятора)

```
winget install JRSoftware.InnoSetup
```
(выполнить в **[PS]**).

Возможные варианты установки:
- стандартное: `C:\Program Files (x86)\Inno Setup 6\`;
- per-user: `C:\Users\<user>\AppData\Local\Programs\Inno Setup 6\`.

Узнать реальное расположение:
```
Get-ItemProperty "HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*" |
  Where-Object { $_.DisplayName -like "*Inno*" } |
  Select-Object DisplayName, InstallLocation
```

### 1.6 WebView2 Runtime

На Windows 11 предустановлен. Проверка:
```
reg query "HKLM\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE26-3693-9B74-1A21C54369E3}" /v pv
```
Если версии нет — установщик: https://go.microsoft.com/fwlink/p/?LinkId=2124703

---

## 2. Репозиторий

```
git clone https://github.com/gm3u3-cloud/goink.git
cd goink
```

Русификация (если репо без неё): применить изменения — `frontend/src/i18n/locales/ru.json`,
`scripts/validate-i18n.py`, `frontend/src/i18n/index.ts`, `frontend/src/views/InitView.tsx`,
`frontend/src/components/settings/GeneralConfigTab.tsx`.

Проверка полноты перевода:
```bash
python scripts/validate-i18n.py
```
(нужен python3; критерии: все ключи `en.json` в `ru.json`, паритет плейсхолдеров `{{...}}`,
все 4 плюральные формы `_one/_few/_many/_other`).

---

## 3. Сборка

### 3.1 Зависимости рантайма (первый раз, ~170 МБ, 3–10 минут)

```
make deps
```
Скачивает в `build/runtime/`:
- `onnxruntime.dll` v1.26.0 + VC++ рантайм (`vcruntime140*.dll`, `msvcp140.dll`);
- MinGit 2.47.1;
- BGE-модель `bge-small-zh-v1.5` (int8) + `vocab.txt` (HuggingFace, фолбэк на hf-mirror.com).

При повторном запуске скачанное не качается.

### 3.2 Сборка приложения (первый раз, 5–15 минут)

```
make build
```
Последовательно:
1. `make deps`;
2. `cd frontend && npm ci && npm run build` (Vite);
3. `wails build` — компиляция Go + cgo (наиболее долгий этап).

Результат: `build\bin\goink.exe`.

### 3.3 Запуск без инсталлятора (dev-режим)

Приложение ищет `runtime/` (git, onnx, модели) **в каталоге exe**.
Для прямого запуска `build\bin\goink.exe` нужно связать каталоги:

```
New-Item -ItemType Junction -Path <repo>\build\bin\runtime -Target <repo>\build\runtime
```
(**[PS]**; либо `cp -r build/runtime build/bin/runtime` в **[MSYS2]**).

Данные (БД, романы) при этом создаются в каталоге exe (portable-режим).

```
cd build/bin
./goink.exe
```

---

## 4. Инсталлятор

`iscc` должен быть в PATH. Если Inno Setup стоит в нестандартном месте:

```
export PATH="<путь к Inno Setup 6 в msys2-формате>:$PATH"
# пример: export PATH="/c/Users/<user>/AppData/Local/Programs/Inno Setup 6:$PATH"
```

Сборка:
```
make package-windows VERSION=1.4.0
```
(`VERSION` нормализуется Makefile в 4 сегмента для PE-версионных ресурсов;
без параметра версия берётся из `git describe`, а без тегов даёт
`v<hash>-dirty` в имени файла.)

Результат: `build\dist\goink-v1.4.0-windows-amd64.exe`.

### Что делает инсталлятор на целевой машине

- кладёт `goink.exe` + `runtime/` (git, onnxruntime.dll, модели) + VC++ DLL в один каталог
  (по умолчанию `D:\Goink`, иначе `E:\Goink` / `C:\Goink`);
- VC++ DLL ставятся **рядом с exe** (загрузчик ищет зависимости onnxruntime.dll
  в каталоге exe);
- иконки в «Пуске» и на рабочем столе, пункт «Удалить Goink»,
  опция «запустить после установки»;
- данные (БД, романы, настройки) — в каталоге установки (он пишется без админа).

Требования целевой машины: Windows 10/11 + WebView2 Runtime (на Win11 предустановлен).
Никаких ручных шагов не требуется.

---

## 5. Подводные камни (встреченные при сборке)

| Симптом | Причина | Решение |
|---|---|---|
| `make: *** No rule to make target 'deps'` | Команда не в корне репозитория | `cd` в корень, где лежит `Makefile` |
| `go`/`node`/`npm` не видны в MSYS2, но есть в PowerShell | Терминал открыт до установки; PATH не обновляется | Закрыть и открыть терминал заново |
| `npm error code EALLOWREMOTE ... Refusing to fetch "...npmmirror.com..."` | npm ≥ 11.5/12 (Node 24) блокирует URL-зависимости из lockfile | Node 22 LTS (npm 10.9) |
| То же самое при Node 22 | Глобально установленный npm (`%APPDATA%\npm`) затеняет штатный | Удалить shim'ы `npm`/`npm.cmd`/`npx`/`npx.cmd` и `node_modules` из `%APPDATA%\npm` |
| `EALLOWREMOTE` не лечится, `npm -v` = 12 | `npm config list` покажет `prefix = %APPDATA%\npm` | То же: удалить shim'ы |
| `npm warn EBADENGINE ... i18next-parser ... node ^18\|^20\|^22` | Node новее 22 | Не фатально, но поставить Node 22 |
| `reading https://proxy.golang.org/...: 403 Forbidden` | proxy.golang.org недоступен из сети | `go env -w GOPROXY=https://goproxy.cn,direct` (фолбэк: `GOPROXY=direct`) |
| `internal error: package "os" without types was imported from ...` | Регрессия/проблема Go 1.27.x | Go 1.25.x (версия из `go.mod`) + `go clean -cache` |
| `build constraints exclude all Go files in .../cgo` | `CGO_ENABLED=0`, C-компилятор не найден | `pacman -S mingw-w64-x86_64-toolchain`, затем `go env -w CGO_ENABLED=1` при необходимости |
| `EPERM: rmdir ... node_modules` | Файлы node_modules захвачены процессом | Закрыть IDE/терминалы, `rm -rf node_modules` |
| `iscc: command not found` | Inno Setup не в PATH (per-user установка) | `export PATH="<InstallLocation>:$PATH"` (см. 1.5) |
| Приложение стартует без окна | Нет WebView2 Runtime | Установить (п. 1.6) |
| `goink.exe` не видит модели/git при запуске из `build\bin` | `runtime/` ищется в каталоге exe | Junction (п. 3.3) — для инсталлятора не нужно |

---

## 6. Проверка результата

1. `python scripts/validate-i18n.py` — полнота `ru.json`.
2. `make lint-frontend` — ESLint (0 ошибок).
3. В `frontend/`: `npm test` — vitest (334 теста).
4. Запустить `goink.exe`: стартовый экран → тема → **Русский** → «Начать».
5. Прогнать экраны: bookshelf, chapters, characters, locations, arcs, timeline,
   git history, reader, skills, settings.
6. Плюрализация: 1 / 2–4 / 5–20 / 21+ («1 глава», «3 главы», «5 глав», «21 глава»).
7. Инсталлятор: чистая установка на второй машине → запуск → данные в каталоге установки.

---

## 7. Итоговые версии окружения (проверенные)

| Компонент | Версия |
|---|---|
| OS | Windows 11 |
| MSYS2 | окружение MINGW64 |
| C-тулчейн | mingw-w64-x86_64-toolchain |
| Go | 1.25.x (в `go.mod` — 1.25.0) |
| Node.js | 22 LTS |
| npm | 10.9.x (штатный, без глобальных shim'ов) |
| Wails CLI | v2.12.0 |
| Inno Setup | 6.x |
