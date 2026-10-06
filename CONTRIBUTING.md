# Contributing to ZapDesk

Thanks for your interest. ZapDesk is an early, single-maintainer project, so small and focused
contributions are easiest to review.

## Reporting bugs and ideas

Open an [issue](https://github.com/Gabriel-Almeida0/zapdesk/issues) with:

- macOS version and chip (ZapDesk only targets Apple Silicon, macOS 14+);
- ZapDesk version (or commit) and whether you use the DMG or run from source;
- what you did, what you expected and what happened (logs help).

**Never paste phone numbers, message contents, session files or API keys** in an issue. The
WhatsApp session lives in `~/Library/Application Support/ZapDesk`; do not attach anything from it.

## Development setup

Requirements: macOS on Apple Silicon, Node ≥ 22.12, Go 1.27.

```bash
npm ci
npm run dev           # engine + shared packages + app
```

The engine can run against a fake WhatsApp backend for development and tests; see
`specs/001-zapdesk-mvp/quickstart.md` and `mcp/README.md`.

## Before a pull request

1. Read `docs/features/` and the relevant `specs/` folder for larger changes (they are in
   Portuguese; the code identifiers are too).
2. Run the checks that CI runs:

   ```bash
   npm run tipos         # tsc --noEmit in every workspace
   npm test              # Node tests
   npm run motor:testar  # Go tests
   (cd motor && go vet ./...)
   ```

3. Keep the PR focused, describe what changed and how you tested it.

## Scope

Out of scope by design: number rotation or any technique to evade WhatsApp's spam detection,
and calls. Windows/Linux support and notarized builds are welcome discussions in an issue first.

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).
