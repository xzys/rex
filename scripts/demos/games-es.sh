# Demo: walk EmulationStation's system carousel.
#
# Runs on the Knulli host during a framebuffer capture. Keys go through a
# virtual uinput keyboard — see _uinput_key.py for why that, and not the
# gamepad injection the terminal demos use.
#
# Deliberately just the carousel. Enter opens ES's navigation-bar panel rather
# than entering a system, and at the size this plays back on the site a dense
# list of manufacturers reads as clutter; consoles sliding past reads instantly.
key() { python3 /tmp/rex-uinput-key.py "$@"; }

key 1 \
    right right right \
    1.5 \
    right right \
    1.5 \
    left left \
    1.5 \
    right right right
