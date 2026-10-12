> Back to [README](../../../README.md)

# Telegram

The Telegram channel uses long polling via the Telegram Bot API for bot-based communication. It supports text messages, media attachments (photos, voice, audio, documents), voice transcription ([setup](../../guides/providers.md#voice-transcription)), and built-in command handling.

## Configuration

```json
{
  "channel_list": {
    "telegram": {
      "enabled": true,
      "type": "telegram",
      "token": "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
      "allow_from": ["123456789"],
      "proxy": "",
      "use_markdown_v2": false,
      "media_group_delay_ms": 500
    }
  }
}
```

| Field            | Type   | Required | Description                                                        |
| ---------------- | ------ | -------- | ------------------------------------------------------------------ |
| enabled          | bool   | Yes      | Whether to enable the Telegram channel                             |
| token            | string | Yes      | Telegram Bot API Token                                             |
| allow_from       | array  | No       | Allowlist of user IDs; empty means all users are allowed           |
| proxy            | string | No       | Proxy URL for connecting to the Telegram API (e.g. http://127.0.0.1:7890) |
| use_markdown_v2 | bool   | No       | Enable Telegram MarkdownV2 formatting                              |
| media_group_delay_ms | int | No       | Idle delay before processing Telegram media groups/albums. Defaults to 500 ms |
| table_mode       | string | No       | Markdown table rendering in the **legacy** pipeline: `auto` (default), `code`, `list`, `off` |
| table_max_width  | int    | No       | Max display width of a legacy box table before falling back to cards. Defaults to 60 |
| rich_format      | bool   | No       | Use Telegram rich messages (`sendRichMessage`, Bot API 10.1+): native tables, headings, checklists, collapsible blocks. Enabled by default; set `false` to force the legacy HTML/MarkdownV2 pipeline |

## Rich Messages (recommended)

Since Bot API 10.1 (July 2026) Telegram offers **rich messages**: the bot sends
raw markdown via `sendRichMessage` and clients render it natively — real
tables (up to 20 columns), headings, task lists, collapsible
`<details>` blocks, quotes with attribution, formulas. The Telegram channel
uses this renderer by default for all outbound markdown (`rich_format: true`).

Fallback chain, automatic per message:

1. rich markdown → if Telegram rejects the content (or it exceeds rich limits,
   e.g. a table with more than 20 columns),
2. legacy conversion (HTML or MarkdownV2 with the GFM table pre-renderer
   below) → if that also fails to parse,
3. plain text.

Rich rendering applies to normal replies, streaming drafts
(`sendRichMessageDraft`) and message edits. Tool-feedback progress messages
keep the legacy pipeline. Older clients that predate rich messages show such
messages as plain text — disable `rich_format` if your audience uses them.

## Setup

1. Search for `@BotFather` in Telegram
2. Send the `/newbot` command and follow the prompts to create a new bot
3. Obtain the HTTP API Token
4. Fill in the Token in the configuration file
5. (Optional) Configure `allow_from` to restrict which user IDs can interact (you can get IDs via `@userinfobot`)

## Built-in Commands

Telegram auto-registers PicoClaw's top-level bot commands at startup, including `/start`, `/help`, `/show`, `/list`, and `/use`.

Skill-related commands:

- `/list skills` lists the installed skills visible to the current agent.
- `/list mcp` lists configured MCP servers and whether they are deferred/connected.
- `/show mcp <server>` lists the active tools for a connected MCP server.
- `/use <skill> <message>` forces a skill for a single request.
- `/use <skill>` arms the skill for your next message in the same chat.
- `/use clear` clears a pending skill override.

Examples:

```text
/list skills
/list mcp
/show mcp github
/use git explain how to squash the last 3 commits
/use git
explain how to squash the last 3 commits
```

## Advanced Formatting

You can set `use_markdown_v2: true` to enable enhanced formatting options. This allows the bot to utilize the full range of Telegram MarkdownV2 features, including nested styles, spoilers, and custom fixed-width blocks.

### Markdown tables (legacy pipeline)

When `rich_format` is disabled or content falls back from rich rendering,
Telegram's Bot API has no table entity, so the channel renders GFM tables into
plain text before sending:

- **Box table** — a Unicode box-drawing table inside a monospace code block
  (column widths computed with East-Asian-width awareness, alignment honored).
  Used when the rendered table fits within `table_max_width` display columns.
- **Cards** — each row becomes a bold title (first column) plus
  `field: value` bullet lines. Used automatically for wider tables, since
  Telegram has no horizontal scrolling inside code blocks.

`table_mode` forces the behavior: `auto` (default), `code` (always box),
`list` (always cards), `off` (pass raw pipe tables through, legacy behavior).
Tables inside fenced code blocks are never touched.

Also folded before sending: task lists (`- [x]`/`- [ ]` → ✅/⬜), HTML
`<details><summary>` blocks (title + block quote), single-star *italic*, and
stray inline HTML tags outside code blocks are stripped.

```json
{
  "channel_list": {
    "telegram": {
      "enabled": true,
      "type": "telegram",
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"],
      "use_markdown_v2": true,
      "rich_format": true,
      "table_mode": "auto",
      "table_max_width": 60
    }
  }
}
```
