# targets/ — firmware dumps and vendor trees

Everything in this directory is **gitignored** except this README and `.gitkeep`.

Firmware images, kernel trees, and extracted rootfs are large and their
redistribution is restricted (vendor blobs, PowerVR userspace, TrimUI firmware).
Keep them here so the RE tooling has a stable path, but never commit them.

Layout convention:

```
targets/
  trimui-brick/
    firmware/          # stock TrimUI images as downloaded (.img, boot_package.fex)
    extracted/         # binwalk / OpenixCard output
    device/            # pulled from the running device (config.gz, fdt, dmesg, .ko)
    ghidra/            # Ghidra project dirs (.gpr/.rep) — regenerable, still ignored
    notes.md           # per-artifact provenance: where each file came from, sha256
```

Always record a `sha256sum` and the download URL for anything vendor-supplied —
the RE notes reference specific offsets, and those are meaningless against an
unidentified image.

See `notes/brick-collection-checklist.md` for what to collect and how.
