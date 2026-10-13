# Changelog

## Unreleased

### Downloads

- Validate ranged response offsets, lengths and totals before writing file data.
- Trust demonstrated range support, rather than an `Accept-Ranges` claim; support empty files.
- Strip captured range, compression and cache headers that interfere with full downloads.
- Restart missing or truncated partial files and compare ETag / Last-Modified on resume.
- Guard ranged requests with If-Range and reject changed file responses.
- Publish saved segment offsets after writes, synchronize worker shutdown, and flush completed files before finalization.
- Treat deliberate rate limiting separately from network stalls; track retry progress per connection.
- Re-fetch data when retrying a failed checksum and validate checksum lengths before queueing.
- Preserve file data and task state on failed renames; prevent destination overwrites.
- Retain user-chosen file names and support Windows local torrent path names.
- Carry browser authentication to video and torrent downloads; strip credentials on cross-host HTTP redirects.
- Add public tracker fallbacks only after metadata confirms a torrent is public.
- Decode local torrent file URLs, including spaces and Windows drive letters; retry TCP/UDP port collisions when choosing a listening port.

### Accessibility and customization

- Keep keyboard focus inside dialogs and restore the opener on close.
- Add keyboard entry into the download list, focus-aware actions, explicit input names, and arrow-key settings navigation.
- Give command search combobox semantics and prevent global download shortcuts from intercepting buttons or dialogs.
- Preserve add-dialog input on errors, ignore stale parsing results, show inline status, and stop preview work after close.
- Add Readable: larger text, verified AA text contrast, and disabled motion.
- Add 20-step theme undo/redo, grouped slider edits, validated imports, and a default-look recovery shortcut.
- Adapt narrow windows with compact navigation, responsive dialogs and settings, and usable detail panels.
- Clearly label simulated browser previews and snapshot reactive settings before saving.
- Label theme editors for screen readers and show available audio formats when ffmpeg is missing.

### Validation

- Add HTTP recovery, file safety, authenticated torrent, private tracker, and theme regression tests.
- Run frontend checks, all Go tests, Go vet, Linux engine race detection and Windows packaging in CI.
