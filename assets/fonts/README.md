# Bundled fonts

These fonts are registered with Fontconfig for the lifetime of `e9s-gui` and
are not installed into the user's or system's font directories. They remain
optional: the `System GTK theme` appearance mode and GTK font defaults work
without them.

Pinned upstream artifacts:

| Family | Release artifact | SHA-256 |
| --- | --- | --- |
| JetBrainsMono Nerd Font | Nerd Fonts v3.5.0 `JetBrainsMono.tar.xz` | `0227b220360a6f819b9ead92343e8112b34733054782561af50cfba1e8afab63` |
| 0xProto Nerd Font | Nerd Fonts v3.5.0 `0xProto.tar.xz` | `b6cd12d383255548292c12fc3f8b03e197407d8299393fb27e351aba42224965` |
| Inter | Inter v4.1 `Inter-4.1.zip` | `9883fdd4a49d4fb66bd8177ba6625ef9a64aa45899767dde3d36aa425756b11e` |

Only the regular, bold, italic, and (where supplied) bold-italic desktop TTF
faces are retained. The corresponding OFL licenses and the Nerd Fonts tooling
license are in `licenses/`.
