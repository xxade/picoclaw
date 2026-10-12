# План: расширенное форматирование Telegram (таблицы и др.)

Репозиторий: форк https://github.com/xxade/picoclaw (origin), upstream https://github.com/sipeed/picoclaw.
Рабочая копия: D:\picoclaw, ветка `feature/telegram-extended-formatting`.

## Текущее состояние (проверено в коде)

- Канал: `pkg/channels/telegram/` поверх `github.com/mymmrac/telego`.
- Два режима рендеринга markdown → Telegram:
  - `parser_markdown_to_html.go` → `markdownToTelegramHTML` (parse_mode=HTML, по умолчанию);
  - `parse_markdown_to_md_v2.go` → `markdownToTelegramMarkdownV2` (флаг конфига `use_markdown_v2`).
- Оба — regex-конвертеры без AST. Точки входа: `parseContent()` (telegram.go:1443), `sendChunk()` с fallback в plain text при ошибке парсинга.
- Поддерживается: bold/italic/strike, code blocks, inline code, ссылки, raw URL, списки (`-`/`*` → `•`), заголовки → bold.
- Не поддерживается: **таблицы GFM**, task lists, `<details>`, входящие HTML-теги, выносные сноски.
- Ограничение Bot API: у Telegram **нет entity для таблиц** в HTML и MarkdownV2. Любая «поддержка таблиц» = рендер в текст.

## Этап 0 — подготовка

- [ ] Установить Go в WSL (нет в окружении; нужен только для `go test` пакетов парсеров — не для кросс-сборки).
- [ ] Ветка `feature/telegram-extended-formatting` от актуального main форка.
- [ ] Baseline: `go test ./pkg/channels/telegram/... ./pkg/channels/ ./pkg/config/...` зелёные.

## Этап 1 — таблицы (ядро задачи)

Новые файлы в `pkg/channels/telegram/` (диффуз держим внутри канала — важно для merge-аблиости upstream):

- [ ] `table.go` — распознаватель GFM pipe-таблиц:
  - заголовок + separator-строка (`|---|:--:|--:|`), выравнивание;
  - escape `\|` внутри ячеек; пустые ячейки; строки разной ширины (pad/truncate);
  - НЕ матчить таблицы внутри fenced code и inline code.
- [ ] Рендер в моноширинную псевдографику (`┌─┬─┐ ├─┼─┤ └─┴─┘`) внутри code block:
  - ширина колонок через `github.com/mattn/go-runewidth` (уже indirect-зависимость) — корректно для кириллицы, CJK, emoji;
  - выравнивание колонок (left/center/right) через pad пробелами.
- [ ] Auto-fallback «карточки»: если таблица шире лимита (по умолчанию ~60 колонок-символов), рендер блоком
  `**заголовок строки**` + `поле: значение` (горизонтального скролла в Telegram нет).
- [ ] Интеграция с `channels.SplitMessage`: таблица как единый блок не рвётся посреди строки;
  проверка лимита 4096 после расширения (в HTML-режиме `<pre>` добавляет символы — учтено в telegram.go:257).
- [ ] Один общий пайплайн для обоих режимов: таблица детектится ДО прочего markdown, результат — готовый `<pre>`-блок (HTML-режим) или fenced ``` -блок (MarkdownV2-режим).
- [ ] Юнит-тесты: базовая таблица, выравнивание, `\|` в ячейке, кириллица/CJK/emoji, узкий fallback, таблица внутри code block (не трогать), разрыв длинной таблицы.

## Этап 2 — остальное расширенное форматирование

- [ ] Task lists `- [ ]` / `- [x]` → `⬜` / `✅`.
- [ ] `<details><summary>` и HTML-спойлеры → Telegram `||spoiler||`.
- [ ] Входящие HTML-теги (`<b>`, `<br>`, `<table>`...) из ответов моделей: санитизация/конвертация вместо сырых `<...>`.
- [ ] MarkdownV2-режим: pass-through нативных `~~subscript~~`, `##superscript##`, `!TG Emoji`; вложенные цитаты `>>` (block quote with attribution).
- [ ] Не делаем (осознанно): картинки-таблицы через headless-рендерер — PicoClaw таргетит слабые устройства.

## Этап 3 — конфигурация

- [ ] `pkg/config/config.go` (TelegramConfig, ~549): `table_mode: code|list|auto|off` (default auto), `table_max_width` (default 60).
- [ ] Проброс в канал + дефолты.
- [ ] Поля в web-конфигураторе: `web/frontend/src/components/channels/channel-forms/telegram-form.tsx`.

## Этап 4 — документация и PR

- [ ] `docs/channels/telegram/README.md` (+ zh): новый раздел «Таблицы», таблица опций.
- [ ] `CHANGES_TELEGRAM_FORMATTING.md` в корне: что сделано / добавлено / открыто.
- [ ] Коммиты в feature-ветку, push в origin (xxade/picoclaw).
- [ ] Опционально: PR в sipeed/picoclaw upstream (диффуз изолирован в канале + опциональные конфиги —_merge_-приемлемо).

## Открытые решения (нужен ответ перед этапом 1)

1. Стиль рамок по умолчанию: полная псевдографика `┌─┬─┐` или лёгкая `│ row │`? (предложение: полная + auto-fallback в карточки).
2. Включать ли таблицы по умолчанию (`auto`) или за флагом (дефолт `off`) на первом PR?
