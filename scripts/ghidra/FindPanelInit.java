//Locate an Allwinner BSP LCD panel driver in a stock kernel or U-Boot blob by name,
//and report cross-references so the DSI init sequence can be followed by hand.
//
//Primary RE task for the TrimUI Brick: the `jd9168` panel init sequence exists in the
//stock kernel and in U-Boot, but in no source tree.
//See notes/ghidra-linux-port-plan.md section 5.4.
//
//Usage (raw-binary import flags are in notes/ghidra-runbook.md):
//  analyzeHeadless <proj> brick -import kernel -processor AARCH64:LE:64:v8A \
//      -loader BinaryLoader -loader-baseAddr <base> \
//      -scriptPath scripts/ghidra -postScript FindPanelInit.java jd9168
//
//@category Brick
import ghidra.app.script.GhidraScript;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.*;
import ghidra.program.model.symbol.Reference;

public class FindPanelInit extends GhidraScript {

    @Override
    public void run() throws Exception {
        String needle = "jd9168";
        String[] args = getScriptArgs();
        if (args.length > 0) needle = args[0];

        println("PANEL_SEARCH needle=" + needle);
        println("LANG " + currentProgram.getLanguageID());
        println("IMAGEBASE " + currentProgram.getImageBase());

        int defined = 0;

        // Pass 1: strings Ghidra already recognised.
        DataIterator it = currentProgram.getListing().getDefinedData(true);
        while (it.hasNext() && !monitor.isCancelled()) {
            Data d = it.next();
            Object v = d.getValue();
            if (!(v instanceof String)) continue;
            String s = (String) v;
            if (!s.toLowerCase().contains(needle.toLowerCase())) continue;
            defined++;
            println("STRING " + d.getAddress() + " \"" + s + "\"");
            for (Reference r : getReferencesTo(d.getAddress())) {
                Address from = r.getFromAddress();
                Function f = getFunctionContaining(from);
                println("  -> XREF from " + from + (f != null ? " in " + f.getName() : ""));
            }
        }

        // Pass 2: raw byte scan. The BSP panel struct starts with `char name[32]`, so the
        // name is often stored INLINE in the struct and never becomes a defined string.
        byte[] pat = needle.getBytes("US-ASCII");
        Address at = currentProgram.getMinAddress();
        int raw = 0;
        while (at != null && !monitor.isCancelled()) {
            Address found = currentProgram.getMemory().findBytes(at, pat, null, true, monitor);
            if (found == null) break;
            raw++;
            println("RAWHIT " + found);
            // struct __lcd_panel { char name[32]; struct disp_lcd_panel_fun func; }
            println("  -> candidate function table at " + found.add(32));
            for (Reference r : getReferencesTo(found)) {
                println("  -> XREF from " + r.getFromAddress());
            }
            if (raw > 200) { println("  (truncated)"); break; }
            at = found.add(1);
            if (at.compareTo(currentProgram.getMaxAddress()) >= 0) break;
        }

        println("PANEL_SEARCH_DONE defined=" + defined + " raw=" + raw);
    }
}
